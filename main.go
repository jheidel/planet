package main

import (
	"context"
	"flag"
	"fmt"
	"image/png"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"planet-server/planet"

	"github.com/gorilla/mux"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/clip"
	"github.com/paulmach/orb/maptile"
	"github.com/paulmach/orb/planar"

	log "github.com/sirupsen/logrus"
)

var (
	port = flag.Int("port", 8080, "Serving port")
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

func serveTile(w http.ResponseWriter, r *http.Request) {
	tile, err := tileFromRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "invalid tile argument: %v", err.Error())
		return
	}

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

	now := time.Now().Add(-48 * time.Hour)
	d := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	resp, err := planet.QuickSearch(r.Context(), planet.RequestTileOnDate(tile, d))
	if err != nil {
		log.Fatal(err)
	}

	// TODO cache geometries, avoid repeat API requests.

	// TODO probably sort by coverage desc

	// TODO visualize individual satellite passes.

	// TODO tile server modes:
	//  1) Maximum overlap for date
	//  2) Individual satellite pass selection for AOI

	// open in caltopo

	// TODO merge until covered, then stop!

	var IDs []string
	var union orb.Polygon
	for _, f := range resp.Features {
		g := f.Geometry.Geometry()
		p, ok := g.(orb.Polygon)
		if !ok {
			log.Fatal("not poylgon")
		}

		//ts := tilecover.Geometry(g, tile.Z)

		IDs = append(IDs, f.ID)

		intersect := clip.Polygon(tile.Bound(), p)
		union = PolyUnion(union, intersect)

		log.Infof("This chunk has coverage %.2f",
			planar.Area(intersect)/planar.Area(tile.Bound()))

		coverage := planar.Area(union) / planar.Area(tile.Bound())

		log.Infof("With %q, coverage now %.2f", f.ID, coverage)

		if coverage >= 1 {
			break
		}
	}

	out, err := planet.FetchTiles(r.Context(), IDs, tile)
	if err != nil {
		log.Fatal(err)
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
