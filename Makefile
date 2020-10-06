ts := $(shell /bin/date "+%s")

.PHONY: all build clean

all: web/build build

web/build:
	cd web && $(MAKE)

build:
	go get -d -v
	go build -ldflags "-X main.BuildTimestamp=$(ts)"

clean:
	cd web && $(MAKE) clean
	rm -rf planet-server
