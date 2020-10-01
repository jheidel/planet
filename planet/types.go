package planet

import (
	"time"

	"github.com/paulmach/orb/geojson"
)

const (
	// TODO move config
	ApiKey      = "aeb8e20145d34c9497cf351f6de0595e"
	ProductType = "PSScene4Band"
)

type GeoFilter struct {
	Type      string            `json:"type"`
	Config    *geojson.Geometry `json:"config"`
	FieldName string            `json:"field_name"`
}

type DateRange struct {
	Start time.Time `json:"gt"`
	End   time.Time `json:"lte"`
}

type DateRangeFilter struct {
	Type      string     `json:"type"`
	FieldName string     `json:"field_name"`
	Config    *DateRange `json:"config"`
}

type AndFilter struct {
	Type   string        `json:"type"`
	Config []interface{} `json:"config"`
}

type Properties struct {
	Acquired        time.Time `json:"acquired"`
	Published       time.Time `json:"published"`
	ClearPercent    int       `json:"clear_percent"`
	VisiblePercent  int       `json:"visible_percent"`
	CloudPercent    int       `json:"cloud_percent"`
	SatelliteID     string    `json:"satellite_id"`
	PixelResolution int       `json:"pixel_resolution"`
}

type Feature struct {
	Geometry   *geojson.Geometry `json:"geometry"`
	ID         string            `json:"id"`
	Properties *Properties       `json:"properties"`
}

type Request struct {
	Filter    interface{} `json:"filter"`
	ItemTypes []string    `json:"item_types"`
}

type Response struct {
	Features []*Feature `json:"features"`
}
