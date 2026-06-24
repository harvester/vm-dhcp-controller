ROOT := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

MK_HOST_ARCH ?= $(shell uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
export MK_HOST_ARCH

MK_REPO_ID := $(shell echo -n "$(ROOT)$$(cat /etc/machine-id 2>/dev/null)" | sha256sum | cut -c1-8)
export MK_REPO_ID

MK_DOCKER_PROGRESS ?= plain
export MK_DOCKER_PROGRESS

MK_DOCKER_PULL ?= --pull
export MK_DOCKER_PULL

DOCKER_BUILDKIT := 1
export DOCKER_BUILDKIT

ifdef CI
  BOLD  :=
  CYAN  :=
  RESET :=
else
  BOLD  := \033[1m
  CYAN  := \033[36m
  RESET := \033[0m
endif

BANNER = @printf "$(BOLD)$(CYAN)[target: $@]$(RESET)\n"

DOCKER_BUILD = docker build $(MK_DOCKER_PULL) \
    --progress=$(MK_DOCKER_PROGRESS) \
    --build-arg MK_REPO_ID \
    --build-arg MK_HOST_ARCH \
    -f $(ROOT)/Dockerfile $(ROOT)

.DEFAULT_GOAL := ci

.PHONY: build validate test generate-manifest package package-controller package-agent package-webhook ci gen-version-env clean clean-all run-controller run-agent run-webhook

# ---- gen-version-env ----
gen-version-env:
	@bash $(ROOT)/scripts/version > /dev/null

# ---- build ----
$(ROOT)/bin:
	@mkdir -p $@

build: gen-version-env | $(ROOT)/bin
	$(BANNER)
	$(DOCKER_BUILD) --target build-output \
	    --output type=local,dest=$(ROOT)

# ---- validate ----
validate: gen-version-env
	$(BANNER)
	$(DOCKER_BUILD) --target validate

# ---- test ----
test: gen-version-env
	$(BANNER)
	$(DOCKER_BUILD) --target test

# ---- generate-manifest ----
generate-manifest: gen-version-env
	$(BANNER)
	$(DOCKER_BUILD) --target generate-manifest-output \
	    --output type=local,dest=$(ROOT)/chart

# ---- package ----
package: build
	$(BANNER)
	ARCH=$(MK_HOST_ARCH) $(ROOT)/scripts/package-controller
	ARCH=$(MK_HOST_ARCH) $(ROOT)/scripts/package-agent
	ARCH=$(MK_HOST_ARCH) $(ROOT)/scripts/package-webhook

package-controller: build
	$(BANNER)
	ARCH=$(MK_HOST_ARCH) $(ROOT)/scripts/package-controller

package-agent: build
	$(BANNER)
	ARCH=$(MK_HOST_ARCH) $(ROOT)/scripts/package-agent

package-webhook: build
	$(BANNER)
	ARCH=$(MK_HOST_ARCH) $(ROOT)/scripts/package-webhook

# ---- ci ----
ci: validate build test

# ---- clean ----
clean:
	$(BANNER)
	@rm -rf $(ROOT)/bin
	@rm -f $(ROOT)/scripts/.version_env

clean-all: clean
	$(BANNER)

##@ Local Run

run-controller: ## Run the controller from your host.
	go run ./cmd/controller $(ARGS)

run-agent: ## Run the agent from your host.
	go run ./cmd/agent $(ARGS)

run-webhook: ## Run the webhook from your host.
	go run ./cmd/webhook $(ARGS)
