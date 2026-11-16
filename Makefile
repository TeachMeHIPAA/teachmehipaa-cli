STATICCHECK_VERSION := 2026.2.2

.PHONY: build test vet lint fmt check generate check-generated release-check snapshot

build:
	go build -o bin/tmh ./cmd/tmh

test:
	go test -race ./...

vet:
	go vet ./...

lint: vet check-generated
	@test -z "$$(gofmt -l .)" || { gofmt -l .; echo "run make fmt"; exit 1; }
	go run honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION) ./...

fmt:
	gofmt -w .

# Regenerates the resource commands from openapi/sdk-ir.json.
# go generate scans the package it runs in, so it trips over generated files the
# generator has just deleted; calling the generator directly avoids that.
generate:
	go run ./cmd/gen -ir openapi/sdk-ir.json -out internal/commands/generated

check-generated:
	go run ./cmd/gen -check

check: lint test

release-check:
	goreleaser check

# Builds everything a release ships into dist/ without publishing.
snapshot:
	goreleaser release --snapshot --clean
