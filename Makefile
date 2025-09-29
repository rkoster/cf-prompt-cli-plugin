.PHONY: deploy-korifi deploy-korifi-debug clean-korifi help

# Default cluster name
CLUSTER_NAME ?= korifi-dev

# Help target
help:
	@echo "Available targets:"
	@echo "  deploy-korifi      - Deploy Korifi on kind cluster using devbox dependencies"
	@echo "  deploy-korifi-debug - Deploy Korifi with debugging enabled"
	@echo "  clean-korifi       - Delete the kind cluster"
	@echo "  help               - Show this help message"
	@echo ""
	@echo "Variables:"
	@echo "  CLUSTER_NAME       - Name of the kind cluster (default: korifi-dev)"

# Clone Korifi repository if it doesn't exist
korifi:
	@echo "Cloning Korifi repository..."
	devbox run -- git clone https://github.com/cloudfoundry/korifi.git

# Deploy Korifi using kind
deploy-korifi: korifi
	@echo "Deploying Korifi on kind cluster '$(CLUSTER_NAME)'..."
	@echo "This may take several minutes..."
	devbox run -- bash korifi/scripts/deploy-on-kind.sh $(CLUSTER_NAME)
	@echo ""
	@echo "Korifi deployment completed!"
	@echo "You can now use 'devbox run -- cf login' to connect to the API at https://localhost"
	@echo "Use the 'cf-admin' user when prompted for login."

# Deploy Korifi with debugging
deploy-korifi-debug: korifi
	@echo "Deploying Korifi on kind cluster '$(CLUSTER_NAME)' with debugging enabled..."
	@echo "This may take several minutes..."
	devbox run -- bash korifi/scripts/deploy-on-kind.sh $(CLUSTER_NAME) --debug
	@echo ""
	@echo "Korifi deployment with debugging completed!"
	@echo "Debug ports are available on localhost:30051-30055"
	@echo "You can now use 'devbox run -- cf login' to connect to the API at https://localhost"

# Clean up the kind cluster
clean-korifi:
	@echo "Deleting kind cluster '$(CLUSTER_NAME)'..."
	devbox run -- kind delete cluster --name $(CLUSTER_NAME)
	@echo "Kind cluster '$(CLUSTER_NAME)' deleted."