package tilecache

import (
	"encoding/json"
	"sync"

	log "github.com/sirupsen/logrus"
)

type MultiCache struct {
	m map[string]*TileCache
	l sync.Mutex
}

func NewMulti() *MultiCache {
	return &MultiCache{
		m: make(map[string]*TileCache),
	}
}

func (c *MultiCache) For(k interface{}) *TileCache {
	j, err := json.Marshal(k)
	if err != nil {
		panic(err)
	}
	key := string(j)
	c.l.Lock()
	defer c.l.Unlock()
	r, ok := c.m[key]
	if ok {
		return r
	}
	log.Debugf("Building new cache pool for %v", key)
	r = New()
	c.m[key] = r
	return r
}
