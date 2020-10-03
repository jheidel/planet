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

func (s *TileServer) getFeatures(pctx context.Context, tile maptile.Tile, dt time.Time) ([]*planet.Feature, error) {
	ctx, cancel := context.WithCancel(pctx)
	defer cancel()

	cache := s.Cache.For(dt)

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
		resp, err := s.Client.QuickSearch(ctx, planet.RequestRegionOnDate(region, dt))
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

func (s *TileServer) getTile(r *http.Request) (image.Image, error) {
	tile, err := tileFromRequest(r)
	if err != nil {
		return nil, err
	}

	if tile.Z < 12 {
		return nil, ErrZoom
	}

	// TODO other forms of requests, or maybe other API endpoints?
	dt, err := dateFromRequest(r.Form.Get("date"))
	if err != nil {
		return nil, fmt.Errorf("invalid date %q: %v", r.Form["date"], err)
	}

	features, err := s.getFeatures(r.Context(), tile, dt)
	if err != nil {
		return nil, err
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
