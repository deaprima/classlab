# syntax=docker/dockerfile:1

# ---- build: static binaries, no CGO, so the runtime image needs nothing but CA certificates ----
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/createadmin ./cmd/createadmin

# ---- runtime ----
FROM alpine:3.21
RUN apk add --no-cache ca-certificates \
 && addgroup -S app && adduser -S -G app -u 10001 app \
 && mkdir -p /data/uploads && chown -R app:app /data
COPY --from=build /out/ /app/
ENV PORT=8080 UPLOAD_DIR=/data/uploads
USER app
EXPOSE 8080
# Letters and check-out photos live here; mount a volume so they outlive the container.
VOLUME ["/data/uploads"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD wget -q -O /dev/null "http://127.0.0.1:${PORT}/health" || exit 1
ENTRYPOINT ["/app/server"]
