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
| `krakend.tmpl` | Flexible-Config template rendered into `krakend.json` at pod start. |
| `k8s/configmap.yaml` | ConfigMap that mounts `krakend.tmpl` under `/etc/krakend/templates`. |
| `k8s/secret.example.yaml` | Example Secret with every env var the template expects — copy, populate, and apply the copy (**never commit populated secrets**). |
| `k8s/values.yaml` | Values override for the upstream [KrakenD Helm chart](https://github.com/equinixmetal-helm/krakend) (maintained by Equinix Metal). |

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

## Environment variables consumed by the template

| Variable | Description |
| --- | --- |
| `LDAPAPI_UPSTREAM` | Full URL to the ldapapi-ng Service, e.g. `http://ldapapi-ng.ldapapi-ng.svc:8080`. |
| `KEYCLOAK_JWKS_URL` | Keycloak realm JWKS endpoint. |
| `KEYCLOAK_ISSUER` | Expected `iss` claim. |
| `KEYCLOAK_AUDIENCE` | Expected `aud` claim (client id). |
| `KRAKEND_API_KEYS_JSON` | Raw JSON array of `{ "key": ..., "roles": [...] }` entries injected into the config. |
| `OAUTH2_CLIENT` | Published to service-to-service callers in the format `clientId:clientSecret:tokenUrl`. KrakenD itself does not consume this — it documents how a caller can obtain a token. |

All of these live in the Secret so that rotating them is a `kubectl apply` +
rollout restart away.

## Install

```sh
# 1. Add the upstream chart repo (maintained by Equinix Metal)
helm repo add equinixmetal https://helm.equinixmetal.com
helm repo update

# 2. Create the namespace
kubectl create namespace krakend

# 3. Apply the template ConfigMap and your populated Secret
kubectl apply -n krakend -f k8s/configmap.yaml
cp k8s/secret.example.yaml /tmp/krakend-secret.yaml
# ... edit /tmp/krakend-secret.yaml and fill in real values ...
kubectl apply -n krakend -f /tmp/krakend-secret.yaml

# 4. Install the chart with our values override
helm install krakend equinixmetal/krakend \
  --namespace krakend \
  -f k8s/values.yaml
```

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

The template is plain JSON with `{{ env "VAR" }}` substitutions. To
sanity-check it without starting KrakenD:

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
