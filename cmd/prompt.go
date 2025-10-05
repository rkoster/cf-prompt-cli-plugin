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
		fmt.Println("   or: cf prompt APP_NAME [prompt text]")
		os.Exit(1)
	}

	var app string
	var promptArgs []string

	// Parse arguments - support both --app flag and positional app name
	for i := 0; i < len(args); i++ {
		if args[i] == "--app" && i+1 < len(args) {
			app = args[i+1]
			i++
		} else {
			promptArgs = append(promptArgs, args[i])
		}
	}

	// If no --app flag was used, check if first argument is an app name
	if app == "" && len(promptArgs) > 0 {
		potentialAppName := promptArgs[0]

		// Check if this looks like an app name (exists in current space)
		currentSpace, err := cliConnection.GetCurrentSpace()
		if err == nil {
			apiEndpoint, err := cliConnection.ApiEndpoint()
			if err == nil {
				token, err := cliConnection.AccessToken()
				if err == nil {
					client, err := cfclient.New(apiEndpoint, token)
					if err == nil {
						_, err := client.GetAppGUID(potentialAppName, currentSpace.Guid)
						if err == nil {
							// Found the app, use it as app name and remove from prompt args
							app = potentialAppName
							promptArgs = promptArgs[1:]
						}
					}
				}
			}
		}
	}

	if len(promptArgs) == 0 {
		fmt.Println("Error: No prompt provided")
		fmt.Println("Usage: cf prompt [--app APP_NAME] [prompt text]")
		fmt.Println("   or: cf prompt APP_NAME [prompt text]")
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
	if registryUsername == "" {
		registryUsername = "user"
	}

	registryPassword := os.Getenv("REGISTRY_PASSWORD")
	if registryPassword == "" {
		registryPassword = "password"
	}

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
