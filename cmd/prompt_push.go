package cmd

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/cli/plugin"
	"github.com/ruben/cf-prompt-cli-plugin/pkg/cfclient"
)

func PromptPushCommand(cliConnection plugin.CliConnection, args []string) {
	if len(args) < 2 {
		fmt.Println("Error: Invalid arguments")
		fmt.Println("Usage: cf prompt-push <APP_NAME> <PACKAGE_HASH>")
		os.Exit(1)
	}

	appName := args[0]
	packageHash := args[1]

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

	username, err := cliConnection.Username()
	if err != nil {
		fmt.Printf("Error getting username: %v\n", err)
		os.Exit(1)
	}

	client, err := cfclient.New(apiEndpoint, token)
	if err != nil {
		fmt.Printf("Error creating CF client: %v\n", err)
		os.Exit(1)
	}

	appGUID, err := client.GetAppGUID(appName, currentSpace.Guid)
	if err != nil {
		fmt.Printf("Error getting app GUID for '%s': %v\n", appName, err)
		os.Exit(1)
	}

	fmt.Printf("Updating app %s in org %s / space %s as %s...\n\n", appName, currentOrg.Name, currentSpace.Name, username)

	pkg, err := client.FindPackageByShortHash(appGUID, packageHash)
	if err != nil {
		fmt.Printf("Error finding package: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found package %s (hash: %s)\n", pkg.GUID, cfclient.ShortHash(pkg.GUID))

	dropletGUID, err := client.GetPackageDropletGUID(pkg.GUID)
	if err != nil {
		fmt.Printf("Error getting droplet for package: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found droplet %s\n", dropletGUID)

	fmt.Println("Setting droplet as current...")
	if err := client.SetCurrentDroplet(appGUID, dropletGUID); err != nil {
		fmt.Printf("Error setting current droplet: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("OK")
}
