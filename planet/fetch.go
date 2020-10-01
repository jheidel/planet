package planet

import (
	"context"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/paulmach/orb/maptile"
	log "github.com/sirupsen/logrus"
)

func fetchTile(ctx context.Context, ID string, t maptile.Tile) (image.Image, error) {
	// TODO: could be improved to use some actual load balancing / or fallback mechanism.
	url := fmt.Sprintf("https://tiles%d.planet.com/data/v1/%s/%s/%d/%d/%d.png?api_key=%s", rand.Intn(4), ProductType, ID, t.Z, t.X, t.Y, ApiKey)

	log.Debugf("Fetching tile %q", ID)
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

// FetchTiles downloads tiles from the planet tile server to cover the provided tile. All the IDs provided are unioned.
func FetchTiles(pctx context.Context, IDs []string, t maptile.Tile) (image.Image, error) {
	ctx, cancel := context.WithDeadline(pctx, time.Now().Add(15*time.Second))
	defer cancel()

	imagec := make(chan image.Image)
	errc := make(chan error)

	// Start all image fetches in parallel
	for _, ID := range IDs {
		go func(id string) {
			img, err := fetchTile(ctx, id, t)
			if err != nil {
				errc <- err
				return
			}
			imagec <- img
		}(ID)
	}

	var out *image.RGBA
	var err error
	for range IDs {
		select {
		case imgerr := <-errc:
			if imgerr != nil && err == nil {
				// Keep first error and cancel.
				err = imgerr
				// Abort any remaining operations.
				cancel()
			}
		case img := <-imagec:
			if out == nil {
				out = image.NewRGBA(img.Bounds())
			}
			if out.Bounds() != img.Bounds() {
				err = fmt.Errorf("Bounds mismatch, %v vs %v", out.Bounds(), img.Bounds())
				cancel()
				continue
			}
			draw.Draw(out, img.Bounds(), img, image.Point{0, 0}, draw.Over)
		}
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}
