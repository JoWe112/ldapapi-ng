# syntax=docker/dockerfile:1.7

# ---------- Builder stage ----------
# Pinned, digest-locked. Bump both the tag and the digest together.
#
# --platform=$BUILDPLATFORM makes the builder stage run natively on the host
# (e.g. arm64 on Apple Silicon) and Go itself cross-compiles to TARGETOS/
# TARGETARCH. This avoids QEMU user-mode emulation, which is ~10-30x slower.
FROM --platform=$BUILDPLATFORM golang:1.25.9-alpine3.22@sha256:2c16ac01b3d038ca2ed421d66cea489e3cb670c251b4f8bbcfad2ebfb75f884c AS builder

# BuildKit automatically populates these. Default values keep manual
# `docker build` invocations working without buildx.
ARG TARGETOS=linux
ARG TARGETARCH=amd64

# Build args used to inject version metadata via -ldflags.
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown

# Git is needed by `go build` when modules resolve via VCS; add ca-certificates
# so `go mod download` can speak TLS to proxy.golang.org.
RUN apk add --no-cache ca-certificates git

WORKDIR /build

# Download dependencies first so they are cached independently of the source.
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source.
COPY . .

ENV CGO_ENABLED=0

RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
        -trimpath \
        -ldflags "-s -w \
            -X github.com/JoWe112/ldapapi-ng/internal/version.Version=${VERSION} \
            -X github.com/JoWe112/ldapapi-ng/internal/version.Commit=${COMMIT} \
            -X github.com/JoWe112/ldapapi-ng/internal/version.Date=${DATE}" \
        -o /out/ldapapi-ng \
        ./cmd/ldapapi-ng

# ---------- Runtime stage ----------
FROM alpine:3.23@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659

# Only the bare minimum to run the binary: TLS roots + timezone data.
# `apk upgrade` pulls the latest security patches from the Alpine repos on
# every build, even though the base image itself is digest-pinned — so
# rebuilds automatically pick up CVE fixes without unpinning the FROM.
RUN apk upgrade --no-cache && \
    apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 1001 -g root app

WORKDIR /app

# Copy the static binary with group-0 ownership so OpenShift's random UID
# (which always runs with supplementary group 0) can read and execute it.
COPY --from=builder --chown=1001:0 /out/ldapapi-ng /app/ldapapi-ng

# Make /app group-writable so arbitrary UIDs in group 0 behave like the
# declared USER. This is the OpenShift restricted-v2 SCC pattern.
RUN chgrp -R 0 /app && chmod -R g=u /app

# Non-privileged port; declared as default UID but the image must also work
# with any arbitrary UID injected by OpenShift.
USER 1001
EXPOSE 8080

# Arbitrary UIDs have no home directory entry — point HOME at a writable tmpfs.
ENV HOME=/tmp

# OCI labels aid registry UIs and Trivy's SBOM output.
LABEL org.opencontainers.image.title="ldapapi-ng" \
      org.opencontainers.image.description="REST API for LDAPS authentication and user lookup" \
      org.opencontainers.image.source="https://github.com/JoWe112/ldapapi-ng" \
      org.opencontainers.image.licenses="MIT"

ENTRYPOINT ["/app/ldapapi-ng"]
