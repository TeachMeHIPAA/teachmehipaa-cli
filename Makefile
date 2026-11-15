STATICCHECK_VERSION := 2026.2.2

.PHONY: build test vet lint fmt check

build:
	go build -o bin/tmh ./cmd/tmh

test:
	go test -race ./...

vet:
	go vet ./...

lint: vet
	@test -z "$$(gofmt -l .)" || { gofmt -l .; echo "run make fmt"; exit 1; }
	go run honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION) ./...

fmt:
	gofmt -w .

check: lint test
