#!/bin/bash

set -e

PLUGIN_NAME="cf-prompt-plugin"
PLATFORM=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH="amd64"

if [[ "$PLATFORM" == "darwin" ]]; then
    BINARY_NAME="${PLUGIN_NAME}-darwin-${ARCH}"
elif [[ "$PLATFORM" == "linux" ]]; then
    BINARY_NAME="${PLUGIN_NAME}-linux-${ARCH}"
else
    echo "Unsupported platform: $PLATFORM"
    exit 1
fi

echo "Installing CF Prompt Plugin..."

# Check if binary exists
if [[ ! -f "bin/$BINARY_NAME" ]]; then
    echo "Binary not found. Building first..."
    ./scripts/build.sh
fi

# Uninstall if already installed
cf uninstall-plugin prompt 2>/dev/null || true

# Install the plugin
cf install-plugin "bin/$BINARY_NAME" -f

echo "✅ CF Prompt Plugin installed successfully!"
echo ""
echo "Usage:"
echo "  cf prompt [prompt text]    - Execute prompt as CF task"