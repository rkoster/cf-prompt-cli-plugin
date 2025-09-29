.PHONY: deploy-korifi deploy-korifi-debug clean-korifi help

# Default cluster name
CLUSTER_NAME ?= korifi-dev

# Help target
help:
	@echo "Available targets:"
	@echo "  deploy-korifi      - Deploy stable Korifi v0.16.0 on kind cluster"
	@echo "  deploy-korifi-debug - Deploy Korifi with debugging enabled"
	@echo "  clean-korifi       - Delete the kind cluster"
	@echo "  help               - Show this help message"
	@echo ""
	@echo "Variables:"
	@echo "  CLUSTER_NAME       - Name of the kind cluster (default: korifi-dev)"

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