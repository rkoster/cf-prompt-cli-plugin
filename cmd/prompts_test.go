package cmd

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

func TestArgumentParsing(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedApp    string
		expectedPrompt string
		shouldFail     bool
	}{
		{
			name:           "App name as first argument with -p flag",
			args:           []string{"test", "-p", "update app to say: hello from frankfurt"},
			expectedApp:    "test",
			expectedPrompt: "update app to say: hello from frankfurt",
			shouldFail:     false,
		},
		{
			name:           "App name with -a flag and -p flag",
			args:           []string{"-a", "test", "-p", "update app to say: hello from frankfurt"},
			expectedApp:    "test",
			expectedPrompt: "update app to say: hello from frankfurt",
			shouldFail:     false,
		},
		{
			name:           "App name with --app flag and --prompt flag",
			args:           []string{"--app", "test", "--prompt", "update app to say: hello from frankfurt"},
			expectedApp:    "test",
			expectedPrompt: "update app to say: hello from frankfurt",
			shouldFail:     false,
		},
		{
			name:           "Missing app name",
			args:           []string{"-p", "some prompt"},
			expectedApp:    "",
			expectedPrompt: "",
			shouldFail:     true,
		},
		{
			name:           "Missing prompt",
			args:           []string{"test"},
			expectedApp:    "",
			expectedPrompt: "",
			shouldFail:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, prompt, failed := ParsePromptArgs(tt.args)

			if tt.shouldFail && !failed {
				t.Errorf("Expected parsing to fail, but it succeeded")
			}
			if !tt.shouldFail && failed {
				t.Errorf("Expected parsing to succeed, but it failed")
			}
			if !tt.shouldFail {
				if app != tt.expectedApp {
					t.Errorf("Expected app '%s', got '%s'", tt.expectedApp, app)
				}
				if prompt != tt.expectedPrompt {
					t.Errorf("Expected prompt '%s', got '%s'", tt.expectedPrompt, prompt)
				}
			}
		})
	}
}

func TestSimpleTable(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Create a test table
	table := newSimpleTable([]string{"state", "created", "type", "original prompt"})

	// Add some test data
	table.addRow("ready", "2025-10-05 19:15:30", "bits", "Fix bug")
	table.addRow("awaiting_upload", "2025-10-05 18:42:12", "bits", "(no prompt stored)")
	table.addRow("ready", "2025-10-05 17:30:45", "bits", "Add a new endpoint that returns curre...")

	table.print()

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	fmt.Println("Table output:")
	fmt.Print(output)

	// Verify headers are present
	if !bytes.Contains([]byte(output), []byte("state")) {
		t.Error("Table should contain 'state' header")
	}
	if !bytes.Contains([]byte(output), []byte("created")) {
		t.Error("Table should contain 'created' header")
	}
	if !bytes.Contains([]byte(output), []byte("original prompt")) {
		t.Error("Table should contain 'original prompt' header")
	}

	// Verify data is present
	if !bytes.Contains([]byte(output), []byte("ready")) {
		t.Error("Table should contain 'ready' state")
	}
	if !bytes.Contains([]byte(output), []byte("Fix bug")) {
		t.Error("Table should contain 'Fix bug' prompt")
	}
}
