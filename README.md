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
| `LDAP_BIND_DN`        | *(empty)*      | Optional service account DN used to resolve user DNs.   |
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

## Project layout

```
cmd/ldapapi-ng/     Main entrypoint
internal/config/    Env-based configuration loader
internal/handler/   Gin handlers, middleware, router
internal/ldap/      LDAPS client (auth + search)
internal/version/   Build-time metadata (-ldflags)
docs/               Generated OpenAPI / Swagger spec
```
