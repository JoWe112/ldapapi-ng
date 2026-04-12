# ldapapi-ng

[![CI](https://github.com/JoWe112/ldapapi-ng/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/JoWe112/ldapapi-ng/actions/workflows/ci.yml)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/JoWe112/ldapapi-ng)](go.mod)
[![Go Report Card](https://goreportcard.com/badge/github.com/JoWe112/ldapapi-ng)](https://goreportcard.com/report/github.com/JoWe112/ldapapi-ng)

A small REST API that authenticates users and looks up attributes over LDAPS.
Designed to run behind an API gateway (KrakenD) or standalone with HTTP Basic
Auth validated against LDAP bind.

## Install

### Helm (recommended)

```sh
helm install ldapapi ./helm \
  --namespace ldapapi-ng --create-namespace \
  --set ldap.host=ldap.example.org \
  --set ldap.baseDN=dc=example,dc=org
```

This installs in **gateway mode** (default) — the API is only reachable from
an upstream gateway and a `NetworkPolicy` is created automatically.

For **standalone mode** (HTTP Basic Auth, no gateway):

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

### Container image

Pre-built images are published to the project registry:

```
core.51335.xyz/2025/ldapapi-ng:<version>
```

```sh
docker run --rm -p 8080:8080 \
  -e LDAP_HOST=ldap.example.org \
  -e LDAP_BASE_DN=dc=example,dc=org \
  core.51335.xyz/2025/ldapapi-ng:<version>
```

### KrakenD gateway

When running in gateway mode, the upstream KrakenD Helm chart fronts
`ldapapi-ng`. See [`deploy/krakend/README.md`](deploy/krakend/README.md) for
install instructions, endpoint matrix, and secret values reference.

## Endpoints

| Method | Path | Description |
| --- | --- | --- |
| GET | `/health` | Liveness / readiness probe — returns version and commit. |
| POST | `/v1/auth` | Validate credentials via LDAP bind (HTTP Basic). |
| GET | `/v1/user/:uid` | Fetch LDAP attributes for a user. |
| GET | `/swagger/*` | Swagger UI (only if `SWAGGER_ENABLED=true`). |

Error responses use a common envelope:

```json
{ "error": { "code": "INVALID_CREDENTIALS", "message": "..." } }
```

## Configuration

All settings are read from environment variables. When deploying with the Helm
chart these are set via `helm/values.yaml`.

| Variable | Default | Description |
| --- | --- | --- |
| `LOG_LEVEL` | `INFO` | Log verbosity: `DEBUG`, `INFO`, `WARN`, `ERROR`. |
| `LISTEN_ADDR` | `:8080` | HTTP listen address. |
| `AUTH_MODE` | `gateway` | `gateway` — gateway handles auth; `standalone` — API enforces HTTP Basic Auth via LDAP bind. |
| `LDAP_HOST` | *(required)* | LDAPS hostname. |
| `LDAP_PORT` | `636` | LDAPS port. |
| `LDAP_BASE_DN` | *(required)* | Search base, e.g. `dc=example,dc=org`. |
| `LDAP_BIND_DN` | *(empty)* | Service account DN for directory search. Required when anonymous search is disabled. |
| `LDAP_BIND_PASSWORD` | *(empty)* | Password for the service account. |
| `LDAP_USER_FILTER` | `(uid=%s)` | Search filter template; `%s` is replaced by the uid. |
| `LDAP_CA_CERT_PATH` | *(empty)* | Path to a CA cert PEM used to verify the LDAPS cert. |
| `LDAP_TIMEOUT` | `10s` | Dial / operation timeout. |
| `SWAGGER_ENABLED` | `false` | Expose `/swagger/*` when true. |
| `DEV_MODE` | `false` | Enable Gin debug mode (disallowed with real credentials). |

### LDAPS CA certificate

If your LDAP server uses a certificate signed by an internal CA, provide the
CA certificate so the API can verify the TLS connection.

**Inline PEM** in your Helm values:

```yaml
ldap:
  caCert:
    enabled: true
    content: |
      -----BEGIN CERTIFICATE-----
      MIIFaz...
      -----END CERTIFICATE-----
```

**Existing ConfigMap:**

```yaml
ldap:
  caCert:
    enabled: true
    existingConfigMap: my-ca-configmap
```

The chart mounts the certificate read-only at `/etc/ldapapi-ng/ca/ca.crt` and
sets `LDAP_CA_CERT_PATH` automatically. Override the path with
`ldap.caCert.mountPath` / `ldap.caCert.fileName`.

> `content` and `existingConfigMap` are mutually exclusive.

<details>
<summary><strong>LDAP service account explained</strong></summary>

`LDAP_BIND_DN` / `LDAP_BIND_PASSWORD` configure a **service account** used to
*search* the directory — it is independent of which auth mode the API runs in.

Every request does a two-step flow:

1. Bind as the service account and search for the user's full DN.
2. For `/v1/auth`, rebind as the user to verify the password. For
   `/v1/user/:uid`, return the attributes from the search.

| Directory policy | Service account needed? |
| --- | --- |
| Anonymous search **forbidden** (AD, hardened OpenLDAP) | **Yes** — without it the search fails with "user not found". |
| Anonymous search allowed | No — leave `LDAP_BIND_DN` empty. |

The end-user's own credentials cannot substitute for the service account
because at step 1 the API does not yet know the user's DN — that is the
search's whole purpose.

</details>

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
with OpenShift `restricted-v2` SCC.

```sh
docker buildx build \
  --platform linux/amd64 \
  --build-arg VERSION="$(git describe --tags --always)" \
  --build-arg COMMIT="$(git rev-parse --short HEAD)" \
  --build-arg DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -t core.51335.xyz/2025/ldapapi-ng:<version> \
  --load .
```

### Scan for vulnerabilities (Trivy)

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

### Verify Helm templates

```sh
helm lint ./helm --set ldap.host=x --set ldap.baseDN=y
helm template test ./helm --set ldap.host=x --set ldap.baseDN=y
```

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
