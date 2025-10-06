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
		fmt.Printf("No droplet found for package. Triggering staging...\n")

		// Trigger build
		buildGUID, err := client.TriggerBuild(pkg.GUID)
		if err != nil {
			fmt.Printf("Error triggering build: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Build %s started for package %s\n", buildGUID, pkg.GUID)

		// Wait for completion while showing logs
		status, err := client.WaitForBuildCompletion(buildGUID, os.Stdout)
		if err != nil {
			fmt.Printf("Error during staging: %v\n", err)
			os.Exit(1)
		}

		if status != "STAGED" {
			fmt.Printf("Staging failed with status: %s\n", status)
			os.Exit(1)
		}

		// Get the droplet GUID from the completed build
		dropletGUID, err = client.GetBuildDropletGUID(buildGUID)
		if err != nil {
			fmt.Printf("Error getting droplet from build: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Staging completed. Created droplet %s\n", dropletGUID)
	} else {
		fmt.Printf("Found droplet %s\n", dropletGUID)
	}

	fmt.Println("Setting droplet as current...")
	if err := client.SetCurrentDroplet(appGUID, dropletGUID); err != nil {
		fmt.Printf("Error setting current droplet: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("OK")
	fmt.Println()
	fmt.Printf("Droplet %s has been set as current for app %s.\n", dropletGUID, appName)
	fmt.Printf("Run 'cf app %s' to see the updated app status.\n", appName)
}
