package metaserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"planet-server/planet"
	"planet-server/util"
	"strconv"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/jinzhu/copier"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/maptile"
	log "github.com/sirupsen/logrus"
)

const (
	SearchExpand = 5
)

type MetaServer struct {
}

func New() *MetaServer {
	return &MetaServer{}

}

type metaRequest struct {
	Lat     float64
	Lng     float64
	Z       int
	GroupBy string
}

type metaEntry struct {
	Thumb    string    `json:"thumb"`
	Acquired time.Time `json:"acquired"`

	VisiblePercent int `json:"visible_percent"`
	ClearPercent   int `json:"clear_percent"`
	CloudPercent   int `json:"cloud_percent"`

	Geometry    *geojson.Geometry `json:"geometry"`
	SatelliteID string            `json:"satellite_id"`

	TileName string `json:"tile_name"`
	TileURL  string `json:"tile_url"`
}

type metaResponse struct {
	Results []*metaEntry `json:"results"`
	Error   string       `json:"error"`
}

func parseRequest(r *http.Request) (*metaRequest, error) {
	r.ParseForm()
	req := &metaRequest{
		GroupBy: r.Form.Get("group_by"),
	}
	var err error
	lat := r.Form.Get("lat")
	if lat == "" {
		return nil, fmt.Errorf("missing lat")
	}
	req.Lat, err = strconv.ParseFloat(lat, 64)
	if err != nil {
		return nil, fmt.Errorf("bad lat: %v", err)
	}
	lng := r.Form.Get("lng")
	if lng == "" {
		return nil, fmt.Errorf("missing lng")
	}
	req.Lng, err = strconv.ParseFloat(lng, 64)
	if err != nil {
		return nil, fmt.Errorf("bad lng: %v", err)
	}
	z := r.Form.Get("z")
	if z == "" {
		return nil, fmt.Errorf("missing z")
	}
	req.Z, err = strconv.Atoi(z)
	if err != nil {
		return nil, fmt.Errorf("bad z: %v", err)
	}
	if req.Z < 12 {
		req.Z = 12
	}
	return req, nil
}

func dateOfFeature(f *planet.Feature) string {
	return f.Properties.Acquired.In(util.LocationOrDie()).Format("2006-01-02")
}

func sameDate(f1, f2 *planet.Feature) bool {
	return dateOfFeature(f1) == dateOfFeature(f2)
}

func sameSatellite(f1, f2 *planet.Feature) bool {
	delta := f1.Properties.Acquired.Sub(f2.Properties.Acquired)
	if delta < 0 {
		delta = -1 * delta
	}
	return f1.Properties.SatelliteID == f2.Properties.SatelliteID && delta < time.Hour
}

func mergeFeature(base, other *planet.Feature) *planet.Feature {
	ret := &planet.Feature{}
	copier.Copy(ret, base)

	if other.Properties.Acquired.After(ret.Properties.Acquired) {
		ret.Properties.Acquired = other.Properties.Acquired
	}
	if other.Properties.VisiblePercent > ret.Properties.VisiblePercent {
		ret.Properties.VisiblePercent = other.Properties.VisiblePercent
	}
	if other.Properties.ClearPercent > ret.Properties.ClearPercent {
		ret.Properties.ClearPercent = other.Properties.ClearPercent
	}
	if other.Properties.CloudPercent < ret.Properties.CloudPercent {
		ret.Properties.CloudPercent = other.Properties.CloudPercent
	}

	if sameSatellite(base, other) {
		// These are from the same satellite pass, merge them.
		ret.Geometry = geojson.NewGeometry(util.GeoUnion(base.Geometry.Geometry(), other.Geometry.Geometry()))
	} else {
		ret.Geometry = nil
		ret.Properties.SatelliteID = ""
	}

	return ret
}

type equality func(f1, f2 *planet.Feature) bool

// TODO something better than this awful O(N^2) thing
func flatten(is_equal equality, features []*planet.Feature) []*planet.Feature {
	var ret []*planet.Feature
outer:
	for _, nf := range features {
		for i := len(ret) - 1; i >= 0; i-- {
			ef := ret[i]
			if is_equal(nf, ef) {
				ret[i] = mergeFeature(ef, nf)
				continue outer
			}
		}
		ret = append(ret, nf)
	}
	return ret
}

func (s *MetaServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	jsonError := func(err error, code int) {
		w.WriteHeader(code)
		mr := &metaResponse{Error: err.Error()}
		if err := json.NewEncoder(w).Encode(mr); err != nil {
			log.Errorf("error encode: %v", err)
		}
	}

	req, err := parseRequest(r)
	if err != nil {
		log.Errorf("meta parseRequest: %v", err)
		jsonError(err, http.StatusBadRequest)
		return
	}

	log.Debugf("Search request: %+v", spew.Sdump(req))

	region := maptile.At(orb.Point{req.Lng, req.Lat}, maptile.Zoom(req.Z)).Bound(SearchExpand)

	end := time.Now()
	start := end.Add(-7 * 24 * time.Hour)

	t := time.Now()
	resp, err := planet.QuickSearch(r.Context(), planet.RequestRegion(region, start, end))
	if err != nil {
		log.Errorf("meta QuickSearch: %v", err)
		jsonError(err, http.StatusInternalServerError)
		return
	}
	log.Debugf("API search in %v", time.Since(t))

	var features []*planet.Feature

	switch req.GroupBy {
	case "date":
		features = flatten(sameDate, resp.Features)
	case "satellite":
		features = flatten(sameSatellite, resp.Features)
	default:
		features = resp.Features
	}

	mr := &metaResponse{}
	for _, f := range features {
		mr.Results = append(mr.Results, &metaEntry{
			Thumb:          fmt.Sprintf("/api/thumb/%s.png", f.ID),
			Acquired:       f.Properties.Acquired,
			VisiblePercent: f.Properties.VisiblePercent,
			ClearPercent:   f.Properties.ClearPercent,
			CloudPercent:   f.Properties.CloudPercent,
			Geometry:       f.Geometry,
			SatelliteID:    f.Properties.SatelliteID,

			TileName: "Planet " + dateOfFeature(f),
			TileURL:  fmt.Sprintf("/api/tile/{z}/{x}/{y}.png?date=" + dateOfFeature(f)),
		})
	}

	if err := json.NewEncoder(w).Encode(mr); err != nil {
		log.Errorf("meta encode: %v", err)
	}
}
