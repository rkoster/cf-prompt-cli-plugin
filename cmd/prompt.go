package cmd

import (
	"fmt"
	"os"
	"strings"

	"code.cloudfoundry.org/cli/plugin"
	"github.com/ruben/cf-prompt-cli-plugin/pkg/cfclient"
	"github.com/ruben/cf-prompt-cli-plugin/pkg/prompter"
)

func PromptCommand(cliConnection plugin.CliConnection, args []string) {
	if len(args) == 0 {
		fmt.Println("Error: No prompt provided")
		fmt.Println("Usage: cf prompt [--app APP_NAME] [prompt text]")
		os.Exit(1)
	}

	var app string
	var promptArgs []string

	for i := 0; i < len(args); i++ {
		if args[i] == "--app" && i+1 < len(args) {
			app = args[i+1]
			i++
		} else {
			promptArgs = append(promptArgs, args[i])
		}
	}

	if len(promptArgs) == 0 {
		fmt.Println("Error: No prompt provided")
		fmt.Println("Usage: cf prompt [--app APP_NAME] [prompt text]")
		os.Exit(1)
	}

	prompt := strings.Join(promptArgs, " ")

	if app == "" {
		var err error
		app, err = getCurrentApp(cliConnection)
		if err != nil {
			fmt.Printf("Error getting current app: %v\n", err)
			fmt.Println("Please specify an app with --app APP_NAME or target an app first")
			os.Exit(1)
		}
	}

	fmt.Printf("Executing prompt on package for app: %s\n", app)
	fmt.Printf("Prompt: %s\n", prompt)

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

	currentSpace, err := cliConnection.GetCurrentSpace()
	if err != nil {
		fmt.Printf("Error getting current space: %v\n", err)
		os.Exit(1)
	}

	currentOrg, err := cliConnection.GetCurrentOrg()
	if err != nil {
		fmt.Printf("Error getting current org: %v\n", err)
		os.Exit(1)
	}

	client, err := cfclient.New(apiEndpoint, token)
	if err != nil {
		fmt.Printf("Error creating CF client: %v\n", err)
		os.Exit(1)
	}

	appGUID, err := client.GetAppGUID(app, currentSpace.Guid)
	if err != nil {
		fmt.Printf("Error getting app GUID: %v\n", err)
		os.Exit(1)
	}

	registryUsername := os.Getenv("REGISTRY_USERNAME")
	registryPassword := os.Getenv("REGISTRY_PASSWORD")

	deployer := prompter.NewAppDeployer(cliConnection)
	
	if err := deployer.Deploy(
		apiEndpoint,
		token,
		appGUID,
		currentSpace.Guid,
		currentOrg.Guid,
		registryUsername,
		registryPassword,
		prompt,
	); err != nil {
		fmt.Printf("Error deploying prompter app: %v\n", err)
		os.Exit(1)
	}

	defer func() {
		if err := deployer.Cleanup(); err != nil {
			fmt.Printf("Warning: Failed to cleanup prompter app: %v\n", err)
		}
	}()

	if err := deployer.MonitorLogs(os.Stdout); err != nil {
		fmt.Printf("Error monitoring logs: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Prompt execution completed successfully")
}
