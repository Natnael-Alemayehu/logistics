# One image, one binary: the dashboard, public tracking, the driver app and
# background jobs. Templates, static files and migrations are embedded.
# Go's security fixes reach only its two newest releases: keep this on the
# newer one, and CI's go-version with it.
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/kiflet ./cmd/server

# The provider sandbox (--target sandbox): its own image, for development
# and staging. The app's image, the last stage, never contains it.
FROM build AS sandbox-build
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/kiflet-sandbox ./cmd/sandbox

FROM alpine:3.24 AS sandbox
# /keys belongs to the sandbox, so a new volume there takes its owner and
# `kiflet-sandbox keys` can write the key pairs into it.
RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -H -u 10001 sandbox \
 && mkdir /keys \
 && chown sandbox /keys
COPY --from=sandbox-build /out/kiflet-sandbox /usr/local/bin/kiflet-sandbox
USER sandbox
# Its pages show the time as a phone in Ethiopia would.
ENV SANDBOX_ADDR=0.0.0.0:8091 \
    SANDBOX_KEYS_DIR=/keys \
    TZ=Africa/Addis_Ababa
EXPOSE 8091
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s \
  CMD wget -qO- http://127.0.0.1:8091/healthz >/dev/null || exit 1
ENTRYPOINT ["kiflet-sandbox"]
CMD ["serve"]

FROM alpine:3.24
RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -H -u 10001 kiflet \
 && mkdir -p /data/uploads /data/keys \
 && chown -R kiflet /data
COPY --from=build /out/kiflet /usr/local/bin/kiflet
USER kiflet
WORKDIR /data
ENV PORT=8080 \
    STORAGE_PATH=/data/uploads \
    APP_SECRET_FILE=/data/keys/app_secret
VOLUME /data
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s \
  CMD wget -qO- http://127.0.0.1:8080/readyz >/dev/null || exit 1
ENTRYPOINT ["kiflet"]
CMD ["serve"]
