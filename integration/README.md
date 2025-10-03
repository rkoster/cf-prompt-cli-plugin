# Integration Tests

This directory contains integration tests for the cf-prompt-cli-plugin using Ginkgo testing framework.

## Prerequisites

Before running the integration tests, ensure you have:

1. **Deployed Korifi**: Run `make deploy-korifi` to deploy Korifi on a kind cluster
2. **CF CLI**: Cloud Foundry CLI must be installed and configured
3. **Plugin Built and Installed**: Run `make install` to build and install the cf-prompt plugin

## Test Structure

- `integration_suite_test.go`: Ginkgo test suite setup
- `prompt_integration_test.go`: Integration tests for the cf prompt plugin
- `assets/hello-world-app/`: Simple Go application used for testing

## Running Integration Tests

```bash
# Run all integration tests
make integration-test

# Or run directly with go test
devbox run -- go test -v ./integration/... -ginkgo.v -timeout 30m
```

## Test Environment Variables

You can customize the test environment with the following variables:

- `CF_ORG`: Cloud Foundry organization (default: "cf-org")
- `CF_SPACE`: Cloud Foundry space (default: "cf-space")

Example:
```bash
CF_ORG=my-org CF_SPACE=my-space make integration-test
```

## Test Workflow

The integration test performs the following steps:

1. **Push Application**: Deploys the hello-world-app to Cloud Foundry
2. **Verify Initial State**: Confirms the app responds with "hello world"
3. **Execute Prompt Command**: Attempts to use `cf prompt` to change the text
4. **Validate Results**: Checks if the app is still accessible (the actual prompt functionality is not fully implemented yet)

## Known Limitations

The current implementation focuses on test infrastructure rather than full functionality:

- The cf prompt plugin is not expected to successfully modify the app yet
- Tests validate that the workflow can be executed, not that it produces the desired result
- The prompt command will be invoked but may not fully succeed in changing the app text

## Cleanup

The tests automatically clean up deployed apps after each test run using the `AfterEach` hook.
