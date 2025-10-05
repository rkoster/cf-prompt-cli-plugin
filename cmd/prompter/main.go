package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ruben/cf-prompt-cli-plugin/pkg/cfclient"
	"github.com/ruben/cf-prompt-cli-plugin/pkg/opencode"
	"github.com/ruben/cf-prompt-cli-plugin/pkg/registry"
)

type Config struct {
	AccessToken      string
	API              string
	AppID            string
	SpaceID          string
	OrgID            string
	RegistryUsername string
	RegistryPassword string
	Prompt           string
}

func main() {
	if err := opencode.EnsureInstalled(); err != nil {
		fmt.Fprintf(os.Stderr, "Error installing opencode: %v\n", err)
		os.Exit(1)
	}

	config, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	if err := run(config); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func loadConfig() (*Config, error) {
	promptBase64 := os.Getenv("PROMPT_BASE64")
	if promptBase64 == "" {
		return nil, fmt.Errorf("PROMPT_BASE64 environment variable is required")
	}

	promptBytes, err := base64.StdEncoding.DecodeString(promptBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode PROMPT_BASE64: %w", err)
	}

	config := &Config{
		AccessToken:      os.Getenv("CF_ACCESS_TOKEN"),
		API:              os.Getenv("CF_API"),
		AppID:            os.Getenv("APP_ID"),
		SpaceID:          os.Getenv("SPACE_ID"),
		OrgID:            os.Getenv("ORG_ID"),
		RegistryUsername: os.Getenv("REGISTRY_USERNAME"),
		RegistryPassword: os.Getenv("REGISTRY_PASSWORD"),
		Prompt:           string(promptBytes),
	}

	if config.AccessToken == "" {
		return nil, fmt.Errorf("CF_ACCESS_TOKEN environment variable is required")
	}
	if config.API == "" {
		return nil, fmt.Errorf("CF_API environment variable is required")
	}
	if config.AppID == "" {
		return nil, fmt.Errorf("APP_ID environment variable is required")
	}
	if config.SpaceID == "" {
		return nil, fmt.Errorf("SPACE_ID environment variable is required")
	}

	return config, nil
}

func run(config *Config) error {
	workDir, err := os.MkdirTemp("", "cf-prompter-*")
	if err != nil {
		return fmt.Errorf("failed to create working directory: %w", err)
	}
	defer os.RemoveAll(workDir)

	fmt.Println("Creating CF client...")
	client, err := cfclient.New(config.API, config.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to create CF client: %w", err)
	}

	fmt.Printf("Getting latest package for app %s...\n", config.AppID)
	pkg, err := client.GetLatestPackage(config.AppID)
	if err != nil {
		return fmt.Errorf("failed to get latest package: %w", err)
	}

	fmt.Printf("Downloading package %s...\n", pkg.GUID)

	regClient, err := registry.NewClient(config.RegistryUsername, config.RegistryPassword)
	if err != nil {
		return fmt.Errorf("failed to create registry client: %w", err)
	}

	if err := client.DownloadPackage(pkg, workDir); err != nil {
		return fmt.Errorf("failed to download package: %w", err)
	}

	fmt.Println("Package downloaded successfully")

	packageDir := filepath.Join(workDir, "app")
	if _, err := os.Stat(packageDir); os.IsNotExist(err) {
		packageDir = workDir
	}

	fmt.Println("\nExecuting opencode run...")
	fmt.Println("================================================================================")
	if err := opencode.Run(packageDir, config.Prompt, os.Stdout); err != nil {
		return fmt.Errorf("opencode run failed: %w", err)
	}
	fmt.Println("================================================================================")

	fmt.Println("\nCreating new package revision...")
	if err := regClient.UploadPackage(client, config.AppID, packageDir); err != nil {
		return fmt.Errorf("failed to create new package: %w", err)
	}

	fmt.Println("Prompter completed successfully")
	return nil
}
