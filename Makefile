.PHONY: build test fmt lint vet check tidy release-check release-snapshot clean

build:
	go build -o ws-dev ./cmd/ws-dev

test:
	go test ./...

fmt:
	golangci-lint fmt ./...

lint:
	golangci-lint run ./...

vet:
	go vet ./...

check: fmt vet lint test

tidy:
	go mod tidy

release-check:
	goreleaser check

release-snapshot:
	goreleaser release --snapshot --clean

clean:
	rm -rf dist ws-dev
