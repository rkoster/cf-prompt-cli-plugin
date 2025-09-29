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

Or use the install script:

```bash
./scripts/install.sh
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
├── pkg/                    # Shared packages  
├── internal/               # Internal packages
├── scripts/                # Build and deployment scripts
├── main.go                 # Plugin entry point
└── Makefile               # Build targets
```

### Development Environment

This project includes a complete Korifi development environment:

```bash
# Deploy Korifi on kind cluster
make deploy-korifi

# Clean up
make clean-korifi
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `make test`
5. Submit a pull request

## License

[Add your license here]