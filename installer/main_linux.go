package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
)

func init() {
	installOllama = installOllamaLinux
}

func installOllamaLinux(ctx context.Context, release string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to find home: %w", err)
	}
	succeeded := false
	outputDir := filepath.Join(home, ".ollama", "ollama")
	executablePath := filepath.Join(outputDir, "bin", "ollama")

	if _, err := os.Stat(executablePath); err == nil {
		return executablePath, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("failed to check ollama executable: %w", err)
	}

	defer func() {
		if !succeeded {
			// On failure, remove partially extracted files.
			_ = os.RemoveAll(outputDir)
		}
	}()

	filename := "ollama-linux-amd64.tgz"
	if runtime.GOARCH == "arm64" {
		filename = "ollama-linux-arm64.tgz"
	}
	assetURL, err := getReleaseAssetURL(ctx, release, filename)
	if err != nil {
		return "", err
	}

	log.Printf("Downloading ollama from %s...", assetURL)

	// For Linux, Ollama is an archive that we need to extract.
	//TODO: Support ROCm
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download ollama: %w", err)
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("error downloading ollama: status %s", resp.Status)
	}
	defer resp.Body.Close()

	gzipReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read gzip archive: %w", err)
	}
	tarReader := tar.NewReader(gzipReader)
	var links []tar.Header
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("error reading tar archive: %w", err)
		}
		if !filepath.IsLocal(header.Name) {
			return "", fmt.Errorf("error extracting archive: path %s: %w", header.Name, tar.ErrInsecurePath)
		}
		outPath := filepath.Join(outputDir, header.Name)
		info := header.FileInfo()
		switch header.Typeflag {
		case tar.TypeDir:
			if err = os.MkdirAll(outPath, info.Mode()); err != nil {
				return "", fmt.Errorf("error extracting %s: failed to make directory: %w", header.Name, err)
			}
			if err = os.Chmod(outPath, header.FileInfo().Mode()); err != nil {
				return "", fmt.Errorf("error extracting %s: failed to change permissions: %w", header.Name, err)
			}
		case tar.TypeReg:
			file, err := os.OpenFile(outPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
			if err != nil {
				return "", fmt.Errorf("error extracting %s: failed to create file: %w", header.Name, err)
			}
			n, err := io.Copy(file, tarReader)
			file.Close()
			if err != nil {
				return "", fmt.Errorf("error extracting %s: failed to copy: %w", header.Name, err)
			}
			if n < header.Size {
				return "", fmt.Errorf("error extracting %s: extracted %d of %d bytes", header.Name, n, header.Size)
			}
		case tar.TypeLink, tar.TypeSymlink:
			// defer hard & symlink creation until the files exist; note we copy here.
			if !filepath.IsLocal(header.Linkname) {
				return "", fmt.Errorf("error extracting %s: %w", header.Name, tar.ErrInsecurePath)
			}
			links = append(links, *header)
		default:
			return "", fmt.Errorf("error extracting %s: don't know how to handle %v", header.Name, header.Typeflag)
		}
	}

	for _, link := range links {
		newName := filepath.Join(outputDir, link.Name)
		oldName := filepath.Join(outputDir, link.Linkname)
		if link.Typeflag == tar.TypeLink {
			err = os.Link(oldName, newName)
		} else {
			err = os.Symlink(oldName, newName)
		}
		if err != nil {
			return "", fmt.Errorf("error extracting %s: could not create link: %w", link.Name, err)
		}
	}

	succeeded = true

	return executablePath, nil
}
