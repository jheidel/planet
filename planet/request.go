package planet

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	log "github.com/sirupsen/logrus"
)

// QuickSearch queries the /quick-search planet API endpoint.
func QuickSearch(ctx context.Context, req *Request) (*Response, error) {
	j, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	log.Debugf("Making API request %q", string(j))

	v := make(url.Values)
	v.Add("_sort", "acquired desc")
	r, err := http.NewRequest("POST", "https://api.planet.com/data/v1/quick-search?"+v.Encode(), bytes.NewBuffer(j))
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", "application/json")
	r.SetBasicAuth(ApiKey, "")

	res, err := http.DefaultClient.Do(r.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	dec := json.NewDecoder(res.Body)
	resp := &Response{}
	if err := dec.Decode(resp); err != nil {
		return nil, err
	}
	return resp, nil
}
