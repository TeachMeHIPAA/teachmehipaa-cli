STATICCHECK_VERSION := 2026.2.2

.PHONY: build test vet lint fmt check generate docs check-generated shellcheck release-check snapshot

build:
	go build -o bin/tmh ./cmd/tmh

test:
	go test -race ./...

vet:
	go vet ./...

lint: vet check-generated shellcheck
	@test -z "$$(gofmt -l .)" || { gofmt -l .; echo "run make fmt"; exit 1; }
	go run honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION) ./...

shellcheck:
	@if command -v shellcheck >/dev/null; then shellcheck examples/*.sh scripts/*.sh; \
	else echo "shellcheck not installed; skipping examples/*.sh and scripts/*.sh"; fi

fmt:
	gofmt -w .

# Regenerates the resource commands and docs/reference/resources.md from
# openapi/sdk-ir.json, then the command reference from the new commands.
# go generate scans the package it runs in, so it trips over generated files the
# generator has just deleted; calling the generator directly avoids that.
generate:
	go run ./cmd/gen -ir openapi/sdk-ir.json -out internal/commands/generated
	$(MAKE) docs

# Regenerates the Markdown command reference in docs/reference/commands from
# tmh's command tree.
docs:
	go run ./cmd/docgen

check-generated:
	go run ./cmd/gen -check
	go run ./cmd/docgen -check

check: lint test

release-check:
	goreleaser check

# Builds everything a release ships into dist/ without publishing.
snapshot:
	goreleaser release --snapshot --clean
