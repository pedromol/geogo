# syntax=docker/dockerfile:1.7

FROM debian:12 as base

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates wget gzip && rm -rf /var/lib/apt/lists/*

RUN --mount=type=secret,id=secret_url \
	wget -qO /tmp/GeoLite2-City.db "$(cat /run/secrets/secret_url)" && \
	if gzip -t /tmp/GeoLite2-City.db 2>/dev/null; then \
		gunzip -c /tmp/GeoLite2-City.db > /GeoLite2-City.mmdb; \
	else \
		mv /tmp/GeoLite2-City.db /GeoLite2-City.mmdb; \
	fi

FROM golang:alpine as build

WORKDIR /go/src/app

COPY . /go/src/app

RUN CGO_ENABLED=0 go build -o /go/src/app/main -ldflags '-extldflags "-static" -w -s' -tags timetzdata .

FROM scratch

COPY --from=build /go/src/app/main /main
COPY --from=base /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=base /GeoLite2-City.mmdb /go/src/app/GeoLite2-City.mmdb

ENV GEOIP_DB_PATH=/go/src/app/GeoLite2-City.mmdb

ENTRYPOINT ["/main"]
