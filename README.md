# ldapapi-ng

A small REST API that authenticates users and looks up attributes over LDAPS.
Written in Go, designed to run behind an API gateway (KrakenD) or standalone
with HTTP Basic Auth validated against LDAP bind.

## Endpoints

| Method | Path             | Description                                         |
|--------|------------------|-----------------------------------------------------|
| GET    | `/health`        | Liveness / readiness probe, returns version & commit. |
| POST   | `/v1/auth`       | Validate credentials via LDAP bind (HTTP Basic).    |
| GET    | `/v1/user/:uid`  | Fetch LDAP attributes for a user.                   |
| GET    | `/swagger/*`     | Swagger UI (only if `SWAGGER_ENABLED=true`).        |

All error responses use a common envelope:

```json
{ "error": { "code": "INVALID_CREDENTIALS", "message": "..." } }
```

## Configuration

Configuration is read from environment variables.

| Variable              | Default        | Description                                              |
|-----------------------|----------------|----------------------------------------------------------|
| `LISTEN_ADDR`         | `:8080`        | HTTP listen address.                                     |
| `LDAP_HOST`           | *(required)*   | LDAPS hostname.                                          |
| `LDAP_PORT`           | `636`          | LDAPS port.                                              |
| `LDAP_BASE_DN`        | *(required)*   | Search base, e.g. `dc=example,dc=org`.                   |
| `LDAP_BIND_DN`        | *(empty)*      | Service account DN used to search the directory. See [LDAP service account](#ldap-service-account) below. |
| `LDAP_BIND_PASSWORD`  | *(empty)*      | Password for the service account.                        |
| `LDAP_USER_FILTER`    | `(uid=%s)`     | Search filter template; `%s` is replaced by the uid.     |
| `LDAP_CA_CERT_PATH`   | *(empty)*      | Path to a CA cert PEM used to verify the LDAPS cert.     |
| `LDAP_TIMEOUT`        | `10s`          | Dial / operation timeout.                                |
| `AUTH_MODE`           | `gateway`      | `gateway` or `standalone`.                               |
| `SWAGGER_ENABLED`     | `false`        | Expose `/swagger/*` when true.                           |
| `DEV_MODE`            | `false`        | Enable Gin debug mode (disallowed with real credentials).|

### Auth modes

- **gateway**: an upstream gateway (e.g. KrakenD) handles authentication and
  a NetworkPolicy must restrict ingress to the gateway only.
- **standalone**: the API itself enforces HTTP Basic Auth by performing an
  LDAP bind with the submitted credentials.

### LDAP service account

`LDAP_BIND_DN` / `LDAP_BIND_PASSWORD` configure a **service account** used to
*search* the directory. Every request — both `POST /v1/auth` and `GET /v1/user/:uid` — does a two-step flow:

1. Bind as the service account and run `LDAP_USER_FILTER` under `LDAP_BASE_DN`
   to resolve the user's full DN.
2. For `/v1/auth`, rebind as the resolved DN with the end-user's password to
   verify the credentials. For `/v1/user/:uid`, return the attributes from the
   search result.

Whether you need the service account depends on your **directory's ACLs**,
not on which auth mode the API runs in:

| Directory policy | `LDAP_BIND_DN` / `_PASSWORD` |
|---|---|
| Anonymous search is **forbidden** (Active Directory, most hardened OpenLDAP) | **Required** — without it the search in step 1 returns nothing and auth fails with "user not found" even for correct passwords. |
| Anonymous search is allowed | Optional — leave empty to skip the service bind. |

The end-user's own credentials (from HTTP Basic Auth in standalone mode, or
from the JWT in gateway mode) cannot substitute for the service account,
because at step 1 the API does not yet know the user's DN — that is the
search's whole purpose.

## Build

```sh
go build ./...
```

A release build is produced with version metadata:

```sh
go build -trimpath \
  -ldflags "-s -w \
    -X github.com/JoWe112/ldapapi-ng/internal/version.Version=$(git describe --tags --always) \
    -X github.com/JoWe112/ldapapi-ng/internal/version.Commit=$(git rev-parse --short HEAD) \
    -X github.com/JoWe112/ldapapi-ng/internal/version.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  ./cmd/ldapapi-ng
```

