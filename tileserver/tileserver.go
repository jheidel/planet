package tileserver

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"planet-server/planet"
	"planet-server/tilecache"
	"planet-server/util"
	"strconv"
	"strings"
	"time"

	"github.com/eidolon/wordwrap"
	"github.com/gorilla/mux"
	"github.com/llgcode/draw2d/draw2dimg"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/clip"
	"github.com/paulmach/orb/maptile"
	"github.com/paulmach/orb/planar"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	log "github.com/sirupsen/logrus"
)

const (
	// Number of tiles to expand when querying planet for image sets
	BoundExpand = 5

	TileSize = 256

	MinZ = 11
)

var (
	ErrZoom = errors.New("zoom in to view planet tiles")
)

type TileServer struct {
	Cache  *tilecache.MultiCache
	Client *planet.Client
}

func New(p *planet.Client) *TileServer {
	return &TileServer{
		Cache:  tilecache.NewMulti(),
		Client: p,
	}
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

func dateFromRequest(v string) (time.Time, error) {
	if v == "" {
		return time.Time{}, fmt.Errorf("missing date")
	}
	// TODO request from frontend can include TZ
	return time.ParseInLocation("2006-01-02", v, util.LocationOrDie())
}

func blankImage() *image.RGBA {
	return image.NewRGBA(image.Rectangle{
		Min: image.ZP,
		Max: image.Point{X: TileSize, Y: TileSize},
	})
}

func toErrorTile(err error) image.Image {
	img := blankImage()

	gc := draw2dimg.NewGraphicContext(img)

	gc.Save()
	gc.SetStrokeColor(color.RGBA{255, 255, 0, 255})
	gc.SetFillColor(color.RGBA{0, 0, 0, 0})
	gc.SetLineWidth(1)

	gc.MoveTo(0, 0)
	gc.LineTo(0, 255)
	gc.LineTo(255, 255)
	gc.LineTo(255, 0)
	gc.LineTo(0, 0)
	gc.Close()

	gc.FillStroke()
	gc.Restore()

	padding := 5

	col := color.RGBA{255, 0, 0, 255}
	wrapper := wordwrap.Wrapper((256-(2*padding))/7, false)
	lines := strings.Split(wrapper(err.Error()), "\n")

	p := 7 + padding
	for _, line := range lines {
		point := fixed.Point26_6{fixed.Int26_6(padding * 64), fixed.Int26_6(p * 64)}
		p += 14
		d := &font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(col),
			Face: basicfont.Face7x13,
			Dot:  point,
		}
		d.DrawString(string(line))
	}
	return img
}

func (s *TileServer) getFeatures(pctx context.Context, tile maptile.Tile, ts time.Time, satellite string) ([]*planet.Feature, error) {
	ctx, cancel := context.WithCancel(pctx)
	defer cancel()

	var cache *tilecache.TileCache
	if satellite == "" {
		cache = s.Cache.For(ts)
	} else {
		key := struct {
			Ts        time.Time
			Satellite string
		}{ts, satellite}
		cache = s.Cache.For(key)
	}

	// Try the cache directly first.
	features, ok := cache.Get(tile.Bound())
	if ok {
		return features, nil
	}

	apic := make(chan []*planet.Feature, 1)
	errc := make(chan error, 1)
	go func() {
		// Request a padded region to reduce the number of API requests.
		region := tile.Bound(BoundExpand)

		var req *planet.Request
		if satellite == "" {
			req = planet.RequestRegionOnDate(region, ts)
		} else {
			req = planet.RequestRegionForSatellite(region, ts, satellite)
		}

		resp, err := s.Client.QuickSearch(ctx, req)
		if err != nil {
			errc <- err
			return
		}
		cache.Put(region, resp.Features)
		apic <- resp.Features
	}()

	watcher := cache.Watch(tile.Bound())
	defer watcher.Close()

	// Wait for either the API to return a result, or we get an equivalent result
	// from the cache.
	select {
	case features := <-watcher.C:
		log.Debugf("Resolved request from watcher")
		return features, nil
	case features := <-apic:
		return features, nil
	case err := <-errc:
		return nil, err
	}
}

func getTileIDs(tile maptile.Tile, features []*planet.Feature) []string {
	var IDs []string
	var union orb.Polygon
	for _, f := range features {
		g := f.Geometry.Geometry()
		p, ok := g.(orb.Polygon)
		if !ok {
			log.Fatal("not polygon")
		}

		intersect := clip.Polygon(tile.Bound(), p.Clone())

		overlap := planar.Area(intersect) / planar.Area(tile.Bound())
		if overlap == 0 {
			continue // Not a matching tile
		}

		union = util.PolyUnion(union, intersect)
		coverage := planar.Area(union) / planar.Area(tile.Bound())
		IDs = append(IDs, f.ID)

		log.Debugf("Tile %q: overlap %.2f, coverage %.2f", f.ID, overlap, coverage)

		if coverage >= 1 {
			break
		}
	}
	return IDs
}

func parseUnix(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("missing ts")
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(i, 0), nil
}

func (s *TileServer) getTile(r *http.Request) (image.Image, error) {
	tile, err := tileFromRequest(r)
	if err != nil {
		return nil, err
	}

	ID := r.Form.Get("id")
	date := r.Form.Get("date")

	var IDs []string
	if ID != "" {
		// Search by ID
		IDs = []string{ID}
	} else {
		// Search by date or satellite (mosaic)
		var sat string
		var ts time.Time

		if date != "" {
			// Search by date
			var err error
			ts, err = dateFromRequest(r.Form.Get("date"))
			if err != nil {
				return nil, fmt.Errorf("invalid date %q: %v", r.Form["date"], err)
			}
			if tile.Z < MinZ {
				// Zoom is bounded for date mosaic to prevent insane tile server load
				return nil, ErrZoom
			}
		} else {
			// Search by satellite
			sat = r.Form.Get("satellite_id")
			if sat == "" {
				return nil, fmt.Errorf("missing satellite_id")
			}
			var err error
			ts, err = parseUnix(r.Form.Get("ts"))
			if err != nil {
				return nil, err
			}
		}

		features, err := s.getFeatures(r.Context(), tile, ts, sat)
		if err != nil {
			return nil, err
		}

		IDs = getTileIDs(tile, features)
	}

	img, err := s.Client.FetchTiles(r.Context(), IDs, tile)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tiles: %v", err)
	}

	return img, nil
}

func (s *TileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	w.Header().Set("Content-Type", "image/png")
	var img image.Image
	img, err := s.getTile(r)
	if err != nil {
		if err != ErrZoom {
			log.Errorf("serve tile error: %v", err)
		}
		img = toErrorTile(err)
	}

	if err := png.Encode(w, img); err != nil {
		// These errors are expected when clients abort requests.
		log.Debugf("png encode failed: %v", err)
	}
}
