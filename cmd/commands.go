package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"code.cloudfoundry.org/cli/plugin"
)

func PromptCommand(cliConnection plugin.CliConnection, args []string) {
	if len(args) == 0 {
		fmt.Println("Error: No prompt provided")
		fmt.Println("Usage: cf prompt [--app APP_NAME] [prompt text]")
		os.Exit(1)
	}

	// Parse arguments for --app flag
	var app string
	var promptArgs []string
	
	for i := 0; i < len(args); i++ {
		if args[i] == "--app" && i+1 < len(args) {
			app = args[i+1]
			i++ // Skip the next argument (app name)
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
	
	fmt.Printf("Executing prompt: %s\n", prompt)
	
	// Get current app if not specified
	if app == "" {
		var err error
		app, err = getCurrentApp(cliConnection)
		if err != nil {
			fmt.Printf("Error getting current app: %v\n", err)
			fmt.Println("Please specify an app with --app APP_NAME or target an app first")
			os.Exit(1)
		}
	}

	fmt.Printf("Using app: %s\n", app)

	// Execute prompt as CF task using CF API client
	err := executePromptAsTask(cliConnection, app, prompt)
	if err != nil {
		fmt.Printf("Error executing prompt: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Prompt execution started successfully")
}

func getCurrentApp(cliConnection plugin.CliConnection) (string, error) {
	fmt.Println("Getting current app...")
	
	// Use exec.Command to run cf apps directly
	cmd := exec.Command("cf", "apps")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to list apps: %w", err)
	}

	// Parse the output to find the first app
	lines := strings.Split(string(output), "\n")
	
	// Skip header lines and find first app
	for i, line := range lines {
		if i < 3 { // Skip "Getting apps...", empty line, and header line
			continue
		}
		
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		// Extract app name (first column)
		fields := strings.Fields(line)
		if len(fields) > 0 {
			appName := fields[0]
			fmt.Printf("Found app: %s\n", appName)
			return appName, nil
		}
	}

	return "", fmt.Errorf("no apps found in current space")
}

func executePromptAsTask(cliConnection plugin.CliConnection, app, prompt string) error {
	fmt.Println("Executing prompt as task...")
	
	// Create task command from prompt
	taskCmd := fmt.Sprintf("echo \"Executing: %s\"", strings.ReplaceAll(prompt, "\"", "\\\""))
	
	fmt.Printf("Running task on app '%s' with command: %s\n", app, taskCmd)

	// Use exec.Command to run cf run-task directly
	cmd := exec.Command("cf", "run-task", app, "--command", taskCmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Run-task output: %s\n", string(output))
		return fmt.Errorf("failed to create task: %w", err)
	}

	fmt.Printf("Task output: %s\n", string(output))
	fmt.Println("Task created successfully")
	return nil
}

