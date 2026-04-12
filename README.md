# ldapapi-ng

[![CI](https://github.com/JoWe112/ldapapi-ng/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/JoWe112/ldapapi-ng/actions/workflows/ci.yml)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/JoWe112/ldapapi-ng)](go.mod)
[![Go Report Card](https://goreportcard.com/badge/github.com/JoWe112/ldapapi-ng)](https://goreportcard.com/report/github.com/JoWe112/ldapapi-ng)

A small REST API that authenticates users and looks up attributes over LDAPS.
Designed to run behind an API gateway (KrakenD) or standalone with HTTP Basic
Auth validated against LDAP bind.

---

## Deploy

### Helm (recommended)

A Helm chart is included under `helm/`. It supports two authentication
topologies controlled by `auth.mode`.

**Gateway mode** (default) — the API is only reachable from an upstream
gateway. A `NetworkPolicy` is installed automatically:

```sh
helm install ldapapi ./helm \
  --namespace ldapapi-ng --create-namespace \
  --set ldap.host=ldap.example.org \
  --set ldap.baseDN=dc=example,dc=org \
  --set networkPolicy.gatewayNamespaceSelector.matchLabels."kubernetes\.io/metadata\.name"=krakend
```

**Standalone mode** — the API enforces HTTP Basic Auth via LDAP bind and is
exposed directly:

```sh
helm install ldapapi ./helm \
  --namespace ldapapi-ng --create-namespace \
  --set auth.mode=standalone \
  --set ldap.host=ldap.example.org \
  --set ldap.baseDN=dc=example,dc=org \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=ldapapi.example.org \
  --set ingress.hosts[0].paths[0].path=/ \
  --set ingress.hosts[0].paths[0].pathType=Prefix
```

Verify rendering without deploying:

```sh
helm lint ./helm --set ldap.host=x --set ldap.baseDN=y
helm template test ./helm --set ldap.host=x --set ldap.baseDN=y
```

The chart runs unchanged on both OpenShift (`restricted-v2` SCC) and vanilla
Kubernetes.

### KrakenD gateway

When running in gateway mode, the upstream KrakenD Helm chart fronts
`ldapapi-ng`. Configuration, example Secret, and install instructions live in
[`deploy/krakend/`](deploy/krakend/) — see
[`deploy/krakend/README.md`](deploy/krakend/README.md) for the full endpoint
matrix and secret values reference.

JWT routes validate tokens against a Keycloak JWKS; a parallel
`/apikey/v1/*` route group accepts `X-API-Key` for service-to-service callers.

### Container image

Pre-built images are published to the project registry:

```
core.51335.xyz/2025/ldapapi-ng:<version>
```

To run locally:

```sh
docker run --rm -p 8080:8080 \
  -e LDAP_HOST=ldap.example.org \
  -e LDAP_BASE_DN=dc=example,dc=org \
  -e LDAP_CA_CERT_PATH=/etc/ssl/certs/ca.pem \
  -v /path/to/ca.pem:/etc/ssl/certs/ca.pem:ro \
  core.51335.xyz/2025/ldapapi-ng:<version>
```

---

## Configuration

All settings are read from environment variables.

| Variable | Default | Description |
| --- | --- | --- |
| `LOG_LEVEL` | `INFO` | Log verbosity: `DEBUG`, `INFO`, `WARN`, `ERROR`. |
| `LISTEN_ADDR` | `:8080` | HTTP listen address. |
| `AUTH_MODE` | `gateway` | `gateway` or `standalone` (see [Auth modes](#auth-modes)). |
| `LDAP_HOST` | *(required)* | LDAPS hostname. |
| `LDAP_PORT` | `636` | LDAPS port. |
| `LDAP_BASE_DN` | *(required)* | Search base, e.g. `dc=example,dc=org`. |
| `LDAP_BIND_DN` | *(empty)* | Service account DN for directory search (see [LDAP service account](#ldap-service-account)). |
| `LDAP_BIND_PASSWORD` | *(empty)* | Password for the service account. |
| `LDAP_USER_FILTER` | `(uid=%s)` | Search filter template; `%s` is replaced by the uid. |
| `LDAP_CA_CERT_PATH` | *(empty)* | Path to a CA cert PEM used to verify the LDAPS cert. |
| `LDAP_TIMEOUT` | `10s` | Dial / operation timeout. |
| `SWAGGER_ENABLED` | `false` | Expose `/swagger/*` when true. |
| `DEV_MODE` | `false` | Enable Gin debug mode (disallowed with real credentials). |

When deploying with the Helm chart, these are set via `helm/values.yaml` —
see the chart's inline comments for the full value reference.

---

## API usage

### Endpoints

| Method | Path | Description |
| --- | --- | --- |
| GET | `/health` | Liveness / readiness probe, returns version and commit. |
| POST | `/v1/auth` | Validate credentials via LDAP bind (HTTP Basic). |
| GET | `/v1/user/:uid` | Fetch LDAP attributes for a user. |
| GET | `/swagger/*` | Swagger UI (only if `SWAGGER_ENABLED=true`). |

All error responses use a common envelope:

```json
{ "error": { "code": "INVALID_CREDENTIALS", "message": "..." } }
```

### Auth modes

| Mode | How it works |
| --- | --- |
| **gateway** | An upstream gateway (e.g. KrakenD) handles authentication. A NetworkPolicy restricts ingress to the gateway pods only. |
| **standalone** | The API itself enforces HTTP Basic Auth by performing an LDAP bind with the submitted credentials. |

### LDAP service account

`LDAP_BIND_DN` / `LDAP_BIND_PASSWORD` configure a **service account** used to
*search* the directory — it is independent of which auth mode the API runs in.

Every request does a two-step flow:

1. Bind as the service account and search for the user's full DN.
2. For `/v1/auth`, rebind as the user to verify the password. For
   `/v1/user/:uid`, return the attributes from the search.

| Directory policy | Service account needed? |
| --- | --- |
| Anonymous search is **forbidden** (Active Directory, most hardened OpenLDAP) | **Yes** — without it the search fails with "user not found". |
| Anonymous search is allowed | No — leave `LDAP_BIND_DN` empty. |

---

## Development

### Build from source

```sh
go build ./...
```

Release build with version metadata:

```sh
go build -trimpath \
  -ldflags "-s -w \
    -X github.com/JoWe112/ldapapi-ng/internal/version.Version=$(git describe --tags --always) \
    -X github.com/JoWe112/ldapapi-ng/internal/version.Commit=$(git rev-parse --short HEAD) \
    -X github.com/JoWe112/ldapapi-ng/internal/version.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  ./cmd/ldapapi-ng
```

### Test

Run the unit tests with race detection:

```sh
go test ./... -race
```

Lint and format checks (also run in CI):

```sh
go vet ./...
gofmt -l .
```

### Build container image

The multi-stage `Dockerfile` produces a small Alpine-based image compatible
with OpenShift `restricted-v2` SCC. The builder stage uses Go cross-compilation
so the build stays fast even on Apple Silicon.

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

### Regenerate the OpenAPI spec

```sh
go install github.com/swaggo/swag/cmd/swag@latest
swag init -g cmd/ldapapi-ng/main.go -o docs --parseInternal
```

---

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

## License

Licensed under the [Apache License, Version 2.0](LICENSE). See [`NOTICE`](NOTICE) for the copyright line.
