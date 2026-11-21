# regcorpus make targets (R5: corpus gate; R11: make setup sets core.hooksPath).
# GNU Make 3.81 compatible. CORPUS_REF and LOOKUP_ONLY arrive in plan step 25.
# Run from the repo root, or with `make -C <root>`: recipes use relative paths.

# This Makefile, captured before anything else is read (MAKEFILES come first).
self := $(lastword $(MAKEFILE_LIST))

.DEFAULT_GOAL := help
.PHONY: help setup build test check

help: ## List targets
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  %-6s %s\n", $$1, $$2}' '$(self)'

setup: ## Point git at .githooks/ and make the hooks executable (a person, once per clone)
	git config core.hooksPath .githooks
	chmod +x .githooks/pre-commit .claude/hooks/*.sh

build: ## Build ./regcorpus
	go build -o regcorpus ./cmd/regcorpus

test: ## Run the Go tests, the R11 guard tests and the pre-commit gate tests
	go test ./...
	bash .claude/hooks/r11-guard_test.sh
	bash .githooks/pre-commit_test.sh

check: build ## Check the working tree (C6); pre-commit checks the commit itself
	./regcorpus check --corpus
