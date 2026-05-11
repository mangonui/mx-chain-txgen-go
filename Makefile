.PHONY: build run clean test vet tidy

BINARY := txgen
CMD_DIR := ./cmd/txgen
BUILD_DIR := $(CMD_DIR)

build:
	cd $(CMD_DIR) && GOWORK=off go build -o $(BINARY) .

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
