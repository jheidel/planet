package tilecache

import (
	"planet-server/planet"
	"sync"
	"time"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/maptile"
	log "github.com/sirupsen/logrus"
)

type cachedData struct {
	Features []*planet.Feature
	Bound    orb.Bound
	Added    time.Time
}

const (
	CacheHistory = 10 * time.Minute
)

type TileCache struct {
	// TODO some more efficient data structure for this
	//  - don't use O(N) lookup
	//  - simplify bounds that are contained in others
	cache []*cachedData

	mu sync.Mutex
}

func New() *TileCache {
	return &TileCache{}
}

func (c *TileCache) Get(tile maptile.Tile) ([]*planet.Feature, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	b := tile.Bound()
	trunc := 0
	for i, d := range c.cache {
		if time.Since(d.Added) > CacheHistory {
			trunc = i + 1
			continue
		}
		if d.Bound.Contains(b.Min) && d.Bound.Contains(b.Max) {
			return d.Features, true
		}
	}
	c.cache = c.cache[trunc:]
	return nil, false
}

func (c *TileCache) Put(bound orb.Bound, features []*planet.Feature) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = append(c.cache, &cachedData{
		Bound:    bound,
		Features: features,
		Added:    time.Now(),
	})
	log.Debugf("Tile cache has %d entries", len(c.cache))
}
