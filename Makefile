TARGETS := $(shell ls scripts)

.dapper:
	@echo Downloading dapper
	@curl -sL https://releases.rancher.com/dapper/latest/dapper-$$(uname -s)-$$(uname -m) > .dapper.tmp
	@@chmod +x .dapper.tmp
	@./.dapper.tmp -v
	@mv .dapper.tmp .dapper

$(TARGETS): .dapper
	./.dapper $@

.DEFAULT_GOAL := default

.PHONY: $(TARGETS)

##@ Local Run

.PHONY: run-controller run-agent
run-controller: ## Run the controller from your host.
	go run ./cmd/controller $(ARGS)

run-agent: ## Run the agent from your host.
	go run ./cmd/agent $(ARGS)
