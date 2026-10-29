# One image, one binary: the dashboard, public tracking, the driver app and
# background jobs. Templates, static files and migrations are embedded.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/menged ./cmd/server

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -H -u 10001 menged \
 && mkdir -p /data/uploads /data/keys \
 && chown -R menged /data
COPY --from=build /out/menged /usr/local/bin/menged
USER menged
WORKDIR /data
ENV PORT=8080 \
    STORAGE_PATH=/data/uploads \
    APP_SECRET_FILE=/data/keys/app_secret
VOLUME /data
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s \
  CMD wget -qO- http://127.0.0.1:8080/readyz >/dev/null || exit 1
ENTRYPOINT ["menged"]
CMD ["serve"]
