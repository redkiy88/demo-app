# Build context is the repo root: docker build -f weather-service/Dockerfile .
FROM golang:1.26.5-alpine AS build
WORKDIR /src
COPY proto/ proto/
COPY weather-service/ weather-service/
WORKDIR /src/weather-service
RUN CGO_ENABLED=0 go build -o /out/weather-service .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/weather-service /weather-service
USER nonroot:nonroot
ENTRYPOINT ["/weather-service"]
