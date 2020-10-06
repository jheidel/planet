####
# Build the web frontend
####

FROM alpine AS builder-web
WORKDIR /web/
RUN apk add --no-cache nodejs nodejs-npm make

# Make sure npm is up to date
RUN npm install -g npm

# Install yarn for web dependency management
RUN npm install -g yarn

# Install polymer CLI
RUN yarn global add polymer-cli

# Copy web source files
COPY web/ .

# Build the frontend
RUN make

####
# Build the go binary
####

FROM golang:alpine AS builder-go
RUN apk add --no-cache git make
WORKDIR /go/src/planet-server/

# Copy all source files.
COPY . .

# Copy built web package from the previous stage.

# Build the standalone executable.
RUN make build

####
# Compose everything into the final minimal image.
####

FROM nginx:alpine
WORKDIR /app
COPY --from=builder-go /go/src/planet-server/planet-server /app/
COPY --from=builder-web /web/build/es6-bundled/ /web/

# Use local timezone.
# TODO use system time instead of hardcoded.
RUN apk add --update tzdata
ENV TZ=America/Los_Angeles

ENTRYPOINT ["/docker-entrypoint.sh"]

# TODO: create nginx config file to host static web assets from /web and proxy
# api requests to :8080

EXPOSE 80
CMD ["nginx && ./planet-server", "--port", "8080"]
