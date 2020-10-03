package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"planet-server/metaserver"
	"planet-server/planet"
	"planet-server/thumbserver"
	"planet-server/tileserver"
	"syscall"

	"github.com/gorilla/mux"

	log "github.com/sirupsen/logrus"
)

var (
	port  = flag.Int("port", 8080, "Serving port")
	debug = flag.Bool("debug", false, "Enable debug logging verbosity")
)

func topLevelContext() context.Context {
	ctx, cancelf := context.WithCancel(context.Background())
	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigs
		log.Warnf("Caught signal %q, shutting down.", sig)
		cancelf()
	}()
	return ctx
}

func main() {
	ctx := topLevelContext()

	flag.Parse()

	if *debug {
		log.SetLevel(log.DebugLevel)
		log.Debugf("Debug logging enabled")
	}

	pl := planet.New(ctx)
	ts := tileserver.New(pl)
	ms := metaserver.New(pl)
	ths := thumbserver.New(pl)

	router := mux.NewRouter()
	router.Handle("/api/tile/{z:[0-9]+}/{x:[0-9]+}/{y:[0-9]+}.png", ts).Methods("GET")
	router.Handle("/api/thumb/{id:[A-Za-z0-9_-]+}.png", ths).Methods("GET")
	router.Handle("/api/search", ms).Methods("GET")
	router.HandleFunc("/api/key", pl.ServeKeySaveHandler).Methods("POST")

	router.PathPrefix("/debug/pprof/").Handler(http.DefaultServeMux)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: router,
	}
	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()
	log.Infof("Starting HTTP server on port %d", *port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("ListenAndServe(): %v", err)
	}
	log.Infof("Shutdown")
}
