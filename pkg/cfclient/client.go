package cfclient

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/config"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
)

type Client struct {
	cf *client.Client
}

func New(apiURL, token string) (*Client, error) {
	cfg, err := config.New(apiURL, config.Token(token, ""), config.SkipTLSValidation())
	if err != nil {
		return nil, fmt.Errorf("failed to create config: %w", err)
	}

	cf, err := client.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return &Client{cf: cf}, nil
}

func (c *Client) GetAppGUID(appName, spaceGUID string) (string, error) {
	opts := client.NewAppListOptions()
	opts.SpaceGUIDs = client.Filter{Values: []string{spaceGUID}}
	opts.Names = client.Filter{Values: []string{appName}}

	apps, err := c.cf.Applications.ListAll(context.Background(), opts)
	if err != nil {
		return "", fmt.Errorf("failed to list apps: %w", err)
	}

	if len(apps) == 0 {
		return "", fmt.Errorf("app '%s' not found", appName)
	}

	return apps[0].GUID, nil
}

func (c *Client) GetLatestPackage(appGUID string) (*resource.Package, error) {
	opts := client.NewPackageListOptions()
	opts.AppGUIDs = client.Filter{Values: []string{appGUID}}
	opts.States = client.Filter{Values: []string{"READY"}}
	opts.OrderBy = "-created_at"

	packages, err := c.cf.Packages.ListAll(context.Background(), opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list packages: %w", err)
	}

	if len(packages) == 0 {
		return nil, fmt.Errorf("no ready packages found for app")
	}

	return packages[0], nil
}

func (c *Client) DownloadPackage(pkg *resource.Package, destDir string) error {
	// Check if this is an image-based package (Korifi)
	if pkg.Data.Docker != nil || (pkg.Data.Bits != nil && pkg.DataRaw != nil) {
		// Parse the raw data to check for image field
		var data map[string]interface{}
		if err := json.Unmarshal(pkg.DataRaw, &data); err == nil {
			if imageURL, exists := data["image"]; exists {
				return c.downloadFromImage(imageURL.(string), destDir)
			}
		}
	}

	// Fallback to traditional download
	downloadURL := pkg.Links["download"].Href
	fmt.Printf("Download URL: %s\n", downloadURL)

	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create download request: %w", err)
	}

	httpClient := c.cf.HTTPAuthClient()

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download package: %w", err)
	}
	defer resp.Body.Close()

	fmt.Printf("Download response status: %d\n", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download package: status %d", resp.StatusCode)
	}

	zipFile := filepath.Join(destDir, "package.zip")
	out, err := os.Create(zipFile)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save package: %w", err)
	}

	return c.unzip(zipFile, destDir)
}

func (c *Client) unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return fmt.Errorf("failed to open zip entry: %w", err)
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()

		if err != nil {
			return fmt.Errorf("failed to extract file: %w", err)
		}
	}

	os.Remove(src)
	return nil
}

func (c *Client) CreatePackage(appGUID, sourceDir string) (*resource.Package, error) {
	pkgCreate := resource.NewPackageCreate(appGUID)
	pkg, err := c.cf.Packages.Create(context.Background(), pkgCreate)
	if err != nil {
		return nil, fmt.Errorf("failed to create package: %w", err)
	}

	zipFile := filepath.Join(os.TempDir(), fmt.Sprintf("package-%s.zip", pkg.GUID))
	if err := c.zipDirectory(sourceDir, zipFile); err != nil {
		return nil, fmt.Errorf("failed to zip directory: %w", err)
	}
	defer os.Remove(zipFile)

	file, err := os.Open(zipFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip file: %w", err)
	}
	defer file.Close()

	uploadURL := pkg.Links["upload"].Href

	req, err := http.NewRequest("POST", uploadURL, file)
	if err != nil {
		return nil, fmt.Errorf("failed to create upload request: %w", err)
	}

	req.Header.Set("Content-Type", "application/zip")

	httpClient := c.cf.HTTPAuthClient()

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to upload package: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("failed to upload package: status %d", resp.StatusCode)
	}

	return pkg, nil
}

func (c *Client) downloadFromImage(imageURL, destDir string) error {
	fmt.Printf("Downloading from container image: %s\n", imageURL)

	// Create a temporary container to extract source
	containerName := fmt.Sprintf("cf-prompt-extract-%d", os.Getpid())

	// Try to create and copy from container
	createCmd := exec.Command("docker", "create", "--name", containerName, imageURL)
	if err := createCmd.Run(); err != nil {
		return fmt.Errorf("failed to create container from image: %w", err)
	}

	// Ensure cleanup
	defer exec.Command("docker", "rm", containerName).Run()

	// Copy files from container
	copyCmd := exec.Command("docker", "cp", containerName+":/workspace/.", destDir)
	if err := copyCmd.Run(); err != nil {
		// Try alternative paths
		copyCmd = exec.Command("docker", "cp", containerName+":/app/.", destDir)
		if err := copyCmd.Run(); err != nil {
			copyCmd = exec.Command("docker", "cp", containerName+":/.", destDir)
			if err := copyCmd.Run(); err != nil {
				return fmt.Errorf("failed to copy files from container: %w", err)
			}
		}
	}

	fmt.Printf("Successfully extracted source from container image\n")
	return nil
}
func (c *Client) zipDirectory(source, target string) error {
	zipfile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer zipfile.Close()

	archive := zip.NewWriter(zipfile)
	defer archive.Close()

	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		header.Name = relPath

		header.Method = zip.Deflate

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})
}
