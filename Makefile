# regcorpus make targets (R5: corpus gate; R11: make setup sets core.hooksPath).
# GNU Make 3.81 compatible. CORPUS_REF and LOOKUP_ONLY arrive in plan step 25.

.DEFAULT_GOAL := help
.PHONY: help setup build test check

help: ## List targets
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "} {printf "  %-6s %s\n", $$1, $$2}'

setup: ## Point git at .githooks/ and make the hooks executable (run once per clone)
	git config core.hooksPath .githooks
	chmod +x .githooks/pre-commit .claude/hooks/*.sh

build: ## Build ./regcorpus
	go build -o regcorpus ./cmd/regcorpus

test: ## Run the Go tests, then the R11 guard tests
	go test ./...
	bash .claude/hooks/r11-guard_test.sh

check: ## Run the per-commit corpus check (C6)
	go run ./cmd/regcorpus check --corpus
