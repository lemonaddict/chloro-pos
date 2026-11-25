# syntax=docker/dockerfile:1

# 1) Build the Svelte app
FROM node:22-alpine AS web
WORKDIR /web
COPY frontend/package.json frontend/package-lock.json* ./
RUN if [ -f package-lock.json ]; then npm ci; else npm install; fi
COPY frontend/ ./
RUN npm run build

# 2) Build a static Go binary
FROM golang:1.26-alpine AS api
WORKDIR /src
COPY backend/ ./
# Resolves modules. Drop `go mod tidy` once go.sum is committed.
RUN go mod tidy && go mod download
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/chloropos ./cmd/api

# 3) Small runtime image: one binary plus the built frontend, running as a non-root user
FROM alpine:3.21
RUN addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=api /out/chloropos /app/chloropos
COPY --from=web /web/dist /app/web
ENV STATIC_DIR=/app/web HTTP_ADDR=:8080 APP_ENV=production
USER app
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/app/chloropos"]
