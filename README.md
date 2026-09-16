# demo-app (geo-weather-app)

Where the sky is: detects your approximate location from your IP (GeoLite2)
and shows the weather there right now (Open-Meteo), with an animated sky and
end-to-end distributed tracing (browser → gateway → geoip/weather, viewable
in Grafana Tempo).

Deployed at [demo.waflab.dev](https://demo.waflab.dev). This repo used to hold
the original `demo-app` (a single toy Go service used to shake out the K3s/
Envoy Gateway/HPA infra) — that's been fully replaced by this app, same repo,
new code (see git history for the old version). Infra lives in the sibling
`DevOps` repo (`~/Desktop/DevOps`, this repo lives at `DevOps/demo-app`);
this repo only owns app code, Dockerfiles and the `k8s/` manifests that get
applied into that cluster's `demo` namespace.

```
Browser (React, OTel Web SDK)
   │ fetch /api/whereami  (traceparent header)
   ▼
gateway-service  (public, HTTP) ──gRPC──> geoip-service   (GeoLite2-City.mmdb)
   │ continues trace                └─gRPC──> weather-service (Open-Meteo)
   ▼
Envoy Gateway (demo.waflab.dev): "/api/*" -> gateway-service, "/" -> frontend
   spans exported via OTLP/gRPC directly to Tempo (monitoring namespace)
```

## Versions

Checked live at implementation time, not from memory:

| Component                              | Version   |
|-----------------------------------------|-----------|
| Go                                       | 1.26.5    |
| React / Vite / TypeScript                | 19.3.0 / 8.3.0 / 7.0.2 |
| grpc-go / protobuf-go                    | v1.83.2 / v1.36.12 |
| OpenTelemetry Go SDK / otelgrpc          | v1.46.0 / v0.71.0 |
| OpenTelemetry JS (web SDK, exporter)     | 2.11.0 / 0.222.0 |
| oschwald/maxminddb-golang                | v2.6.0 |
| nginx-unprivileged (frontend image base) | 1.30.5-alpine |

## Repo layout

- `proto/` — shared `.proto` contracts (`geoip`, `weather`) + generated Go code.
- `gateway-service/` — public HTTP entrypoint, aggregates geoip+weather over gRPC.
- `geoip-service/` — gRPC server wrapping a GeoLite2-City `.mmdb` file.
- `weather-service/` — gRPC server calling Open-Meteo (no API key needed).
- `frontend/` — Vite/React app.
- `k8s/` — Deployments/Services/HPA/PVC for the `demo` namespace.

## Local development

```bash
cd proto && go build ./...

cd geoip-service && go build ./...     # needs MAXMIND_ACCOUNT_ID/MAXMIND_LICENSE_KEY env vars on first run
cd weather-service && go build ./...
cd gateway-service && go build ./...

cd frontend && npm install && npm run dev
```

## Credentials

Add to `~/Desktop/DevOps/.env` (same file the rest of the lab uses — never commit it):

```bash
MAXMIND_ACCOUNT_ID=...
MAXMIND_LICENSE_KEY=...
```

Get these from your MaxMind account (GeoLite2 sign-up is free). MaxMind enforces
an undisclosed daily download limit, so:
- The database only downloads if `/data/GeoLite2-City.mmdb` is missing (first boot / fresh PVC).
- `POST /internal/update-db` on `geoip-service` (cluster-internal only) forces a
  refresh, but refuses with `429` if less than 24h have passed since the last download.

## CI

`.github/workflows/build-push.yml` builds and pushes all 4 images to GHCR on
every push to `main` (tags: `sha-<short-sha>` and `latest`). It's build+push
only — nothing auto-deploys to the cluster, that stays a manual step below
with an explicit version tag (deliberate: the API server isn't public, and
this avoids storing kubeconfig in GitHub secrets for a lab).

## Build & push images (manual/local)

```bash
export GITHUB_TOKEN=...   # from ~/Desktop/DevOps/.env, needs write:packages
echo "$GITHUB_TOKEN" | docker login ghcr.io -u <your-github-username> --password-stdin

cd ~/Desktop/DevOps/demo-app
# Cluster nodes are amd64 (Hetzner cx23). On an Apple Silicon Mac, Docker
# builds arm64 by default -> always pin --platform or you'll get
# "exec format error" in the pod logs.
for svc in geoip-service weather-service gateway-service; do
  docker build --platform linux/amd64 -f "$svc/Dockerfile" -t "ghcr.io/redkiy88/$svc:1.0.1" .
  docker push "ghcr.io/redkiy88/$svc:1.0.1"
done

docker build --platform linux/amd64 -f frontend/Dockerfile -t ghcr.io/redkiy88/geo-weather-frontend:1.0.1 .
docker push ghcr.io/redkiy88/geo-weather-frontend:1.0.1
```

## Deploy

```bash
export KUBECONFIG=~/.kube/waflab-k3s.yaml   # SSH tunnel to the API must be up, see DevOps/FAQ.md

kubectl apply -f k8s/namespace.yaml

# Same ghcr-creds secret pattern as the old demo-app:
read -rsp "GHCR token (read:packages): " GHCR_TOKEN; echo
kubectl -n demo create secret docker-registry ghcr-creds \
  --docker-server=ghcr.io --docker-username=<your-github-username> --docker-password="$GHCR_TOKEN"
unset GHCR_TOKEN

read -rp  "MaxMind account ID: " MAXMIND_ACCOUNT_ID
read -rsp "MaxMind license key: " MAXMIND_LICENSE_KEY; echo
kubectl -n demo create secret generic maxmind \
  --from-literal=account_id="$MAXMIND_ACCOUNT_ID" --from-literal=license_key="$MAXMIND_LICENSE_KEY"
unset MAXMIND_ACCOUNT_ID MAXMIND_LICENSE_KEY

kubectl apply -f k8s/geoip-service.yaml -f k8s/weather-service.yaml \
  -f k8s/gateway-service.yaml -f k8s/frontend.yaml

kubectl -n demo rollout status deploy/geoip-service
kubectl -n demo rollout status deploy/weather-service
kubectl -n demo rollout status deploy/gateway-service
kubectl -n demo rollout status deploy/frontend
```

`kubernetes/demo/httproute.yaml` in the `DevOps` repo already routes
`/api` → `gateway-service` and `/` → `frontend`; no change needed there unless
paths change.

## Verify

```bash
curl https://demo.waflab.dev/api/whereami   # JSON with city/temperature
```

Open https://demo.waflab.dev in a browser, then in Grafana (`DevOps/FAQ.md` has
the port-forward command) → Explore → Tempo → search by the trace ID shown at
the bottom of the page — you'll see the full request broken down across
gateway-service → geoip-service / weather-service.
