# Build context is the repo root: docker build -f gateway-service/Dockerfile .
FROM golang:1.26.5-alpine AS build
WORKDIR /src
COPY proto/ proto/
COPY gateway-service/ gateway-service/
WORKDIR /src/gateway-service
RUN CGO_ENABLED=0 go build -o /out/gateway-service .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/gateway-service /gateway-service
USER nonroot:nonroot
ENTRYPOINT ["/gateway-service"]
