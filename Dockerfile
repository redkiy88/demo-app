# ---- Build stage ----
FROM golang:1.27.1-alpine3.24 AS build

WORKDIR /src

# Download dependencies first to benefit from layer caching.
COPY go.mod ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/demo-app .

# ---- Runtime stage ----
FROM alpine:3.24.1

RUN addgroup -S -g 10001 app && adduser -S -D -H -u 10001 -G app app

COPY --from=build /out/demo-app /usr/local/bin/demo-app

# Numeric UID so Kubernetes runAsNonRoot can verify it.
USER 10001:10001

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/demo-app"]
