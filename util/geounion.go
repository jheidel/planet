package util

import (
	"github.com/paulmach/orb"

	log "github.com/sirupsen/logrus"
)

func GeoUnion(g1, g2 orb.Geometry) orb.Geometry {
	p1, ok := g1.(orb.Polygon)
	if !ok {
		log.Errorf("p1 not a polygon")
		return g2
	}
	p2, ok := g2.(orb.Polygon)
	if !ok {
		log.Errorf("p1 not a polygon")
		return g1
	}
	return PolyUnion(p1, p2)
}
