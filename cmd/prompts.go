package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"code.cloudfoundry.org/cli/plugin"
	"github.com/ruben/cf-prompt-cli-plugin/pkg/cfclient"
)

func shortHash(guid string) string {
	hash := sha256.Sum256([]byte(guid))
	hexHash := hex.EncodeToString(hash[:])
	return hexHash[:7]
}

type simpleTable struct {
	headers     []string
	rows        [][]string
	columnWidth []int
}

// newSimpleTable creates a new table with specified headers
func newSimpleTable(headers []string) *simpleTable {
	t := &simpleTable{
		headers:     headers,
		columnWidth: make([]int, len(headers)),
	}

	// Initialize column widths with header lengths
	for i, header := range headers {
		t.columnWidth[i] = len(header)
	}

	return t
}

// addRow adds a row to the table and updates column widths
func (t *simpleTable) addRow(values ...string) {
	t.rows = append(t.rows, values)

	// Update column widths based on this row
	for i, value := range values {
		if i < len(t.columnWidth) && len(value) > t.columnWidth[i] {
			t.columnWidth[i] = len(value)
		}
	}
}

// print outputs the table in CF CLI format
func (t *simpleTable) print() {
	// Print headers
	headerLine := ""
	for i, header := range t.headers {
		if i == len(t.headers)-1 {
			// Last column doesn't need padding
			headerLine += header
		} else {
			headerLine += fmt.Sprintf("%-*s   ", t.columnWidth[i], header)
		}
	}
	fmt.Println(headerLine)

	// Print rows
	for _, row := range t.rows {
		rowLine := ""
		for i, cell := range row {
			if i >= len(t.headers) {
				break // Don't exceed number of headers
			}
			if i == len(t.headers)-1 {
				// Last column doesn't need padding
				rowLine += cell
			} else {
				rowLine += fmt.Sprintf("%-*s   ", t.columnWidth[i], cell)
			}
		}
		fmt.Println(rowLine)
	}
}

func PromptsCommand(cliConnection plugin.CliConnection, args []string) {
	if len(args) == 0 {
		fmt.Println("Error: No app name provided")
		fmt.Println("Usage: cf prompts <APP_NAME>")
		os.Exit(1)
	}

	appName := args[0]

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

	appGUID, err := client.GetAppGUID(appName, currentSpace.Guid)
	if err != nil {
		fmt.Printf("Error getting app GUID for '%s': %v\n", appName, err)
		os.Exit(1)
	}

	packages, err := client.ListPackagesWithPrompts(appGUID)
	if err != nil {
		fmt.Printf("Error listing packages: %v\n", err)
		os.Exit(1)
	}

	// Get current org and user info to match cf apps format
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

	// Print header like cf apps
	fmt.Printf("Getting packages for app %s in org %s / space %s as %s...\n\n", appName, currentOrg.Name, currentSpace.Name, username)

	if len(packages) == 0 {
		fmt.Printf("No packages found for app '%s'\n", appName)
		return
	}

	table := newSimpleTable([]string{"hash", "state", "created", "type", "original prompt"})

	for _, pkg := range packages {
		hash := shortHash(pkg.GUID)
		createdAt := pkg.CreatedAt.Format("2006-01-02 15:04:05")

		prompt, hasPrompt := client.GetOriginalPrompt(pkg)
		if !hasPrompt {
			prompt = "(no prompt stored)"
		}

		if len(prompt) > 50 {
			prompt = prompt[:47] + "..."
		}

		state := strings.ToLower(string(pkg.State))
		table.addRow(hash, state, createdAt, pkg.Type, prompt)
	}

	table.print()
}
