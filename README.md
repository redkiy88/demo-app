# demo-app

A tiny, Go HTTP service

## Endpoints

| Method | Path        | Response                                         |
|--------|-------------|--------------------------------------------------|
| GET    | `/`         | `{"service","version","hostname"}`               |
| GET    | `/health`   | `200 {"status":"ok"}` (liveness)                 |
| GET    | `/ready`    | `200 {"status":"ready"}`, `503` while shutting down (readiness) |
| GET    | `/hostname` | `{"hostname"}`                                   |

The server listens on `:8080`, logs JSON to stdout, and shuts down gracefully
on `SIGTERM`/`SIGINT` (in-flight requests get up to 15s to finish).

## Run locally

```sh
go run .
# with a version:
go run -ldflags "-X main.version=1.0.0" .
```

```sh
curl localhost:8080/
curl localhost:8080/health
curl localhost:8080/ready
curl localhost:8080/hostname
```

## Run with Docker

```sh
docker build --build-arg VERSION=1.0.0 -t demo-app:1.0.0 .
docker run --rm -p 8080:8080 demo-app:1.0.0
```

## Files

```
.
├── main.go        # the whole app: handlers, server timeouts, graceful shutdown
├── go.mod         # module definition (no external dependencies)
├── Dockerfile     # multi-stage build → small Alpine image, non-root user
├── .dockerignore  # keeps the build context small
└── README.md
```
