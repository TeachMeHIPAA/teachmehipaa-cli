STATICCHECK_VERSION := 2026.2.2

.PHONY: build test vet lint fmt check generate check-generated

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
generate:
	go generate ./internal/commands/generated

check-generated:
	go run ./cmd/gen -check

check: lint test
