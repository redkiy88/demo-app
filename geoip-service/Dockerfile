# Build context is the repo root: docker build -f geoip-service/Dockerfile .
FROM golang:1.26.5-alpine AS build
WORKDIR /src
COPY proto/ proto/
COPY geoip-service/ geoip-service/
WORKDIR /src/geoip-service
RUN CGO_ENABLED=0 go build -o /out/geoip-service .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/geoip-service /geoip-service
USER nonroot:nonroot
ENTRYPOINT ["/geoip-service"]
