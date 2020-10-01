package planet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/sync/semaphore"

	log "github.com/sirupsen/logrus"
)

var (
	MaxConcurrent = semaphore.NewWeighted(2)
)

// QuickSearch queries the /quick-search planet API endpoint.
func QuickSearch(pctx context.Context, req *Request) (*Response, error) {
	ctx, cancel := context.WithDeadline(pctx, time.Now().Add(10*time.Second))
	defer cancel()

	if err := MaxConcurrent.Acquire(ctx, 1); err != nil {
		return nil, fmt.Errorf("max concurrent: %v", err)
	}
	defer MaxConcurrent.Release(1)

	j, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("json encode: %v", err)
	}
	log.Debugf("Making API request %q", string(j))

	// TODO add rate limiting on our side here rather than relying on server push-back

	// TODO move retrying client into library

	buf := bytes.NewBuffer(j)

	var res *http.Response
	for ctx.Err() == nil {
		v := make(url.Values)
		v.Add("_sort", "acquired desc")
		r, err := http.NewRequest("POST", "https://api.planet.com/data/v1/quick-search?"+v.Encode(), buf)
		if err != nil {
			return nil, err
		}
		r.Header.Set("Content-Type", "application/json")
		r.SetBasicAuth(ApiKey, "")
		resp, err := http.DefaultClient.Do(r.WithContext(ctx))
		// TODO ugly
		res = resp
		if err != nil {
			return nil, fmt.Errorf("http do: %v", err)
		}
		if res.StatusCode == 200 {
			break
		}

		if res.StatusCode == 429 || (res.StatusCode > 500 && res.StatusCode <= 600) {
			// Rate limit
			buf := new(bytes.Buffer)
			buf.ReadFrom(res.Body)
			log.Warnf("http %s: %v", res.Status, buf.String())

			// TODO randomized exponential backoff
			t := time.After(time.Second)
			select {
			case <-ctx.Done():
			case <-t:
			}
			continue
		}

		buf := new(bytes.Buffer)
		buf.ReadFrom(res.Body)
		return nil, fmt.Errorf("http %s: %v", res.Status, buf.String())
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("ctx: %v", err)
	}

	dec := json.NewDecoder(res.Body)
	resp := &Response{}
	if err := dec.Decode(resp); err != nil {
		return nil, fmt.Errorf("json decode: %v")
	}
	return resp, nil
}
