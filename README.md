# cf-prompt-cli-plugin

A Cloud Foundry CLI plugin that uses AI (via OpenCode) to automatically modify your application code based on natural language prompts. The plugin downloads your app's source code, executes the prompt using OpenCode, and creates a new package revision ready to deploy.

## How It Works

1. **Download**: Fetches your app's latest package from Cloud Foundry
2. **Execute**: Runs OpenCode with your natural language prompt to modify the code
3. **Upload**: Creates a new package revision with the AI-generated changes
4. **Deploy**: Use `cf prompt-push` to stage and deploy the changes

## Features

- **AI-Powered Code Changes**: Modify your app using natural language prompts via OpenCode
- **Package-Based Workflow**: Creates new package revisions without disrupting your running app
- **Prompt History**: Track all prompts used to create each package revision
- **Selective Deployment**: Review and deploy only the changes you want

## Installation

### Prerequisites

- Go 1.19+ 
- Cloud Foundry CLI v6.7.0+
- Devbox (for development)
- A Korifi-based Cloud Foundry deployment

### Build and Install

```bash
# Clone the repository
git clone https://github.com/rkoster/cf-prompt-cli-plugin
cd cf-prompt-cli-plugin

# Build and install the plugin
make install
```

This will build the plugin, including the embedded prompter binary, and install it to your CF CLI.

## Usage

### Initial Setup

Before using prompts, initialize the prompter app for your application (one-time setup):

```bash
cf prompt-init my-app
```

This creates a `my-app-prompter` application that will execute OpenCode runs.

### Execute a Prompt

Run a natural language prompt to modify your app:

```bash
cf prompt my-app -p "change hello world to foo bar"
cf prompt my-app -p "add error handling to the API endpoints"
cf prompt my-app -p "optimize database queries"
```

This will:
1. Download your app's current source code
2. Execute OpenCode with your prompt to modify the code
3. Create a new package revision with the changes
4. The prompter app will automatically stop when complete

### List Available Packages

View all package revisions with their prompts and status:

```bash
cf prompts my-app
```

Output shows:
- **hash**: Short package identifier (asterisk indicates currently deployed)
- **state**: Package state (ready, processing, etc.)
- **droplet**: Whether a droplet exists (staged/unstaged)
- **created**: Creation timestamp
- **type**: Package type
- **original prompt**: The prompt used to create this revision

### Deploy a Package

Stage and deploy a specific package revision:

```bash
cf prompt-push my-app <PACKAGE_HASH>
```

This will:
1. Find the package by its short hash
2. Trigger staging if no droplet exists (shows build logs)
3. Set the droplet as current for your app
4. Your app will use the new code on next restart

## Commands

| Command | Description | Usage |
|---------|-------------|-------|
| `cf prompt-init` | Initialize prompter app for an application (one-time setup) | `cf prompt-init <APP_NAME>` |
| `cf prompt` | Execute a natural language prompt to modify app code | `cf prompt <APP_NAME> -p 'prompt text'` |
| `cf prompts` | List all package revisions with their prompts and status | `cf prompts <APP_NAME>` |
| `cf prompt-push` | Deploy a specific package revision | `cf prompt-push <APP_NAME> <PACKAGE_HASH>` |

## Workflow Example

```bash
# 1. Push your initial app
cf push my-app

# 2. Initialize the prompter (one-time)
cf prompt-init my-app

# 3. Create a code revision using a prompt
cf prompt my-app -p "add a /health endpoint that returns 200 OK"

# 4. View all package revisions and their prompts
cf prompts my-app

# 5. Deploy a specific revision (use the hash from step 4)
cf prompt-push my-app a1b2c3d

# 6. Restart your app to use the new code
cf restart my-app
```

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

#### Colima Configuration

When using Colima instead of docker on a linux host, additional network configuration is required:

```bash
# Add network route for Kubernetes cluster access
sudo route add -net 192.168.5.0/24 -interface bridge100
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

#### Testing Authentication

```bash
# Test login with admin credentials
echo -e "admin\nadmin" | CF_TRACE=true cf login -a https://localhost:443

# Verify authentication
cf auth
```

#### Architecture

The UAA integration includes:
- **UAA Server**: Deployed in `uaa-system` namespace with template-based configuration
- **Gateway API**: Using Envoy Gateway for traffic routing to both Korifi and UAA
- **SAML Support**: Pre-configured with embedded SAML certificates for immediate use
- **Local Registry**: Docker registry for kpack builds at `localregistry-docker-registry.default.svc.cluster.local:30050`
- **Korifi Integration**: Experimental UAA mode enabled with proper authentication flow
- **Template-Based Deployment**: All components deployed via Kubernetes manifests in `templates/` directory

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `make test`
5. Submit a pull request

## License

[Add your license here]
