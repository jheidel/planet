package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"planet-server/tileserver"
	"syscall"

	"github.com/gorilla/mux"

	log "github.com/sirupsen/logrus"
)

var (
	port  = flag.Int("port", 8080, "Serving port")
	debug = flag.Bool("debug", true, "Enable debug logging verbosity")
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

	ts := tileserver.New()

	router := mux.NewRouter()
	router.Handle("/api/tile/{z:[0-9]+}/{x:[0-9]+}/{y:[0-9]+}.png", ts).Methods("GET")

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
