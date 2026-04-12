# KrakenD gateway for ldapapi-ng

This directory contains the KrakenD gateway configuration that fronts
`ldapapi-ng` when it runs in **gateway mode**. In this topology KrakenD is the
only component reachable from outside the cluster; `ldapapi-ng` itself is
locked down to accept traffic only from the KrakenD pods via a NetworkPolicy
(see `helm/templates/networkpolicy.yaml`).

```
client ──TLS──▶ Ingress / HTTPRoute ──▶ KrakenD ──cluster HTTP──▶ ldapapi-ng ──LDAPS──▶ Directory
                                         (JWT / API key)
```

## Files

| File | Purpose |
| --- | --- |
| `krakend.tmpl` | Standalone copy of the Flexible-Config template (for reference / local testing). The same content is inlined in `k8s/values.yaml` under `krakend.config`. |
| `k8s/values.yaml` | Values override for the upstream [KrakenD Helm chart](https://github.com/equinixmetal-helm/krakend) (maintained by Equinix Metal). Contains the full config template inline — the chart renders it into a ConfigMap automatically. |
| `k8s/secret.example.yaml` | Example Secret with every env var the template expects — copy, populate, and apply the copy (**never commit populated secrets**). |

## Endpoints

Two parallel route groups are exposed — pick whichever auth method the caller
can use. Backends all point at `{{ env "LDAPAPI_UPSTREAM" }}`.

| Path | Method | Auth | Upstream |
| --- | --- | --- | --- |
| `/health` | GET | none (liveness) | `/health` |
| `/v1/auth` | POST | JWT (`auth/validator`, Keycloak JWKS) | `/v1/auth` |
| `/v1/user/{uid}` | GET | JWT | `/v1/user/{uid}` |
| `/apikey/v1/auth` | POST | API key (`auth/api-keys`, header `X-API-Key`) | `/v1/auth` |
| `/apikey/v1/user/{uid}` | GET | API key | `/v1/user/{uid}` |

JWTs are validated with `alg: RS256`, the `iss` and `aud` claims must match
`KEYCLOAK_ISSUER` / `KEYCLOAK_AUDIENCE`, and the caller must carry the
`ldapapi-user` role under `realm_access.roles`. For the JWT routes the
`preferred_username` claim is propagated to the upstream as
`X-Forwarded-User`.

## Secret values (krakend-secrets)

All configuration that varies per environment lives in a Kubernetes Secret
named `krakend-secrets`. The chart injects them as environment variables via
`krakend.envFromSecret`, and KrakenD's Flexible Config resolves the
`{{ env "VAR" }}` placeholders at pod startup.

| Variable | Required | Example | Description |
| --- | --- | --- | --- |
| `LDAPAPI_UPSTREAM` | **yes** | `http://ldapapi-ng.ldapapi-ng.svc.cluster.local:8080` | Full URL to the ldapapi-ng Service. **Must use the fully-qualified domain** (`<service>.<namespace>.svc.cluster.local`) — the short form (`svc` without `.cluster.local`) does not resolve reliably across namespaces. The port must match `api.listenAddr` in the ldapapi-ng Helm values (default `8080`). |
| `KEYCLOAK_JWKS_URL` | **yes** (JWT routes) | `https://keycloak.example.org/realms/myrealm/protocol/openid-connect/certs` | Keycloak realm JWKS endpoint. KrakenD fetches and caches the signing keys from this URL to validate JWT tokens on the `/v1/*` routes. Must be reachable from the KrakenD pods. |
| `KEYCLOAK_ISSUER` | **yes** (JWT routes) | `https://keycloak.example.org/realms/myrealm` | Expected `iss` claim in the JWT. Must match the issuer your Keycloak realm advertises — typically the realm URL without a trailing slash. |
| `KEYCLOAK_AUDIENCE` | **yes** (JWT routes) | `ldapapi-ng` | Expected `aud` claim (the Keycloak client ID). Tokens without this audience are rejected. |
| `KRAKEND_API_KEYS_JSON` | **yes** (API key routes) | `[{"key":"QNl1...","roles":["ldapapi-user"]}]` | Raw JSON array injected verbatim into the config. Each entry must have `key` (the API key string) and `roles` (must include `ldapapi-user`). Multiple keys are supported. Keep this value opaque — avoid logging or exposing it. |

### Constructing `LDAPAPI_UPSTREAM`

The URL follows the pattern:

```
http://<helm-release-name>.<namespace>.svc.cluster.local:<port>
```

For example, if you installed the ldapapi-ng Helm chart with:

```sh
helm install ldapapi ./helm --namespace ldapapi-ng
```

Then the service name defaults to `ldapapi-ldapapi-ng` (release + chart name)
or `ldapapi-ng` depending on your `nameOverride` / `fullnameOverride`. Check
the actual name:

```sh
kubectl get svc -n ldapapi-ng
```

And set the upstream accordingly, e.g.:

```
http://ldapapi-ldapapi-ng.ldapapi-ng.svc.cluster.local:8080
```

## Install

```sh
# 1. Add the upstream chart repo (maintained by Equinix Metal)
helm repo add equinixmetal https://helm.equinixmetal.com
helm repo update

# 2. Create the namespace and the Secret with your real values
kubectl create namespace krakend
kubectl create secret generic krakend-secrets -n krakend \
  --from-literal=LDAPAPI_UPSTREAM=http://ldapapi-ng.ldapapi-ng.svc.cluster.local:8080 \
  --from-literal=KEYCLOAK_JWKS_URL=https://keycloak.example.org/realms/myrealm/protocol/openid-connect/certs \
  --from-literal=KEYCLOAK_ISSUER=https://keycloak.example.org/realms/myrealm \
  --from-literal=KEYCLOAK_AUDIENCE=ldapapi-ng \
  --from-literal='KRAKEND_API_KEYS_JSON=[{"key":"changeme","roles":["ldapapi-user"]}]'

# 3. Install the chart with our values override
helm install krakend equinixmetal/krakend \
  --namespace krakend \
  -f k8s/values.yaml
```

No manual `kubectl apply` of a ConfigMap is needed — the chart creates the
ConfigMap from the `krakend.config` value in `values.yaml` and mounts it
automatically.

## Label alignment with the ldapapi-ng NetworkPolicy

`helm/templates/networkpolicy.yaml` in the ldapapi-ng chart pins ingress to
pods matching:

- `namespaceSelector: kubernetes.io/metadata.name=krakend`
- `podSelector: app.kubernetes.io/name=krakend`

The defaults in `k8s/values.yaml` keep those labels intact. **Do not** rename
the release or override `nameOverride` without also updating the
`networkPolicy.gatewayNamespaceSelector` / `gatewayPodSelector` values on the
ldapapi-ng side, or traffic from the gateway will be dropped.

## Validating the template locally

The standalone `krakend.tmpl` file in this directory is identical to the
config inlined in `values.yaml`. To sanity-check it without starting KrakenD:

```sh
sed -e 's/{{ env "LDAPAPI_UPSTREAM" }}/http:\/\/localhost:8080/g' \
    -e 's/{{ env "KEYCLOAK_JWKS_URL" }}/https:\/\/example.invalid\/jwks/g' \
    -e 's/{{ env "KEYCLOAK_ISSUER" }}/https:\/\/example.invalid/g' \
    -e 's/{{ env "KEYCLOAK_AUDIENCE" }}/ldapapi-ng/g' \
    -e 's|{{ env "KRAKEND_API_KEYS_JSON" }}|[{"key":"x","roles":["ldapapi-user"]}]|g' \
    krakend.tmpl | jq .
```

Or run KrakenD directly against it:

```sh
LDAPAPI_UPSTREAM=http://localhost:8080 \
KEYCLOAK_JWKS_URL=https://example.invalid/jwks \
KEYCLOAK_ISSUER=https://example.invalid \
KEYCLOAK_AUDIENCE=ldapapi-ng \
KRAKEND_API_KEYS_JSON='[{"key":"x","roles":["ldapapi-user"]}]' \
FC_ENABLE=1 FC_OUT=/tmp/krakend.json FC_TEMPLATES=$PWD \
docker run --rm --network host \
  -e FC_ENABLE -e FC_OUT -e FC_TEMPLATES \
  -e LDAPAPI_UPSTREAM -e KEYCLOAK_JWKS_URL -e KEYCLOAK_ISSUER \
  -e KEYCLOAK_AUDIENCE -e KRAKEND_API_KEYS_JSON \
  -v "$PWD":/etc/krakend/templates:ro \
  devopsfaith/krakend:2.10 check --config /etc/krakend/templates/krakend.tmpl
```
