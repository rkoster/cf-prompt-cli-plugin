.PHONY: deploy-korifi deploy-korifi-debug deploy-uaa clean-korifi build install uninstall test integration-test help

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
	@echo "  deploy-korifi      - Deploy stable Korifi v0.16.0 with UAA on kind cluster"
	@echo "  deploy-korifi-debug - Deploy Korifi with debugging enabled"
	@echo "  deploy-uaa         - Deploy UAA components only"
	@echo "  clean-korifi       - Delete the kind cluster"
	@echo "  help               - Show this help message"
	@echo ""
	@echo "Variables:"
	@echo "  CLUSTER_NAME       - Name of the kind cluster (default: korifi-dev)"
	@echo ""
	@echo "UAA Support:"
	@echo "  The deploy-korifi target now includes UAA deployment with:"
	@echo "  - Fast startup (30 seconds) with working SAML configuration"
	@echo "  - Admin user: admin/admin"
	@echo "  - UAA URL: https://uaa-127-0-0-1.nip.io/uaa"
	@echo "  - Integrated with Korifi authentication"
	@echo "  - SSL/HTTPS enabled with self-signed certificates"
	@echo ""
	@echo "Test login:"
	@echo "  echo -e \"admin\\nadmin\" | CF_TRACE=true cf login -a https://localhost:443"

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

# Deploy Korifi using stable release with UAA
deploy-korifi:
	@echo "Deploying stable Korifi v0.16.0 with UAA on kind cluster '$(CLUSTER_NAME)'..."
	@echo "This may take several minutes..."
	devbox run -- bash scripts/deploy-korifi-stable.sh $(CLUSTER_NAME)

# Deploy Korifi with debugging
deploy-korifi-debug:
	@echo "Deploying stable Korifi v0.16.0 with UAA on kind cluster '$(CLUSTER_NAME)' with debugging enabled..."
	@echo "This may take several minutes..."
	devbox run -- bash scripts/deploy-korifi-stable.sh $(CLUSTER_NAME) --debug

# Clean up the kind cluster
clean-korifi:
	@echo "Deleting kind cluster '$(CLUSTER_NAME)'..."
	devbox run -- kind delete cluster --name $(CLUSTER_NAME)
	@echo "Stopping and removing local Docker registry..."
	devbox run -- docker stop registry 2>/dev/null || true
	devbox run -- docker rm registry 2>/dev/null || true
	@echo "Stopping and removing UAA Docker container..."
	devbox run -- docker stop uaa 2>/dev/null || true
	devbox run -- docker rm uaa 2>/dev/null || true
	@echo "Kind cluster '$(CLUSTER_NAME)', local registry, and UAA container cleaned up."