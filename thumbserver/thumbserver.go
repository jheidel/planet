package thumbserver

import (
	"net/http"
	"planet-server/planet"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

type ThumbServer struct {
	Client *planet.Client
}

func New(p *planet.Client) *ThumbServer {
	return &ThumbServer{Client: p}
}

func (s *ThumbServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ID := mux.Vars(r)["id"]
	w.Header().Set("Content-Type", "image/png")
	if err := s.Client.FetchThumb(r.Context(), ID, w); err != nil {
		log.Errorf("thumb proxy failed: %v", err)
	}
}
