package main

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/cli/plugin"

	"github.com/ruben/cf-prompt-cli-plugin/cmd"
)

type PromptPlugin struct{}

func (p *PromptPlugin) Run(cliConnection plugin.CliConnection, args []string) {
	if len(args) == 0 {
		fmt.Println("Error: No command provided")
		os.Exit(1)
	}

	switch args[0] {
	case "prompt":
		cmd.PromptCommand(cliConnection, args[1:])
	default:
		fmt.Printf("Error: Unknown command '%s'\n", args[0])
		os.Exit(1)
	}
}

func (p *PromptPlugin) GetMetadata() plugin.PluginMetadata {
	return plugin.PluginMetadata{
		Name: "prompt",
		Version: plugin.VersionType{
			Major: 1,
			Minor: 0,
			Build: 0,
		},
		MinCliVersion: plugin.VersionType{
			Major: 6,
			Minor: 7,
			Build: 0,
		},
		Commands: []plugin.Command{
			{
				Name:     "prompt",
				HelpText: "Execute a natural language prompt as a CF task",
				UsageDetails: plugin.Usage{
					Usage: "cf prompt [prompt text]",
					Options: map[string]string{
						"app": "Target application name",
					},
				},
			},
		},
	}
}

func main() {
	plugin.Start(new(PromptPlugin))
}