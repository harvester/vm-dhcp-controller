# syntax=docker/dockerfile:1.10

# ---- builder ----
FROM registry.suse.com/bci/golang:1.26 AS builder
ARG MK_HOST_ARCH
ENV ARCH=$MK_HOST_ARCH
ENV GOTOOLCHAIN=auto

RUN zypper -n install tar gzip bash git docker less file curl wget

COPY --from=golangci/golangci-lint:v2.12.2-alpine@sha256:91b27804074a0bacea298707f016911e60cf0cdbc6c7bf5ccacb5f0606d18d60 /usr/bin/golangci-lint /usr/local/bin/golangci-lint

RUN go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.18.0

ENV HOME=/go/src/github.com/harvester/vm-dhcp-controller

# ---- base ----
FROM builder AS base
WORKDIR /go/src/github.com/harvester/vm-dhcp-controller
COPY . .

# ---- build ----
FROM base AS build
ARG MK_REPO_ID
RUN --mount=type=cache,target=/go/pkg/mod,id=vm-dhcp-controller-go-mod-${MK_REPO_ID} \
    --mount=type=cache,target=/go/src/github.com/harvester/vm-dhcp-controller/.cache/go-build,id=vm-dhcp-controller-go-build-${MK_REPO_ID} \
    ./scripts/build

FROM scratch AS build-output
COPY --from=build /go/src/github.com/harvester/vm-dhcp-controller/bin/ /bin/

# ---- validate ----
FROM base AS validate
ARG MK_REPO_ID
RUN --mount=type=cache,target=/go/pkg/mod,id=vm-dhcp-controller-go-mod-${MK_REPO_ID} \
    --mount=type=cache,target=/go/src/github.com/harvester/vm-dhcp-controller/.cache/go-build,id=vm-dhcp-controller-go-build-${MK_REPO_ID} \
    ./scripts/validate

# ---- test ----
FROM base AS test
ARG MK_REPO_ID
RUN --mount=type=cache,target=/go/pkg/mod,id=vm-dhcp-controller-go-mod-${MK_REPO_ID} \
    --mount=type=cache,target=/go/src/github.com/harvester/vm-dhcp-controller/.cache/go-build,id=vm-dhcp-controller-go-build-${MK_REPO_ID} \
    ./scripts/test

# ---- generate-manifest ----
FROM base AS generate-manifest
RUN ./scripts/generate-manifest

FROM scratch AS generate-manifest-output
COPY --from=generate-manifest /go/src/github.com/harvester/vm-dhcp-controller/chart/crds/ /crds/
