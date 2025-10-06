package registry

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/ruben/cf-prompt-cli-plugin/pkg/cfclient"
)

type Client struct {
	username string
	password string
}

func NewClient(username, password string) (*Client, error) {
	return &Client{
		username: username,
		password: password,
	}, nil
}

func (c *Client) DownloadPackage(pkg interface{}, destDir string) error {
	type Package interface {
		GetMetadata() map[string]interface{}
	}

	if pkgWithMetadata, ok := pkg.(Package); ok {
		metadata := pkgWithMetadata.GetMetadata()
		if imageRef, exists := metadata["image"]; exists {
			return c.downloadFromRegistry(imageRef.(string), destDir)
		}
	}

	return fmt.Errorf("package does not contain OCI image reference")
}

func (c *Client) downloadFromRegistry(imageRef string, destDir string) error {
	fmt.Printf("Downloading OCI image: %s\n", imageRef)

	var opts []crane.Option
	if c.username != "" && c.password != "" {
		opts = append(opts, crane.WithAuth(&authn.Basic{
			Username: c.username,
			Password: c.password,
		}))
	}

	img, err := crane.Pull(imageRef, opts...)
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}

	return c.extractImage(img, destDir)
}

func (c *Client) extractImage(img v1.Image, destDir string) error {
	layers, err := img.Layers()
	if err != nil {
		return fmt.Errorf("failed to get image layers: %w", err)
	}

	for i, layer := range layers {
		fmt.Printf("Extracting layer %d/%d...\n", i+1, len(layers))

		rc, err := layer.Uncompressed()
		if err != nil {
			return fmt.Errorf("failed to uncompress layer: %w", err)
		}

		tarReader := tar.NewReader(rc)
		for {
			header, err := tarReader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				rc.Close()
				return fmt.Errorf("failed to read tar entry: %w", err)
			}

			target := filepath.Join(destDir, header.Name)

			switch header.Typeflag {
			case tar.TypeDir:
				if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
					rc.Close()
					return fmt.Errorf("failed to create directory: %w", err)
				}
			case tar.TypeReg:
				if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
					rc.Close()
					return fmt.Errorf("failed to create parent directory: %w", err)
				}

				outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
				if err != nil {
					rc.Close()
					return fmt.Errorf("failed to create file: %w", err)
				}

				if _, err := io.Copy(outFile, tarReader); err != nil {
					outFile.Close()
					rc.Close()
					return fmt.Errorf("failed to write file: %w", err)
				}
				outFile.Close()
			}
		}
		rc.Close()
	}

	fmt.Printf("Successfully extracted image to %s\n", destDir)
	return nil
}

func (c *Client) UploadPackage(client *cfclient.Client, appGUID string, sourceDir string, prompt string) error {
	fmt.Printf("Creating package from directory: %s\n", sourceDir)

	pkg, err := client.CreatePackageWithPrompt(appGUID, sourceDir, prompt)
	if err != nil {
		return fmt.Errorf("failed to create package: %w", err)
	}

	fmt.Printf("Package created successfully: %s\n", pkg.GUID)

	fmt.Printf("Triggering build for package %s...\n", pkg.GUID)
	buildGUID, err := client.TriggerBuild(pkg.GUID)
	if err != nil {
		return fmt.Errorf("failed to trigger build: %w", err)
	}

	fmt.Printf("Build triggered successfully: %s\n", buildGUID)
	fmt.Println("Build is now staging (check status with 'cf prompts <app-name>')")

	return nil
}
