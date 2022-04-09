ts := $(shell /bin/date "+%s")

.PHONY: all build clean

all: web/build build

web/build:
	cd web && $(MAKE)

build:
	go mod download
	go get -d -v  # Attempt to upgrade
	go build -ldflags "-X main.BuildTimestamp=$(ts)" -buildvcs=false

clean:
	cd web && $(MAKE) clean
	rm -rf planet-server
