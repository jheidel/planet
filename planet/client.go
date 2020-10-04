package planet

import (
	"context"
	"net/http"
	"planet-server/util"
	"sync"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/hashicorp/go-retryablehttp"
	log "github.com/sirupsen/logrus"
)

var (
	client = retryablehttp.NewClient()
)

var once sync.Once

func planetHTTP() *retryablehttp.Client {
	once.Do(func() {
		client = retryablehttp.NewClient()
		client.Logger = nil
		if log.GetLevel() >= log.DebugLevel {
			client.Logger = log.StandardLogger()
		}
		client.ErrorHandler = retryablehttp.PassthroughErrorHandler
	})
	return client
}

type Client struct {
	APIKey string
	lock   sync.Mutex
}

func New(ctx context.Context) *Client {
	cl := &Client{
		APIKey: util.EnvOrDefault("PLANET_API_KEY", ""),
	}
	go cl.GetAPIKey(ctx) // warm up key
	return cl
}

type AppSettings struct {
	PlanetAPIKey string `datastore:"planet_api_key"`
}

func (p *Client) GetAPIKey(pctx context.Context) string {
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.APIKey != "" {
		return p.APIKey
	}

	ctx, cancel := context.WithDeadline(pctx, time.Now().Add(30*time.Second))
	defer cancel()

	log.Infof("Fetching API key from datastore")
	ds, err := datastore.NewClient(ctx, util.EnvOrDefault("PROJECT_ID", "jheidel-planet"))
	if err != nil {
		log.Errorf("Failed to connect to datastore: %v", err)
		return ""
	}
	var settings AppSettings

	key := datastore.NameKey("settings", "settings", nil)
	if err := ds.Get(ctx, key, &settings); err != nil {
		log.Errorf("datastore settings Get: %v", err)
		return ""
	}

	p.APIKey = settings.PlanetAPIKey
	return p.APIKey
}

func (p *Client) SaveAPIKey(pctx context.Context, key string) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.APIKey = key
	go func(pctx context.Context) {
		ctx, cancel := context.WithDeadline(pctx, time.Now().Add(30*time.Second))
		defer cancel()

		log.Infof("Persisting API key to datastore")
		ds, err := datastore.NewClient(ctx, util.EnvOrDefault("PROJECT_ID", "jheidel-planet"))
		if err != nil {
			log.Errorf("Failed to connect to datastore: %v", err)
			return
		}

		var settings AppSettings
		p.lock.Lock()
		settings.PlanetAPIKey = p.APIKey
		p.lock.Unlock()

		key := datastore.NameKey("settings", "settings", nil)
		if _, err := ds.Put(ctx, key, &settings); err != nil {
			log.Errorf("datastore settings Put: %v", err)
		}
	}(pctx)
}

func (p *Client) ServeKeySaveHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	key := r.Form.Get("key")
	if key == "" {
		http.Error(w, "missing key", http.StatusBadRequest)
		return
	}
	// TODO would be a good idea to check that the key is valid first with a test API call.
	log.Infof("Saving API key %q", key)
	p.SaveAPIKey(context.Background(), key)
}
