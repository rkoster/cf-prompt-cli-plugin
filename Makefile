.PHONY: deploy-korifi deploy-korifi-debug clean-korifi build install uninstall test integration-test help

# Default cluster name
CLUSTER_NAME ?= korifi-dev

# Plugin settings
PLUGIN_NAME = cf-prompt-plugin
PLUGIN_BINARY = $(PLUGIN_NAME)

# Help target
help:
	@echo "Available targets:"
	@echo "  build              - Build the CF prompt plugin"
	@echo "  install            - Install the plugin to CF CLI"
	@echo "  uninstall          - Uninstall the plugin from CF CLI"
	@echo "  test               - Run unit tests"
	@echo "  integration-test   - Run integration tests (requires deployed Korifi)"
	@echo "  deploy-korifi      - Deploy stable Korifi v0.16.0 on kind cluster"
	@echo "  deploy-korifi-debug - Deploy Korifi with debugging enabled"
	@echo "  clean-korifi       - Delete the kind cluster"
	@echo "  help               - Show this help message"
	@echo ""
	@echo "Variables:"
	@echo "  CLUSTER_NAME       - Name of the kind cluster (default: korifi-dev)"

# Build the plugin
build:
	@echo "Building CF prompt plugin..."
	devbox run -- go build -o $(PLUGIN_BINARY)
	@echo "Plugin built successfully: $(PLUGIN_BINARY)"

# Install the plugin to CF CLI
install: uninstall build
	@echo "Installing CF prompt plugin..."
	echo "y" | cf install-plugin $(PLUGIN_BINARY)
	@echo "Plugin installed successfully. Use 'cf prompt --help' to get started."

# Uninstall the plugin from CF CLI
uninstall:
	@echo "Uninstalling CF prompt plugin..."
	cf uninstall-plugin prompt || true
	@echo "Plugin uninstalled."

# Run tests
test:
	@echo "Running unit tests..."
	devbox run -- go test ./cmd/...

# Run integration tests
integration-test: install
	@echo "Running integration tests..."
	@echo "Note: This assumes 'make deploy-korifi' has been executed and cf CLI is available"
	devbox run -- go test -v ./integration/... -ginkgo.v -timeout 30m

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(PLUGIN_BINARY)

# Deploy Korifi using stable release
deploy-korifi:
	@echo "Deploying stable Korifi v0.16.0 on kind cluster '$(CLUSTER_NAME)'..."
	@echo "This may take several minutes..."
	devbox run -- bash scripts/deploy-korifi-stable.sh $(CLUSTER_NAME)

# Deploy Korifi with debugging
deploy-korifi-debug:
	@echo "Deploying stable Korifi v0.16.0 on kind cluster '$(CLUSTER_NAME)' with debugging enabled..."
	@echo "This may take several minutes..."
	devbox run -- bash scripts/deploy-korifi-stable.sh $(CLUSTER_NAME) --debug

# Clean up the kind cluster
clean-korifi:
	@echo "Deleting kind cluster '$(CLUSTER_NAME)'..."
	devbox run -- kind delete cluster --name $(CLUSTER_NAME)
	@echo "Kind cluster '$(CLUSTER_NAME)' deleted."