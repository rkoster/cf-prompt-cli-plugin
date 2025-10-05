# cf-prompt-cli-plugin

A Cloud Foundry CLI plugin that lets developers execute development tasks directly from natural language prompts. 
Run prompts as CF tasks seamlessly—keeping the simplicity and spirit of Cloud Foundry's "I do not care how" philosophy.

## Features

- **Natural Language Tasks**: Execute development tasks using natural language prompts
- **Seamless CF Integration**: Built as a native CF CLI plugin with familiar commands
- **Simple Workflow**: Maintains Cloud Foundry's philosophy of simplicity

## Installation

### Prerequisites

- Go 1.19+ 
- Cloud Foundry CLI v6.7.0+
- Devbox (for development)

### Build and Install

```bash
# Clone the repository
git clone https://github.com/ruben/cf-prompt-cli-plugin
cd cf-prompt-cli-plugin

# Build the plugin
make build

# Install to CF CLI
make install
```



## Usage

### Execute a Prompt as Task

Run a natural language prompt as a CF task:

```bash
cf prompt "optimize database queries in the user service"
cf prompt "run performance tests on the API endpoints"
cf prompt "check logs for errors in the last hour"
```

## Commands

| Command | Description | Usage |
|---------|-------------|-------|
| `cf prompt` | Execute a natural language prompt as a CF task | `cf prompt [prompt text]` |

## Development

### Setup

```bash
# Initialize devbox environment
devbox shell

# Build the plugin
make build

# Run tests
make test
```

### Project Structure

```
├── cmd/                    # Command implementations
├── integration/            # Integration tests
├── templates/              # Kubernetes deployment templates
│   ├── korifi_config.yaml  # UAA-enabled Korifi configuration
│   ├── uaa-config-updated.yaml # UAA server configuration with embedded SAML
│   ├── uaa-deployment-fixed.yaml # UAA deployment manifest
│   ├── uaa-httproute.yaml  # UAA HTTP routing configuration
│   └── localregistry-docker-registry.yaml # Local Docker registry setup
├── scripts/                # Deployment and build scripts
│   └── deploy-korifi-stable.sh # Complete Korifi+UAA deployment script
├── main.go                 # Plugin entry point
└── Makefile               # Build and deployment targets
```

### Development Environment

This project includes a complete Korifi development environment with UAA support:

```bash
# Deploy Korifi with UAA on kind cluster
make deploy-korifi

# Deploy with debugging enabled
make deploy-korifi-debug

# Clean up
make clean-korifi
```

#### Key Deployment Files

- **`scripts/deploy-korifi-stable.sh`**: Main deployment script that orchestrates the entire setup
- **`templates/uaa-config-updated.yaml`**: UAA configuration with embedded SAML certificates (valid until 2035)
- **`templates/uaa-deployment-fixed.yaml`**: UAA server deployment with optimized startup
- **`templates/korifi_config.yaml`**: Korifi configuration with UAA integration enabled
- **`templates/localregistry-docker-registry.yaml`**: Local Docker registry for kpack builds

#### Deployment Process

1. **Cluster Setup**: Creates kind cluster and local Docker registry
2. **Dependencies**: Installs cert-manager, Gateway API, Envoy Gateway, and kpack
3. **UAA Deployment**: Deploys UAA server with pre-configured SAML (30 second startup)
4. **Korifi Deployment**: Installs Korifi v0.16.0 with UAA integration enabled
5. **Networking**: Configures Gateway API routes for both services

#### UAA Configuration

The Korifi deployment includes UAA (User Account and Authentication) server for proper Cloud Foundry authentication:

- **UAA Endpoint**: http://uaa-127-0-0-1.nip.io/uaa
- **API Endpoint**: https://localhost:443 (with Gateway API/Envoy Gateway)
- **Default Admin User**: admin/admin
- **SAML Configuration**: Embedded certificates (valid until 2035)
- **Fast Startup**: 30 seconds deployment time
- **SSL/TLS**: UAA handles SSL termination directly using Cloud Native Buildpack configuration

##### SSL Configuration

UAA is configured to handle SSL/TLS termination directly without a proxy:

- **Certificate Generation**: PEM certificates auto-generated with SAN (Subject Alternative Names)
- **Keystore Conversion**: Entrypoint wrapper script converts PEM to PKCS12 keystore at runtime
- **CNB SSL Support**: Uses standard Cloud Native Buildpack environment variables:
  - `SERVER_PORT=8443`
  - `SERVER_SSL_ENABLED=true`
  - `SERVER_SSL_KEY_STORE=/workspace/keystore.p12`
  - `SERVER_SSL_KEY_STORE_TYPE=PKCS12`
  - `SERVER_SSL_KEY_STORE_PASSWORD=changeit`

#### Testing Authentication

```bash
# Test login with admin credentials
echo -e "admin\nadmin" | CF_TRACE=true cf login -a https://localhost:443

# Verify authentication
cf auth
```

#### Architecture

The UAA integration includes:
- **UAA Server**: Deployed in Docker with SSL/TLS termination using Cloud Native Buildpack configuration
- **Gateway API**: Using Envoy Gateway for traffic routing to both Korifi and UAA
- **SAML Support**: Pre-configured with embedded SAML certificates for immediate use
- **Local Registry**: Docker registry for kpack builds at `localregistry-docker-registry.default.svc.cluster.local:30050`
- **Korifi Integration**: Experimental UAA mode enabled with proper authentication flow
- **Template-Based Deployment**: All components deployed via Kubernetes manifests in `templates/` directory
- **SSL Configuration**: Just-in-time PKCS12 keystore generation from PEM certificates via entrypoint wrapper

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `make test`
5. Submit a pull request

## License

[Add your license here]