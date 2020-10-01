package planet

import (
	"time"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
)

// TODO request tiles for satellite pass
// TODO request individual tile

func RequestRegionOnDate(bound orb.Bound, d time.Time) *Request {
	return &Request{
		Filter: &AndFilter{
			Type: "AndFilter",
			Config: []interface{}{
				&DateRangeFilter{
					Type:      "DateRangeFilter",
					FieldName: "acquired",
					Config: &DateRange{
						Start: d,
						End:   d.Add(24 * time.Hour),
					},
				},
				&GeoFilter{
					Type:      "GeometryFilter",
					FieldName: "geometry",
					Config:    geojson.NewGeometry(bound.ToPolygon()),
				},
			},
		},
		ItemTypes: []string{ProductType},
	}
}
