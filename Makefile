# regcorpus make targets (R5: corpus gate; R11: make setup sets core.hooksPath).
# GNU Make 3.81 compatible. CORPUS_REF and LOOKUP_ONLY arrive in plan step 25.
# Run from the repo root, or with `make -C <root>`: recipes use relative paths.

# This Makefile, captured before anything else is read (MAKEFILES come first).
self := $(lastword $(MAKEFILE_LIST))

.DEFAULT_GOAL := help
.PHONY: help setup build test check labels

help: ## List targets
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  %-6s %s\n", $$1, $$2}' '$(self)'

setup: ## Point git at .githooks/ and make the hooks executable (a person, once per clone)
	git config core.hooksPath .githooks
	chmod +x .githooks/pre-commit .claude/hooks/*.sh

build: ## Build ./regcorpus
	go build -o regcorpus ./cmd/regcorpus

test: ## Run the Go tests, the R11 guard, pre-commit hook, CI gate and label sync tests (CI runs this)
	go test ./...
	bash .claude/hooks/r11-guard_test.sh
	bash .githooks/pre-commit_test.sh
	bash .github/c6_test.sh
	bash .github/labels_test.sh

# make exits 2 whenever a recipe fails, so `make check` exits 2 on violations too.
# For the check's own codes (0 ok, 1 violations, 2 unreadable) run ./regcorpus directly.
check: build ## Check the working tree with this branch's checker (C6; exits 2 on failure); hook and CI use main's
	./regcorpus check --corpus

labels: ## Create or update the GitHub labels in .github/labels.json (needs gh signed in, and jq)
	bash .github/labels.sh
