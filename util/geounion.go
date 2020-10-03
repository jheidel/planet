package util

import (
	"fmt"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
	"github.com/paulsmith/gogeos/geos"

	log "github.com/sirupsen/logrus"
)

func toGeos(g orb.Geometry) (*geos.Geometry, error) {
	p, ok := g.(orb.Polygon)
	if !ok {
		return nil, fmt.Errorf("geometry not supported")
	}
	if len(p) == 0 {
		return nil, nil
	}
	ring := p[0]
	if len(ring) > 0 && !ring.Closed() {
		ring = append(ring, ring[0])
	}
	var shell []geos.Coord
	for _, pt := range ring {
		shell = append(shell, geos.NewCoord(pt[0], pt[1]))
	}
	return geos.NewPolygon(shell)
}

func fromGeos(g *geos.Geometry) (orb.Geometry, error) {
	shell, err := g.Shell()
	if err != nil {
		return orb.Polygon{}, fmt.Errorf("shell: %v", err)
	}
	coords, err := shell.Coords()
	if err != nil {
		return orb.Polygon{}, err
	}
	var ring orb.Ring
	for _, pt := range coords {
		ring = append(ring, orb.Point{pt.X, pt.Y})
	}
	if len(ring) > 0 && !ring.Closed() {
		ring = append(ring, ring[0])
	}
	return orb.Polygon([]orb.Ring{ring}), nil
}

func debugGeoJson(g orb.Geometry) string {
	v, _ := geojson.NewGeometry(g).MarshalJSON()
	return string(v)
}

func geoUnion(g1, g2 orb.Geometry) (orb.Geometry, error) {
	geo1, err := toGeos(g1)
	if err != nil || geo1 == nil {
		return g2,
			fmt.Errorf("toGeos(g1): %v", err)
	}
	geo2, err := toGeos(g2)
	if err != nil || geo2 == nil {
		return g1,
			fmt.Errorf("toGeos(g2): %v", err)
	}
	union, err := geo1.Union(geo2)
	if err != nil {
		return orb.Polygon{},
			fmt.Errorf("union: %v", err)
	}
	hull, err := union.ConvexHull()
	if err != nil {
		return orb.Polygon{},
			fmt.Errorf("hull: %v", err)
	}
	out, err := fromGeos(hull)
	if err != nil {
		return orb.Polygon{},
			fmt.Errorf("fromGeos: %v", err)
	}
	return out, nil
}

func GeoUnion(g1, g2 orb.Geometry) orb.Geometry {
	out, err := geoUnion(g1, g2)
	if err != nil {
		log.Errorf("GeoUnion failed: %v\n\nG1: %s\n\nG2: %s\n\n", err, debugGeoJson(g1), debugGeoJson(g2))
		return orb.Polygon{}
	}
	return out
}
