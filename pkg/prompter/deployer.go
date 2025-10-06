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

func NewAppDeployer(cliConnection plugin.CliConnection, appName string) *AppDeployer {
	return &AppDeployer{
		cliConnection: cliConnection,
		appName:       appName,
	}
}

func (d *AppDeployer) StartPrompter(apiEndpoint, token, appID, spaceID, orgID, registryUsername, registryPassword, prompt string) error {
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = token[7:]
	}

	client, err := cfclient.New(apiEndpoint, token)
	if err != nil {
		return fmt.Errorf("failed to create CF client: %w", err)
	}
	d.cfClient = client

	promptBase64 := base64.StdEncoding.EncodeToString([]byte(prompt))

	prompterApiEndpoint := apiEndpoint
	if strings.Contains(apiEndpoint, "localhost") {
		prompterApiEndpoint = "https://korifi-api-svc.korifi.svc.cluster.local"
	}

	fmt.Printf("Setting environment variables for prompter app '%s'...\n", d.appName)

	envVars := map[string]string{
		"CF_ACCESS_TOKEN":   token,
		"CF_API":            prompterApiEndpoint,
		"APP_ID":            appID,
		"SPACE_ID":          spaceID,
		"ORG_ID":            orgID,
		"REGISTRY_USERNAME": registryUsername,
		"REGISTRY_PASSWORD": registryPassword,
		"PROMPT_BASE64":     promptBase64,
	}

	for key, value := range envVars {
		if _, err := d.cliConnection.CliCommand("set-env", d.appName, key, value); err != nil {
			return fmt.Errorf("failed to set environment variable %s: %w", key, err)
		}
	}

	fmt.Printf("Starting prompter app '%s'...\n", d.appName)
	if _, err := d.cliConnection.CliCommand("start", d.appName); err != nil {
		return fmt.Errorf("failed to start app: %w", err)
	}

	fmt.Println("Prompter app started successfully")
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

func (d *AppDeployer) StopPrompter() error {
	fmt.Printf("Stopping prompter app '%s'...\n", d.appName)
	if _, err := d.cliConnection.CliCommand("stop", d.appName); err != nil {
		return fmt.Errorf("failed to stop app: %w", err)
	}

	fmt.Println("Prompter app stopped successfully")
	return nil
}

type PrompterInitDeployer struct {
	cliConnection plugin.CliConnection
	appName       string
	prompterName  string
	cfClient      *cfclient.Client
}

func NewPrompterInitDeployer(cliConnection plugin.CliConnection, appName string) *PrompterInitDeployer {
	return &PrompterInitDeployer{
		cliConnection: cliConnection,
		appName:       appName,
		prompterName:  fmt.Sprintf("%s-prompter", appName),
	}
}

func (d *PrompterInitDeployer) DeployPrompterApp(apiEndpoint, token string) error {
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = token[7:]
	}

	client, err := cfclient.New(apiEndpoint, token)
	if err != nil {
		return fmt.Errorf("failed to create CF client: %w", err)
	}
	d.cfClient = client

	currentSpace, err := d.cliConnection.GetCurrentSpace()
	if err != nil {
		return fmt.Errorf("failed to get current space: %w", err)
	}

	existingAppGUID, err := d.cfClient.GetAppGUID(d.prompterName, currentSpace.Guid)
	if err == nil && existingAppGUID != "" {
		fmt.Printf("Prompter app '%s' already exists, deleting...\n", d.prompterName)
		if _, err := d.cliConnection.CliCommand("delete", d.prompterName, "-f"); err != nil {
			return fmt.Errorf("failed to delete existing prompter app: %w", err)
		}
		time.Sleep(2 * time.Second)
	}

	tempDir, err := os.MkdirTemp("", "cf-prompter-init-*")
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
	manifest := fmt.Sprintf(`---
applications:
- name: %s
  memory: 1G
  disk_quota: 2G
  instances: 1
  no-route: true
  health-check-type: process
  buildpack: paketo-buildpacks/procfile
`, d.prompterName)

	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	fmt.Printf("Step 1/3: Preparing prompter app '%s'...\n", d.prompterName)

	fmt.Printf("Step 2/3: Pushing prompter app")
	progressDone := make(chan bool)
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				fmt.Print(".")
			case <-progressDone:
				return
			}
		}
	}()

	cmd := exec.Command("cf", "push", d.prompterName, "-p", tempDir, "-f", manifestPath)
	output, err := cmd.CombinedOutput()
	progressDone <- true
	fmt.Println()

	if err != nil {
		fmt.Println(string(output))
		return fmt.Errorf("failed to push app: %w", err)
	}

	fmt.Printf("Step 3/3: Stopping prompter app (ready for use)...\n")
	if _, err := d.cliConnection.CliCommand("stop", d.prompterName); err != nil {
		return fmt.Errorf("failed to stop prompter app: %w", err)
	}

	return nil
}
