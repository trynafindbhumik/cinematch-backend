.PHONY: dev build run clean test lint tidy install

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
BINARY_NAME=api
BINARY_DIR=tmp
MAIN_PATH=./cmd/api

# Development
dev:
	air

# Build
build:
	$(GOBUILD) -o $(BINARY_DIR)/$(BINARY_NAME) $(MAIN_PATH)

# Run without building
run:
	$(GORUN) $(MAIN_PATH)

# Clean
clean:
	rm -rf $(BINARY_DIR)
	rm -f build-errors.log

# Test
test:
	$(GOTEST) -v ./...

# Tidy modules
tidy:
	$(GOMOD) tidy

# Install dependencies
install:
	$(GOMOD) download

# Lint (requires golangci-lint)
lint:
	golangci-lint run

# Check if air is installed
check-air:
	@which air > /dev/null || echo "air not found. Run: go install github.com/air-verse/air@latest"

# Install dev dependencies
install-dev: check-air
	go install github.com/air-verse/air@latest