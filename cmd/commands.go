package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"code.cloudfoundry.org/cli/plugin"
)

func getCurrentApp(cliConnection plugin.CliConnection) (string, error) {
	fmt.Println("Getting current app...")
	
	cmd := exec.Command("cf", "apps")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to list apps: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	
	for i, line := range lines {
		if i < 3 {
			continue
		}
		
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		fields := strings.Fields(line)
		if len(fields) > 0 {
			appName := fields[0]
			fmt.Printf("Found app: %s\n", appName)
			return appName, nil
		}
	}

	return "", fmt.Errorf("no apps found in current space")
}
