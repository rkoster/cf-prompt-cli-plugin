package prompter

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/cli/plugin"
	"github.com/ruben/cf-prompt-cli-plugin/pkg/cfclient"
)

type AppDeployer struct {
	cliConnection plugin.CliConnection
	appName       string
	cfClient      *cfclient.Client
}

func NewAppDeployer(cliConnection plugin.CliConnection) *AppDeployer {
	return &AppDeployer{
		cliConnection: cliConnection,
		appName:       fmt.Sprintf("cf-prompter-%d", time.Now().Unix()),
	}
}

func (d *AppDeployer) Deploy(apiEndpoint, token, appID, spaceID, orgID, registryUsername, registryPassword, prompt string) error {
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = token[7:]
	}

	// Use original endpoint for plugin's CF client
	client, err := cfclient.New(apiEndpoint, token)
	if err != nil {
		return fmt.Errorf("failed to create CF client: %w", err)
	}
	d.cfClient = client

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

	// Use internal cluster endpoint for prompter app running inside the cluster
	prompterApiEndpoint := apiEndpoint
	if strings.Contains(apiEndpoint, "localhost") {
		prompterApiEndpoint = "https://korifi-api-svc.korifi.svc.cluster.local"
	}

	manifest := fmt.Sprintf(`---
applications:
- name: %s
  memory: 1G
  disk_quota: 2G
  instances: 1
  no-route: true
  health-check-type: process
  buildpack: paketo-buildpacks/procfile
  env:
    CF_ACCESS_TOKEN: "%s"
    CF_API: "%s"
    APP_ID: "%s"
    SPACE_ID: "%s"
    ORG_ID: "%s"
    REGISTRY_USERNAME: "%s"
    REGISTRY_PASSWORD: "%s"
    PROMPT_BASE64: "%s"
`, d.appName, token, prompterApiEndpoint, appID, spaceID, orgID, registryUsername, registryPassword, promptBase64)

	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	fmt.Printf("Pushing prompter app '%s' to CF...\n", d.appName)

	cmd := exec.Command("cf", "push", d.appName, "-p", tempDir, "-f", manifestPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to push app: %w", err)
	}

	fmt.Println("Prompter app pushed successfully")
	return nil
}

func (d *AppDeployer) MonitorLogs(stdout io.Writer) error {
	fmt.Println("Monitoring logs for completion...")

	if d.cfClient == nil {
		return fmt.Errorf("CF client not initialized - Deploy must be called first")
	}

	currentSpace, err := d.cliConnection.GetCurrentSpace()
	if err != nil {
		return fmt.Errorf("failed to get current space: %w", err)
	}

	appGUID, err := d.cfClient.GetAppGUID(d.appName, currentSpace.Guid)
	if err != nil {
		return fmt.Errorf("failed to get prompter app GUID: %w", err)
	}

	cmd := exec.Command("cf", "logs", d.appName)
	cmd.Stdout = stdout
	cmd.Stderr = stdout

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start log streaming: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	appStateChan := make(chan string, 1)
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				app, err := d.cfClient.GetApp(appGUID)
				if err != nil {
					fmt.Fprintf(stdout, "Warning: failed to get app state: %v\n", err)
					continue
				}

				if app.State == "STOPPED" {
					fmt.Fprintf(stdout, "\nPrompter app detected as STOPPED - task completed successfully\n")
					appStateChan <- "STOPPED"
					return
				}
			}
		}
	}()

	timeout := time.After(30 * time.Minute)

	select {
	case <-timeout:
		cmd.Process.Kill()
		return fmt.Errorf("timeout waiting for prompter to complete")
	case state := <-appStateChan:
		cmd.Process.Kill()
		if state == "STOPPED" {
			return nil
		}
		return fmt.Errorf("unexpected app state: %s", state)
	case err := <-done:
		if err != nil {
			return fmt.Errorf("log streaming failed: %w", err)
		}
		return nil
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
