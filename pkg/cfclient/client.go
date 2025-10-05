package cfclient

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/config"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type Client struct {
	cf     *client.Client
	apiURL string
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

	return &Client{cf: cf, apiURL: apiURL}, nil
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
	// Debug: list contents of source directory
	fmt.Printf("Creating package from source directory: %s\n", sourceDir)
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read source directory: %w", err)
	}
	fmt.Printf("Source directory contains %d entries:\n", len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			fmt.Printf("  [DIR]  %s\n", entry.Name())
		} else {
			fmt.Printf("  [FILE] %s\n", entry.Name())
		}
	}

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

	// Debug: check zip file size
	zipInfo, err := os.Stat(zipFile)
	if err != nil {
		return nil, fmt.Errorf("failed to stat zip file: %w", err)
	}
	fmt.Printf("Created zip file %s with size %d bytes\n", zipFile, zipInfo.Size())

	file, err := os.Open(zipFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip file: %w", err)
	}
	defer file.Close()

	uploadURL := pkg.Links["upload"].Href

	// Fix localhost URLs by replacing with the correct API endpoint
	if strings.Contains(uploadURL, "localhost") || strings.Contains(uploadURL, "127.0.0.1") {
		// Extract the path from the upload URL
		uploadPath := strings.TrimPrefix(uploadURL, "https://localhost:443")
		uploadPath = strings.TrimPrefix(uploadPath, "http://localhost:443")
		uploadPath = strings.TrimPrefix(uploadPath, "https://127.0.0.1:443")
		uploadPath = strings.TrimPrefix(uploadPath, "http://127.0.0.1:443")
		uploadURL = c.apiURL + uploadPath
	}

	// Create a multipart form upload
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Add the bits file field
	part, err := writer.CreateFormFile("bits", "package.zip")
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	// Copy the zip file content to the form field
	_, err = io.Copy(part, file)
	if err != nil {
		return nil, fmt.Errorf("failed to copy zip content: %w", err)
	}

	// Close the multipart writer
	err = writer.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	req, err := http.NewRequest("POST", uploadURL, &requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create upload request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	httpClient := c.cf.HTTPAuthClient()

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to upload package: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to upload package: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return pkg, nil
}

func (c *Client) downloadFromImage(imageURL, destDir string) error {
	// Parse the image reference
	ref, err := name.ParseReference(imageURL)
	if err != nil {
		return fmt.Errorf("failed to parse image reference: %w", err)
	}

	// Create authenticator for registry
	var auth authn.Authenticator = authn.Anonymous

	// Check for registry credentials in environment
	if username := os.Getenv("REGISTRY_USERNAME"); username != "" {
		if password := os.Getenv("REGISTRY_PASSWORD"); password != "" {
			auth = &authn.Basic{
				Username: username,
				Password: password,
			}
		}
	}

	// Pull the image with authentication
	img, err := remote.Image(ref, remote.WithAuth(auth))
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}

	// Get the layers
	layers, err := img.Layers()
	if err != nil {
		return fmt.Errorf("failed to get image layers: %w", err)
	}

	// Extract files from all layers
	for _, layer := range layers {
		if err := c.extractLayerToDir(layer, destDir); err != nil {
			return fmt.Errorf("failed to extract layer: %w", err)
		}
	}

	return nil
}

func (c *Client) extractLayerToDir(layer v1.Layer, destDir string) error {
	rc, err := layer.Uncompressed()
	if err != nil {
		return fmt.Errorf("failed to get layer contents: %w", err)
	}
	defer rc.Close()

	tarReader := tar.NewReader(rc)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		// Skip directories and non-regular files
		if header.Typeflag != tar.TypeReg {
			continue
		}

		// Create the destination file path
		destPath := filepath.Join(destDir, header.Name)

		// Ensure the directory exists
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		// Create and write the file
		outFile, err := os.Create(destPath)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", destPath, err)
		}

		_, err = io.Copy(outFile, tarReader)
		outFile.Close()

		if err != nil {
			return fmt.Errorf("failed to write file %s: %w", destPath, err)
		}

		// Set file permissions
		if err := os.Chmod(destPath, os.FileMode(header.Mode)); err != nil {
			return fmt.Errorf("failed to set permissions for %s: %w", destPath, err)
		}
	}

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
