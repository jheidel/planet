package util

import (
	"github.com/paulmach/orb"

	log "github.com/sirupsen/logrus"
)

// PolyUnion provides union functionality for orb polygons using the geos library.
func PolyUnion(p1, p2 orb.Polygon) orb.Polygon {
	if len(p1) == 0 {
		return p2
	}
	if len(p2) == 0 {
		return p1
	}
	u := GeoUnion(p1, p2)
	p, ok := u.(orb.Polygon)
	if !ok {
		log.Errorf("GeoUnion didn't return a polygon for %v and %v", debugGeoJson(p1), debugGeoJson(p2))
		return orb.Polygon{}
	}
	return p
}
