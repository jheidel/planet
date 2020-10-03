package util

import (
	"os"
	"time"

	log "github.com/sirupsen/logrus"
)

func EnvOrDefault(key string, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func LocationOrDie() *time.Location {
	loc, err := time.LoadLocation(EnvOrDefault("TZ", "America/Los_Angeles"))
	if err != nil {
		log.Fatalf("Bad location configured %v", err)
	}
	return loc
}
