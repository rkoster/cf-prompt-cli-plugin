package cmd

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/cli/plugin"
	"github.com/ruben/cf-prompt-cli-plugin/pkg/cfclient"
	"github.com/ruben/cf-prompt-cli-plugin/pkg/prompter"
)

// ParsePromptArgs parses command line arguments and returns app name, prompt text, and whether parsing failed
func ParsePromptArgs(args []string) (app string, prompt string, failed bool) {
	if len(args) == 0 {
		return "", "", true
	}

	var nonFlagArgs []string

	// Parse arguments - support -a for app name and -p for prompt
	for i := 0; i < len(args); i++ {
		if (args[i] == "--app" || args[i] == "-a") && i+1 < len(args) {
			app = args[i+1]
			i++
		} else if (args[i] == "--prompt" || args[i] == "-p") && i+1 < len(args) {
			prompt = args[i+1]
			i++
		} else {
			nonFlagArgs = append(nonFlagArgs, args[i])
		}
	}

	// If no -a flag was used, treat first non-flag argument as app name
	if app == "" && len(nonFlagArgs) > 0 {
		app = nonFlagArgs[0]
	}

	if app == "" || prompt == "" {
		return "", "", true
	}

	return app, prompt, false
}

func PromptCommand(cliConnection plugin.CliConnection, args []string) {
	app, prompt, failed := ParsePromptArgs(args)
	if failed {
		fmt.Println("Error: Invalid arguments")
		fmt.Println("Usage: cf prompt APP_NAME -p 'prompt text'")
		fmt.Println("   or: cf prompt -a APP_NAME -p 'prompt text'")
		os.Exit(1)
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

	prompterName := fmt.Sprintf("%s-prompter", app)
	prompterGUID, err := client.GetAppGUID(prompterName, currentSpace.Guid)
	if err != nil || prompterGUID == "" {
		fmt.Printf("Error: Prompter app '%s' does not exist\n", prompterName)
		fmt.Printf("Please run the following command first:\n")
		fmt.Printf("  cf prompt-init %s\n", app)
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

	deployer := prompter.NewAppDeployer(cliConnection, prompterName)

	if err := deployer.StartPrompter(
		apiEndpoint,
		token,
		appGUID,
		currentSpace.Guid,
		currentOrg.Guid,
		registryUsername,
		registryPassword,
		prompt,
	); err != nil {
		fmt.Printf("Error starting prompter app: %v\n", err)
		os.Exit(1)
	}

	defer func() {
		if err := deployer.StopPrompter(); err != nil {
			fmt.Printf("Warning: Failed to stop prompter app: %v\n", err)
		}
	}()

	if err := deployer.MonitorLogs(os.Stdout); err != nil {
		fmt.Printf("Error monitoring logs: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Prompt execution completed successfully")
}