## Test

```sh
go test ./... -race
go vet ./...
gofmt -l .
```

## Regenerating the OpenAPI spec

```sh
go install github.com/swaggo/swag/cmd/swag@latest
swag init -g cmd/ldapapi-ng/main.go -o docs --parseInternal
```

## Container image

The project ships a multi-stage `Dockerfile` producing a small, OpenShift-compatible image based on `alpine:3.22`. The builder stage runs on the host's native architecture and Go cross-compiles to `linux/amd64`, so the build stays fast even on Apple Silicon.

### Build

```sh
docker buildx build \
  --platform linux/amd64 \
  --build-arg VERSION="$(git describe --tags --always)" \
  --build-arg COMMIT="$(git rev-parse --short HEAD)" \
  --build-arg DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -t core.51335.xyz/2025/ldapapi-ng:<version> \
  --load .
```

Push with `--push` instead of `--load` once ready.

### Run locally

```sh
docker run --rm -p 8080:8080 \
  -e LDAP_HOST=ldap.example.org \
  -e LDAP_BASE_DN=dc=example,dc=org \
  -e LDAP_CA_CERT_PATH=/etc/ssl/certs/ca.pem \
  -v /path/to/ca.pem:/etc/ssl/certs/ca.pem:ro \
  core.51335.xyz/2025/ldapapi-ng:<version>
```

### Scan for vulnerabilities (Trivy)

Trivy is run as a container — nothing is installed on the host:

```sh
docker run --rm \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$HOME/.cache/trivy:/root/.cache/trivy" \
  aquasec/trivy:0.69.3 image \
    --severity HIGH,CRITICAL --exit-code 1 \
    core.51335.xyz/2025/ldapapi-ng:<version>
```

The registry rejects images with HIGH or CRITICAL findings, so always scan before pushing.

## Helm chart

A Helm chart is included under `helm/` and supports both authentication topologies via `auth.mode`.

### Gateway mode (default)

The API is only reachable from an upstream gateway. A `NetworkPolicy` is installed automatically. `ingress.enabled` and `httpRoute.enabled` must stay false.

```sh
helm install ldapapi ./helm \
  --set ldap.host=ldap.example.org \
  --set ldap.baseDN=dc=example,dc=org \
  --set networkPolicy.gatewayNamespaceSelector.matchLabels."kubernetes\.io/metadata\.name"=krakend
```

### Standalone mode

The API enforces HTTP Basic Auth via LDAP bind and is exposed directly.

```sh
helm install ldapapi ./helm \
  --set auth.mode=standalone \
  --set ldap.host=ldap.example.org \
  --set ldap.baseDN=dc=example,dc=org \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=ldapapi.example.org \
  --set ingress.hosts[0].paths[0].path=/ \
  --set ingress.hosts[0].paths[0].pathType=Prefix
```

### Verify rendering without deploying

```sh
helm lint ./helm --set ldap.host=x --set ldap.baseDN=y
helm template test ./helm --set ldap.host=x --set ldap.baseDN=y
```

The chart is designed to run unchanged on both OpenShift (`restricted-v2` SCC) and vanilla Kubernetes — `runAsUser` is intentionally left unset so OpenShift can assign a random UID from the namespace range while vanilla K8s falls back to the image's `USER 1001`.

## KrakenD gateway

When running in gateway mode, the upstream KrakenD Helm chart fronts `ldapapi-ng`. The configuration template, ConfigMap, example Secret, and values override live under [`deploy/krakend/`](deploy/krakend/). See [`deploy/krakend/README.md`](deploy/krakend/README.md) for install instructions and the full endpoint matrix. JWT routes validate tokens against a Keycloak JWKS; a parallel `/apikey/v1/*` route group accepts `X-API-Key` for service-to-service callers.

## Project layout

```
cmd/ldapapi-ng/     Main entrypoint
internal/config/    Env-based configuration loader
internal/handler/   Gin handlers, middleware, router
internal/ldap/      LDAPS client (auth + search)
internal/version/   Build-time metadata (-ldflags)
docs/               Generated OpenAPI / Swagger spec
helm/               Helm chart
deploy/krakend/     KrakenD gateway config + k8s manifests
Dockerfile          Multi-stage container build
```
