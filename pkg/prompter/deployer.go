package prompter

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/cli/plugin"
)

type AppDeployer struct {
	cliConnection plugin.CliConnection
	appName       string
}

func NewAppDeployer(cliConnection plugin.CliConnection) *AppDeployer {
	return &AppDeployer{
		cliConnection: cliConnection,
		appName:       fmt.Sprintf("cf-prompter-%d", time.Now().Unix()),
	}
}

func (d *AppDeployer) Deploy(apiEndpoint, token, appID, spaceID, orgID, registryUsername, registryPassword, prompt string) error {
	tempDir, err := os.MkdirTemp("", "cf-prompter-app-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	prompterBinaryPath := filepath.Join(tempDir, "prompter")
	
	if len(PrompterBinary) == 0 {
		return fmt.Errorf("prompter binary not embedded - ensure 'make build-prompter' was run before building the plugin")
	}
	
	if err := os.WriteFile(prompterBinaryPath, PrompterBinary, 0755); err != nil {
		return fmt.Errorf("failed to write prompter binary: %w", err)
	}

	procfilePath := filepath.Join(tempDir, "Procfile")
	procfile := "web: ./prompter\n"
	if err := os.WriteFile(procfilePath, []byte(procfile), 0644); err != nil {
		return fmt.Errorf("failed to write Procfile: %w", err)
	}

	manifestPath := filepath.Join(tempDir, "manifest.yml")
	promptBase64 := base64.StdEncoding.EncodeToString([]byte(prompt))
	
	manifest := fmt.Sprintf(`---
applications:
- name: %s
  memory: 1G
  disk_quota: 2G
  instances: 1
  no-route: true
  health-check-type: process
  buildpacks:
  - procfile_buildpack
  env:
    CF_ACCESS_TOKEN: %s
    CF_API: %s
    APP_ID: %s
    SPACE_ID: %s
    ORG_ID: %s
    REGISTRY_USERNAME: %s
    REGISTRY_PASSWORD: %s
    PROMPT_BASE64: %s
`, d.appName, token, apiEndpoint, appID, spaceID, orgID, registryUsername, registryPassword, promptBase64)

	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	fmt.Printf("Pushing prompter app '%s' to CF...\n", d.appName)
	if _, err := d.cliConnection.CliCommand("push", d.appName, "-p", tempDir, "-f", manifestPath); err != nil {
		return fmt.Errorf("failed to push app: %w", err)
	}

	fmt.Println("Prompter app pushed successfully")
	return nil
}

func (d *AppDeployer) MonitorLogs(stdout io.Writer) error {
	fmt.Println("Monitoring logs for completion...")
	
	timeout := time.After(30 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	lastLogTime := time.Now()
	seenCompletion := false
	
	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for prompter to complete")
		case <-ticker.C:
			output, err := d.cliConnection.CliCommandWithoutTerminalOutput("logs", d.appName, "--recent")
			if err != nil {
				continue
			}
			
			logsStr := strings.Join(output, "\n")
			
			if strings.Contains(logsStr, "Prompter completed successfully") {
				if !seenCompletion {
					fmt.Fprintln(stdout, logsStr)
					fmt.Println("\nPrompter execution completed!")
					seenCompletion = true
				}
				time.Sleep(2 * time.Second)
				return nil
			}
			
			if strings.Contains(logsStr, "opencode run failed") || 
			   strings.Contains(logsStr, "Error:") && strings.Contains(logsStr, "PROMPT_BASE64") {
				fmt.Fprintln(stdout, logsStr)
				return fmt.Errorf("prompter execution failed - check logs above")
			}
			
			if time.Since(lastLogTime) > 3*time.Second && logsStr != "" {
				fmt.Fprintln(stdout, logsStr)
				lastLogTime = time.Now()
			}
		}
	}
}

func (d *AppDeployer) Cleanup() error {
	fmt.Printf("Deleting prompter app '%s'...\n", d.appName)
	if _, err := d.cliConnection.CliCommand("delete", d.appName, "-f"); err != nil {
		return fmt.Errorf("failed to delete app: %w", err)
	}
	
	fmt.Println("Prompter app deleted successfully")
	return nil
}
