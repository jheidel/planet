package planet

import (
	"time"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
)

func RequestRegion(bound orb.Bound, start, end time.Time) *Request {
	return &Request{
		Filter: &AndFilter{
			Type: "AndFilter",
			Config: []interface{}{
				&DateRangeFilter{
					Type:      "DateRangeFilter",
					FieldName: "acquired",
					Config: &DateRange{
						Start: start,
						End:   end,
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

func RequestRegionOnDate(bound orb.Bound, d time.Time) *Request {
	return RequestRegion(bound, d, d.Add(24*time.Hour))
}

func RequestRegionForSatellite(bound orb.Bound, d time.Time, satellite string) *Request {
	start := d.Add(-15 * time.Minute)
	end := d.Add(15 * time.Minute)
	req := RequestRegion(bound, start, end)
	af := req.Filter.(*AndFilter)
	af.Config = append(af.Config, interface{}(&StringInFilter{
		Type:      "StringInFilter",
		FieldName: "satellite_id",
		Config:    []string{satellite},
	}))
	return req
}
