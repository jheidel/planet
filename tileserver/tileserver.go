package tileserver

import (
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
	"time"

	"github.com/gorilla/mux"
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
)

type TileServer struct {
	Cache *tilecache.MultiCache
}

func New() *TileServer {
	return &TileServer{
		Cache: tilecache.NewMulti(),
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
	loc, err := time.LoadLocation(util.EnvOrDefault("TZ", "America/Los_Angeles"))
	if err != nil {
		return time.Time{}, err
	}
	return time.ParseInLocation("2006-01-02", v, loc)
}

func blankImage() *image.RGBA {
	return image.NewRGBA(image.Rectangle{
		Min: image.Point{X: 0, Y: 0},
		Max: image.Point{X: TileSize, Y: TileSize},
	})
}

func writeErrorTile(w http.ResponseWriter, err error) {
	img := blankImage()

	col := color.RGBA{255, 0, 0, 255}
	point := fixed.Point26_6{fixed.Int26_6(0 * 64), fixed.Int26_6(256 / 2 * 64)}

	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: basicfont.Face7x13,
		Dot:  point,
	}
	d.DrawString(err.Error())

	w.Header().Set("Content-Type", "image/png")
	if err := png.Encode(w, img); err != nil {
		log.Fatal(err)
	}
}

func (s *TileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	tile, err := tileFromRequest(r)
	if err != nil {
		writeErrorTile(w, fmt.Errorf("invalid tile argument: %v", err.Error()))
		return
	}

	if tile.Z < 12 {
		writeErrorTile(w, errors.New("zoom in to load planet tiles"))
		return
	}

	// TODO other forms of requests, or maybe other API endpoints?
	dt, err := dateFromRequest(r.Form.Get("date"))
	if err != nil {
		writeErrorTile(w, fmt.Errorf("invalid date %q: %v", r.Form["date"], err))
		return
	}

	features, ok := s.Cache.For(dt).Get(tile)
	if !ok {
		region := tile.Bound(BoundExpand)
		resp, err := planet.QuickSearch(r.Context(), planet.RequestRegionOnDate(region, dt))
		if err != nil {
			writeErrorTile(w, err)
			return
		}
		features = resp.Features
		s.Cache.For(dt).Put(region, features)
	}

	// TODO visualize individual satellite passes.

	// TODO tile server modes:
	//  1) Maximum overlap for date
	//  2) Individual satellite pass selection for AOI
	//  3) Individual image selection

	// URL to open in caltopo

	// TODO thumbnail proxyP

	var IDs []string
	var union orb.Polygon
	for _, f := range features {
		g := f.Geometry.Geometry()
		p, ok := g.(orb.Polygon)
		if !ok {
			log.Fatal("not poylgon")
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

	out, err := planet.FetchTiles(r.Context(), IDs, tile)
	if err != nil {
		writeErrorTile(w, fmt.Errorf("failed to fetch tiles: %v", err))
		return
	}

	w.Header().Set("Content-Type", "image/png")
	if err := png.Encode(w, out); err != nil {
		log.Fatal(err)
	}
}
