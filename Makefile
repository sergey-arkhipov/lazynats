BINARY := lazynats
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(DATE)

.PHONY: build
build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

.PHONY: run
run:
	go run . --config ./config.example.yaml

.PHONY: test
test:
	go test ./... -race -cover

.PHONY: lint
lint:
	golangci-lint run

.PHONY: fmt
fmt:
	gofmt -l -w .
	goimports -l -w .

.PHONY: release-dry-run
release-dry-run:
	goreleaser release --snapshot --clean

.PHONY: clean
clean:
	rm -f $(BINARY)
	rm -rf dist
