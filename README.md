## Planet Data Viewer

[![Build Status](https://app.travis-ci.com/jheidel/planet-server.svg?branch=master)](https://app.travis-ci.com/jheidel/planet-server)

This project is a map tile server which wraps the [Planet Developer API](https://developers.planet.com/).
It implements on-demand mosaics and drill-down.

A public web frontend is available: https://planet.jeffheidel.com

This is just a hobby project so please don't post the link to social media.
If my AppEngine budget blows up I'll need to implement access controls.

Alternatively, run using the image from [Docker Hub](https://hub.docker.com/r/jheidel/planet-server):
```
docker run -p 80:8080 jheidel/planet-server
```

Built with [Polymer](https://www.polymer-project.org/) and [Go](https://golang.org/).
