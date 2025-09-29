#!/bin/bash

set -e

echo "Building CF Prompt Plugin..."

# Build for multiple platforms
GOOS=linux GOARCH=amd64 devbox run -- go build -o bin/cf-prompt-plugin-linux-amd64
GOOS=darwin GOARCH=amd64 devbox run -- go build -o bin/cf-prompt-plugin-darwin-amd64
GOOS=windows GOARCH=amd64 devbox run -- go build -o bin/cf-prompt-plugin-windows-amd64.exe

echo "✅ Build completed successfully!"
echo "Binaries created in bin/ directory:"
ls -la bin/