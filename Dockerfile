# syntax=docker/dockerfile:1.7

FROM debian:12 as base

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates wget && rm -rf /var/lib/apt/lists/*

RUN --mount=type=secret,id=secret_url wget -qO /GeoLite2-City.mmdb "$(cat /run/secrets/secret_url)"

FROM golang:alpine as build

WORKDIR /go/src/app

COPY . /go/src/app

RUN CGO_ENABLED=0 go build -ldflags '-extldflags "-static" -w -s' -tags timetzdata main.go

FROM scratch

COPY --from=build /go/src/app/main /main
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=base /GeoLite2-City.mmdb /go/src/app/GeoLite2-City.mmdb

ENV GEOIP_DB_PATH=/go/src/app/GeoLite2-City.mmdb

ENTRYPOINT ["/main"]
