package tilecache

import (
	"planet-server/planet"
	"sync"
	"time"

	"github.com/paulmach/orb"
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

	watchers map[*TileWatcher]bool

	mu sync.Mutex
}

func New() *TileCache {
	return &TileCache{
		watchers: make(map[*TileWatcher]bool),
	}
}

func (c *TileCache) get(bound orb.Bound) ([]*planet.Feature, bool) {
	trunc := 0
	for i, d := range c.cache {
		if time.Since(d.Added) > CacheHistory {
			trunc = i + 1
			continue
		}
		if d.Bound.Contains(bound.Min) && d.Bound.Contains(bound.Max) {
			return d.Features, true
		}
	}
	c.cache = c.cache[trunc:]
	return nil, false
}

func (c *TileCache) Get(bound orb.Bound) ([]*planet.Feature, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.get(bound)
}

func (c *TileCache) Put(bound orb.Bound, features []*planet.Feature) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = append(c.cache, &cachedData{
		Bound:    bound,
		Features: features,
		Added:    time.Now(),
	})
	log.Debugf("Tile cache has %d entries and %d watchers", len(c.cache), len(c.watchers))
	for w, _ := range c.watchers {
		if result, ok := c.get(w.bound); ok {
			log.Debugf("notifying watcher of result")
			w.C <- result
			w.remove()
		}
	}
}

type TileWatcher struct {
	C chan []*planet.Feature

	parent *TileCache
	bound  orb.Bound
}

func (w *TileWatcher) remove() {
	delete(w.parent.watchers, w)
}

func (w *TileWatcher) Close() {
	w.parent.mu.Lock()
	w.remove()
	w.parent.mu.Unlock()
	close(w.C)
}

func (c *TileCache) Watch(bound orb.Bound) *TileWatcher {
	w := &TileWatcher{
		C: make(chan []*planet.Feature),

		parent: c,
		bound:  bound,
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.watchers[w] = true
	return w
}
