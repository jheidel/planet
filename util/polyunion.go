package util

import (
	"github.com/paulmach/orb"

	hull "github.com/furstenheim/go-convex-hull-2d"
)

type coordinates []orb.Point

func (c coordinates) Take(i int) (x, y float64) {
	return c[i][0], c[i][1]
}

func (c coordinates) Len() int {
	return len(c)
}

func (c coordinates) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

func (c coordinates) Slice(i, j int) hull.Interface {
	return c[i:j]
}

func toOuterRing(p orb.Polygon) coordinates {
	if len(p) == 0 {
		return coordinates{}
	}
	return coordinates(p[0])
}

// PolyUnion provides union functionality for orb polygons using the geos library.
func PolyUnion(p1, p2 orb.Polygon) orb.Polygon {
	var c coordinates
	c = append(c, toOuterRing(p1)...)
	c = append(c, toOuterRing(p2)...)
	h := hull.New(c)

	var ring orb.Ring
	for i := 0; i < h.Len(); i++ {
		x, y := h.Take(i)
		ring = append(ring, orb.Point{x, y})
	}
	if len(ring) > 0 && !ring.Closed() {
		ring = append(ring, ring[0])
	}
	return orb.Polygon{ring}
}
