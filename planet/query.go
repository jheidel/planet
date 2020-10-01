package planet

import (
	"time"

	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/maptile"
)

func RequestTileOnDate(tile maptile.Tile, d time.Time) *Request {
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
					Config:    geojson.NewGeometry(tile.Bound().ToPolygon()),
				},
			},
		},
		ItemTypes: []string{ProductType},
	}
}
