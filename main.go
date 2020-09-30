package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/clip"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/maptile"
	"github.com/paulmach/orb/planar"

	log "github.com/sirupsen/logrus"
)

var (
	port = flag.Int("port", 8080, "Serving port")
)

const (
	ApiKey      = "aeb8e20145d34c9497cf351f6de0595e"
	ProductType = "PSScene4Band"
)

func topLevelContext() context.Context {
	ctx, cancelf := context.WithCancel(context.Background())
	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigs
		log.Warnf("Caught signal %q, shutting down.", sig)
		cancelf()
	}()
	return ctx
}

func fetchTile(ctx context.Context, ID string, t maptile.Tile) (image.Image, error) {
	// TODO: DNS load balancing
	url := fmt.Sprintf("https://tiles2.planet.com/data/v1/%s/%s/%d/%d/%d.png?api_key=%s", ProductType, ID, t.Z, t.X, t.Y, ApiKey)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		buf := new(strings.Builder)
		io.Copy(buf, res.Body)
		return nil, fmt.Errorf("tile server %v: %q", res.Status, buf.String())
	}
	return png.Decode(res.Body)
}

func tileFromRequest(r *http.Request) (maptile.Tile, error) {
	x, err := strconv.Atoi(mux.Vars(r)["x"])
	if err != nil {
		return maptile.Tile{}, err
	}
	y, err := strconv.Atoi(mux.Vars(r)["y"])
	if err != nil {
		return maptile.Tile{}, err
	}
	z, err := strconv.Atoi(mux.Vars(r)["z"])
	if err != nil {
		return maptile.Tile{}, err
	}
	return maptile.Tile{X: uint32(x), Y: uint32(y), Z: maptile.Zoom(z)}, nil
}

type planetGeoFilter struct {
	Type      string            `json:"type"`
	Config    *geojson.Geometry `json:"config"`
	FieldName string            `json:"field_name"`
}

type planetDateRange struct {
	Start time.Time `json:"gt"`
	End   time.Time `json:"lte"`
}

type planetDateRangeFilter struct {
	Type      string           `json:"type"`
	FieldName string           `json:"field_name"`
	Config    *planetDateRange `json:"config"`
}

type planetAndFilter struct {
	Type   string        `json:"type"`
	Config []interface{} `json:"config"`
}

type planetRequest struct {
	Filter    interface{} `json:"filter"`
	ItemTypes []string    `json:"item_types"`
}

type planetProperties struct {
	Acquired        time.Time `json:"acquired"`
	Published       time.Time `json:"published"`
	ClearPercent    int       `json:"clear_percent"`
	VisiblePercent  int       `json:"visible_percent"`
	PixelResolution int       `json:"pixel_resolution"`
}

type planetFeature struct {
	Geometry   *geojson.Geometry `json:"geometry"`
	ID         string            `json:"id"`
	Properties *planetProperties `json:"properties"`
}

type planetResponse struct {
	Features []*planetFeature `json:"features"`
}

func requestFromBounds(d time.Time, g orb.Geometry) *planetRequest {
	return &planetRequest{
		Filter: &planetAndFilter{
			Type: "AndFilter",
			Config: []interface{}{
				&planetDateRangeFilter{
					Type:      "DateRangeFilter",
					FieldName: "acquired",
					Config: &planetDateRange{
						Start: d,
						End:   d.Add(24 * time.Hour),
					},
				},
				&planetGeoFilter{
					Type:      "GeometryFilter",
					FieldName: "geometry",
					Config:    geojson.NewGeometry(g),
				},
			},
		},
		ItemTypes: []string{ProductType},
	}
}

func serveTile(w http.ResponseWriter, r *http.Request) {
	tile, err := tileFromRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "invalid tile argument: %v", err.Error())
		return
	}

	// TODO actual error handling

	// TODO tune threshold
	if tile.Z < 12 {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "zoom in to load planet tiles")
		return
	}

	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		log.Fatal(err)
	}

	now := time.Now().Add(-24 * time.Hour)
	d := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	j, _ := json.Marshal(requestFromBounds(d, tile.Bound().ToPolygon()))

	log.Infof("Request :%v", string(j))

	v := make(url.Values)
	v.Add("_sort", "acquired desc")
	req, err := http.NewRequest("POST", "https://api.planet.com/data/v1/quick-search?"+v.Encode(), bytes.NewBuffer(j))
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(r.Context())
	req.SetBasicAuth(ApiKey, "")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Errorf("%v", err)
		return
	}

	dec := json.NewDecoder(res.Body)
	resp := &planetResponse{}
	if err := dec.Decode(resp); err != nil {
		log.Fatal(err)
	}

	// TODO probably sort by coverage desc

	// TODO visualize individual satellite passes.

	// TODO tile server modes:
	//  1) Maximum overlap for date
	//  2) Individual satellite pass selection for AOI

	// open in caltopo

	// TODO cache geometries, avoid repeat API requests.

	// TODO merge until covered, then stop!

	var IDs []string
	var polys orb.MultiPolygon
	for _, f := range resp.Features {
		g := f.Geometry.Geometry()
		p, ok := g.(orb.Polygon)
		if !ok {
			log.Fatal("not poylgon")
		}

		//ts := tilecover.Geometry(g, tile.Z)

		IDs = append(IDs, f.ID)

		intersect := clip.Polygon(tile.Bound(), p)
		polys = append(polys, intersect)
		coverage := planar.Area(polys) / planar.Area(tile.Bound())

		log.Infof("This chunk has coverage %.2f",
			planar.Area(intersect)/planar.Area(tile.Bound()))

		// TODO multipolygon doesn't do union :-(
		log.Infof("With %q, coverage now %.2f", f.ID, coverage)

		if coverage >= 1 {
			break
		}

		// TODO
		//if _, ok := ts[tile]; ok {
		//}
	}

	// TODO parallel requests
	var out *image.RGBA
	for _, ID := range IDs {
		log.Infof("Fetching tile %q", ID)
		t, err := fetchTile(r.Context(), ID, tile)
		if err != nil {
			log.Fatal(err)
		}

		if out == nil {
			out = image.NewRGBA(t.Bounds())
		}

		if out.Bounds() != t.Bounds() {
			log.Fatal("Bad bounds")
		}

		draw.Draw(out, t.Bounds(), t, image.Point{0, 0}, draw.Over)
	}
	if out == nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "no tile data")
		return
	}

	w.Header().Set("Content-Type", "image/png")
	if err := png.Encode(w, out); err != nil {
		log.Fatal(err)
	}
}

// TODO caching of tiles

func main() {
	ctx := topLevelContext()

	router := mux.NewRouter()
	router.HandleFunc("/tile/{z:[0-9]+}/{x:[0-9]+}/{y:[0-9]+}.png", serveTile).Methods("GET")

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: router,
	}
	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()
	log.Infof("Starting")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("ListenAndServe(): %v", err)
	}
	log.Infof("Shutdown")
}
