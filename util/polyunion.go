package util

import (
	polyclip "github.com/akavel/polyclip-go"
	"github.com/paulmach/orb"
)

func toClip(p orb.Polygon) polyclip.Polygon {
	var poly polyclip.Polygon
	for _, ring := range p {
		if len(ring) > 0 && !ring.Closed() {
			ring = append(ring, ring[0])
		}
		var ct polyclip.Contour
		for _, pt := range ring {
			ct = append(ct, polyclip.Point{X: pt[0], Y: pt[1]})
		}
		poly = append(poly, ct)
	}
	return poly
}

func fromClip(poly polyclip.Polygon) orb.Polygon {
	var p orb.Polygon
	for _, ct := range poly {
		var ring orb.Ring
		for _, pt := range ct {
			ring = append(ring, orb.Point{pt.X, pt.Y})
		}
		if len(ring) > 0 && !ring.Closed() {
			ring = append(ring, ring[0])
		}
		p = append(p, ring)
	}
	return p
}

// PolyUnion provides union functionality for orb polygons using the polyclip
// library.
func PolyUnion(p1, p2 orb.Polygon) orb.Polygon {
	if len(p1) == 0 {
		return p2
	}
	if len(p2) == 0 {
		return p1
	}
	c1 := toClip(p1)
	c2 := toClip(p2)
	r := c1.Construct(polyclip.UNION, c2)
	return fromClip(r)
}
