package opencode

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
)

const OpencodeVersion = "0.8.8"

func EnsureInstalled() error {
	installDir := getInstallDir()
	
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	binaryPath := filepath.Join(installDir, "opencode")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}

	if _, err := os.Stat(binaryPath); err == nil {
		return nil
	}

	fmt.Printf("Installing opencode version %s...\n", OpencodeVersion)
	
	if err := downloadAndInstall(installDir); err != nil {
		return fmt.Errorf("failed to install opencode: %w", err)
	}

	fmt.Println("opencode installed successfully")
	return nil
}

func getInstallDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), ".opencode", "bin")
	}
	return filepath.Join(homeDir, ".opencode", "bin")
}

func downloadAndInstall(installDir string) error {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	if arch == "amd64" {
		arch = "x64"
	}

	if err := validatePlatform(osName, arch); err != nil {
		return err
	}

	filename := fmt.Sprintf("opencode-%s-%s.zip", osName, arch)
	url := fmt.Sprintf("https://github.com/sst/opencode/releases/download/v%s/%s", OpencodeVersion, filename)

	tempDir, err := os.MkdirTemp("", "opencode-install-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	zipPath := filepath.Join(tempDir, filename)
	if err := downloadFile(url, zipPath); err != nil {
		return fmt.Errorf("failed to download opencode: %w", err)
	}

	if err := extractZip(zipPath, tempDir); err != nil {
		return fmt.Errorf("failed to extract opencode: %w", err)
	}

	binaryName := "opencode"
	if runtime.GOOS == "windows" {
		binaryName = "opencode.exe"
	}

	sourcePath := filepath.Join(tempDir, binaryName)
	destPath := filepath.Join(installDir, binaryName)

	if err := os.Rename(sourcePath, destPath); err != nil {
		return fmt.Errorf("failed to move binary: %w", err)
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(destPath, 0755); err != nil {
			return fmt.Errorf("failed to make binary executable: %w", err)
		}
	}

	return nil
}

func validatePlatform(osName, arch string) error {
	switch osName {
	case "linux", "darwin":
		if arch != "x64" && arch != "arm64" {
			return fmt.Errorf("unsupported architecture for %s: %s", osName, arch)
		}
	case "windows":
		if arch != "x64" {
			return fmt.Errorf("unsupported architecture for windows: %s", arch)
		}
	default:
		return fmt.Errorf("unsupported OS: %s", osName)
	}
	return nil
}

func downloadFile(url, filepath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func extractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		path := filepath.Join(destDir, f.Name)
		
		outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}

	return nil
}
