# Micron Makefile
# Build automation for cross-platform binaries

BINARY_NAME=micron
VERSION?=0.1.0
BUILD_DIR=./bin
CMD_DIR=.

# Go build flags
LDFLAGS=-ldflags "-X main.version=${VERSION}"

# Default target
.PHONY: all
all: build

# Build for current platform
.PHONY: build
build:
	go build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME} ${CMD_DIR}

# Run tests
.PHONY: test
test:
	go test -v ./...

# Run benchmarks
.PHONY: bench
bench:
	go test -bench=. ./cmd/...

# Build for all platforms
.PHONY: build-all
build-all: build-windows build-linux build-macos

.PHONY: build-windows
build-windows:
	GOOS=windows GOARCH=amd64 go build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME}-windows.exe ${CMD_DIR}

.PHONY: build-linux
build-linux:
	GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME}-linux ${CMD_DIR}

.PHONY: build-macos
build-macos:
	GOOS=darwin GOARCH=amd64 go build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME}-macos ${CMD_DIR}

# Clean build artifacts
.PHONY: clean
clean:
	rm -rf ${BUILD_DIR}/*

# Install dependencies
.PHONY: deps
deps:
	go mod download
	go mod tidy

# Run the CLI
.PHONY: run
run:
	go run ${CMD_DIR}

# Format code
.PHONY: fmt
fmt:
	go fmt ./...

# Run linter
.PHONY: lint
lint:
	golangci-lint run ./...

# Generate test coverage
.PHONY: coverage
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
