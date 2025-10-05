package opencode

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

func Run(workDir, prompt string, stdout io.Writer) error {
	cmd := exec.Command("opencode", "run", prompt)
	cmd.Dir = workDir
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("opencode run failed: %w", err)
	}

	return nil
}
