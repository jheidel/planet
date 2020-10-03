package planet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/go-retryablehttp"
	"net/url"
	"time"

	"golang.org/x/sync/semaphore"

	log "github.com/sirupsen/logrus"
)

var (
	// Simple way to do client-side rate limiting. This is needed to stay under
	// planet quota.
	MaxConcurrent = semaphore.NewWeighted(3)
)

// QuickSearch queries the /quick-search planet API endpoint.
func QuickSearch(pctx context.Context, req *Request) (*Response, error) {
	ctx, cancel := context.WithDeadline(pctx, time.Now().Add(15*time.Second))
	defer cancel()

	if err := MaxConcurrent.Acquire(ctx, 1); err != nil {
		return nil, fmt.Errorf("api max concurrent: %v", err)
	}
	defer MaxConcurrent.Release(1)

	j, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("api encode: %v", err)
	}
	log.Debugf("Making API request %q", string(j))

	v := make(url.Values)
	v.Add("_sort", "acquired desc")
	r, err := retryablehttp.NewRequest("POST", "https://api.planet.com/data/v1/quick-search?"+v.Encode(), j)
	if err != nil {
		return nil, err
	}

	r.Header.Set("Content-Type", "application/json")
	r.SetBasicAuth(ApiKey, "")

	client := retryablehttp.NewClient()
	res, err := client.Do(r.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		buf := new(bytes.Buffer)
		buf.ReadFrom(res.Body)
		return nil, fmt.Errorf("api %s: %v", res.Status, buf.String())
	}

	dec := json.NewDecoder(res.Body)
	resp := &Response{}
	if err := dec.Decode(resp); err != nil {
		return nil, fmt.Errorf("api decode: %v")
	}
	return resp, nil
}
