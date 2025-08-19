#!/bin/bash

# HAR TUI Build Script
# Builds the application for multiple platforms

set -e

VERSION=${1:-v1.0.0}
BINARY_NAME="har-tui"

echo "🐱 Building HAR TUI DELUXE ${VERSION}"

# Clean previous builds
echo "🧹 Cleaning previous builds..."
rm -rf build/
mkdir -p build/

# Build for different platforms
echo "🔨 Building for multiple platforms..."

# Linux AMD64
echo "  📦 Building for Linux (amd64)..."
GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=${VERSION}" -o build/${BINARY_NAME}-linux-amd64 cmd/har-tui/main.go

# Linux ARM64
echo "  📦 Building for Linux (arm64)..."
GOOS=linux GOARCH=arm64 go build -ldflags "-X main.version=${VERSION}" -o build/${BINARY_NAME}-linux-arm64 cmd/har-tui/main.go

# macOS AMD64 (Intel)
echo "  📦 Building for macOS (Intel)..."
GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.version=${VERSION}" -o build/${BINARY_NAME}-darwin-amd64 cmd/har-tui/main.go

# macOS ARM64 (Apple Silicon)
echo "  📦 Building for macOS (Apple Silicon)..."
GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.version=${VERSION}" -o build/${BINARY_NAME}-darwin-arm64 cmd/har-tui/main.go

# Windows AMD64
echo "  📦 Building for Windows (amd64)..."
GOOS=windows GOARCH=amd64 go build -ldflags "-X main.version=${VERSION}" -o build/${BINARY_NAME}-windows-amd64.exe cmd/har-tui/main.go

# Create checksums
echo "🔐 Generating checksums..."
cd build/
sha256sum * > checksums.sha256
cd ..

echo "✅ Build complete! Files created:"
ls -la build/

echo ""
echo "🚀 Ready for release!"
echo "   Upload the build/ directory contents to your GitHub release"