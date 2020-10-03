package planet

import (
	"sync"

	"github.com/hashicorp/go-retryablehttp"
	log "github.com/sirupsen/logrus"
)

var (
	client = retryablehttp.NewClient()
)

var once sync.Once

func planetClient() *retryablehttp.Client {
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
