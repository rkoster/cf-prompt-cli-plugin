package opencode

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func Run(workDir, prompt string, stdout io.Writer) error {
	binaryPath := getOpencodeBinaryPath()
	
	cmd := exec.Command(binaryPath, "run", prompt)
	cmd.Dir = workDir
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("opencode run failed: %w", err)
	}

	return nil
}

func getOpencodeBinaryPath() string {
	installDir := getInstallDir()
	binaryName := "opencode"
	if runtime.GOOS == "windows" {
		binaryName = "opencode.exe"
	}
	return filepath.Join(installDir, binaryName)
}
