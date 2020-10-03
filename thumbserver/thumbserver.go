package thumbserver

import (
	"net/http"
	"planet-server/planet"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

type ThumbServer struct {
}

func New() *ThumbServer {
	return &ThumbServer{}
}

func (s *ThumbServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ID := mux.Vars(r)["id"]
	w.Header().Set("Content-Type", "image/png")
	if err := planet.FetchThumb(r.Context(), ID, w); err != nil {
		log.Errorf("thumb proxy failed: %v", err)
	}
}
