.PHONY: build run clean test vet tidy

BINARY := txgen
CMD_DIR := ./cmd/txgen
BUILD_DIR := $(CMD_DIR)

# Build-time identity. Fail open: if git/date are unavailable the
# defaults baked into version/version.go ("dev" / "unknown") survive.
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT    ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILDDATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo unknown)

VERSION_PKG := github.com/mangonui/mx-chain-txgen-go/version
LDFLAGS := -ldflags "-X $(VERSION_PKG).Version=$(VERSION) \
	-X $(VERSION_PKG).Commit=$(COMMIT) \
	-X $(VERSION_PKG).BuildDate=$(BUILDDATE)"

build:
	cd $(CMD_DIR) && GOWORK=off go build $(LDFLAGS) -o $(BINARY) .

run: build
	cd $(CMD_DIR) && GOWORK=off ./$(BINARY)

tidy:
	GOWORK=off go mod tidy

vet:
	GOWORK=off go vet ./...

test:
	GOWORK=off go test ./... -count=1 -timeout 60s

clean:
	rm -f $(CMD_DIR)/$(BINARY)
