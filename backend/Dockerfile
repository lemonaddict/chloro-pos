# syntax=docker/dockerfile:1
#
# API only: the Go server, no frontend, no STATIC_DIR. This is the image for the Render Web Service.
# Build context is backend/ (render.yaml sets dockerContext). The all-in-one image stays in the
# Dockerfile at the repository root.
#
#   docker build -t chloropos-api backend
#   docker run --rm -p 8080:8080 -e DATABASE_URL=... -e JWT_SECRET=... -e ALLOWED_ORIGINS=https://app.example.com chloropos-api

# 1) Build a static Go binary. Keep the Go version in step with `go` in go.mod.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/chloropos ./cmd/api

# 2) Small runtime image: one binary, non-root user.
FROM alpine:3.21
RUN addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=build /out/chloropos /app/chloropos
# Production listens on $PORT (Render sets it) and falls back to :8080 when it is not set.
ENV APP_ENV=production
USER app
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s \
  CMD wget -q -O /dev/null "http://127.0.0.1:${PORT:-8080}/healthz" || exit 1
ENTRYPOINT ["/app/chloropos"]
