# HAR TUI Makefile

# Variables
BINARY_NAME=har-tui
MAIN_FILE=cmd/har-tui/main.go
VERSION=v1.0.0

# Default target
.PHONY: all
all: build

# Build the application
.PHONY: build
build:
	go build -o $(BINARY_NAME) $(MAIN_FILE)

# Build with version info
.PHONY: build-release
build-release:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY_NAME) $(MAIN_FILE)

# Clean build artifacts
.PHONY: clean
clean:
	go clean
	rm -f $(BINARY_NAME)
	rm -f *.curl.sh *.body.txt *.tmp

# Install to system PATH
.PHONY: install
install: build
	sudo cp $(BINARY_NAME) /usr/local/bin/

# Uninstall from system PATH
.PHONY: uninstall
uninstall:
	sudo rm -f /usr/local/bin/$(BINARY_NAME)

# Run with test data
.PHONY: run
run: build
	./$(BINARY_NAME) test.har

# Run with enhanced test data
.PHONY: run-enhanced
run-enhanced: build
	./$(BINARY_NAME) enhanced-test.har

# Format Go code
.PHONY: fmt
fmt:
	go fmt ./...

# Run Go vet
.PHONY: vet
vet:
	go vet ./...

# Tidy go modules
.PHONY: tidy
tidy:
	go mod tidy

# Build for multiple platforms
.PHONY: build-all
build-all: clean build/
	GOOS=linux GOARCH=amd64 go build -o build/$(BINARY_NAME)-linux-amd64 $(MAIN_FILE)
	GOOS=darwin GOARCH=amd64 go build -o build/$(BINARY_NAME)-darwin-amd64 $(MAIN_FILE)
	GOOS=darwin GOARCH=arm64 go build -o build/$(BINARY_NAME)-darwin-arm64 $(MAIN_FILE)
	GOOS=windows GOARCH=amd64 go build -o build/$(BINARY_NAME)-windows-amd64.exe $(MAIN_FILE)

# Create build directory
build/:
	mkdir -p build

# Help target
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build        - Build the application"
	@echo "  build-all    - Build for multiple platforms"
	@echo "  clean        - Clean build artifacts"
	@echo "  install      - Install to /usr/local/bin"
	@echo "  uninstall    - Remove from /usr/local/bin"
	@echo "  run          - Build and run with test.har"
	@echo "  run-enhanced - Build and run with enhanced-test.har"
	@echo "  fmt          - Format Go code"
	@echo "  vet          - Run go vet"
	@echo "  tidy         - Tidy go modules"
	@echo "  help         - Show this help message"