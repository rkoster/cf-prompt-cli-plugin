package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/cli/plugin"
	"github.com/ruben/cf-prompt-cli-plugin/pkg/cfclient"
	"github.com/ruben/cf-prompt-cli-plugin/pkg/opencode"
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

	pkg, err := client.GetLatestPackage(appGUID)
	if err != nil {
		fmt.Printf("Error getting latest package: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found package: %s\n", pkg.GUID)

	workDir, err := os.MkdirTemp("", "cf-prompt-package-*")
	if err != nil {
		fmt.Printf("Error creating temp directory: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(workDir)

	fmt.Printf("Downloading and unpacking package to: %s\n", workDir)
	if err := client.DownloadPackage(pkg, workDir); err != nil {
		fmt.Printf("Error downloading package: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Package downloaded and unpacked successfully")

	packageDir := filepath.Join(workDir, "app")
	if _, err := os.Stat(packageDir); os.IsNotExist(err) {
		packageDir = workDir
	}

	fmt.Println("Executing opencode run...")
	if err := opencode.Run(packageDir, prompt, os.Stdout); err != nil {
		fmt.Printf("Error running opencode: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nCreating new package revision...")
	newPkg, err := client.CreatePackage(appGUID, packageDir)
	if err != nil {
		fmt.Printf("Error creating package: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("New package created: %s\n", newPkg.GUID)
	fmt.Println("Prompt execution on package completed successfully")
}
