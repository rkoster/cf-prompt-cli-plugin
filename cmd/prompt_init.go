package cmd

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/cli/plugin"
	"github.com/ruben/cf-prompt-cli-plugin/pkg/prompter"
)

func PromptInitCommand(cliConnection plugin.CliConnection, args []string) {
	if len(args) == 0 {
		fmt.Println("Error: Missing required argument APP_NAME")
		fmt.Println("Usage: cf prompt-init <APP_NAME>")
		os.Exit(1)
	}

	appName := args[0]
	fmt.Printf("Initializing prompter for app: %s\n", appName)

	apiEndpoint, err := cliConnection.ApiEndpoint()
	if err != nil {
		fmt.Printf("Error getting API endpoint: %v\n", err)
		os.Exit(1)
	}

	token, err := cliConnection.AccessToken()
	if err != nil {
		fmt.Printf("Error getting access token: %v\n", err)
		os.Exit(1)
	}

	deployer := prompter.NewPrompterInitDeployer(cliConnection, appName)

	if err := deployer.DeployPrompterApp(apiEndpoint, token); err != nil {
		fmt.Printf("Error deploying prompter app: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Prompter app '%s-prompter' deployed successfully\n", appName)
}
