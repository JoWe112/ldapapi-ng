# Security Policy

## Reporting a vulnerability

If you believe you have found a security vulnerability in `ldapapi-ng`,
please **do not** open a public GitHub issue. Instead, report it privately
via GitHub's [private vulnerability reporting](https://github.com/JoWe112/ldapapi-ng/security/advisories/new)
feature.

Please include:

- A description of the issue and the impact you believe it could have
- Steps to reproduce, or a proof-of-concept
- The affected version (commit SHA or released tag)
- Any suggested mitigation, if you have one in mind

You can expect an initial acknowledgement within a few business days. Once
the report is validated, a fix will be developed on a private branch and
released together with a GitHub Security Advisory that credits the reporter
(unless you prefer to remain anonymous).

## Supported versions

Only the latest released version on the `main` branch is supported for
security updates. There are no long-lived maintenance branches at this time.

## Scope

In scope:

- The `ldapapi-ng` binary and its handling of HTTP, LDAP, TLS, and credentials
- The container image published from this repository
- The Helm chart under `helm/` and the KrakenD configuration under `deploy/krakend/`

Out of scope:

- Issues in upstream dependencies — please report those to the upstream project
- Misconfiguration of your own LDAP directory, Keycloak realm, or cluster
- Denial-of-service caused by hitting the API from an unauthenticated source
  with very high request rates; rate-limiting is the operator's responsibility
  and the project's threat model assumes the gateway mode is fronted by
  KrakenD with its own throttling
