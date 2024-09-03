package main

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/xenking/zipstream"
)

func init() {
	installOllama = installOllamaWindows
}

func installOllamaWindows(ctx context.Context, release string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to find home: %w", err)
	}
	succeeded := false
	outputDir := filepath.Join(home, ".ollama", "ollama")
	executablePath := filepath.Join(outputDir, "ollama.exe")

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

	assetURL, err := getReleaseAssetURL(ctx, release, "ollama-windows-amd64.zip")
	if err != nil {
		return "", err
	}

	log.Printf("Downloading ollama from %s...", assetURL)

	// For Windows, Ollama is a zip archive that we need  to extract.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download ollama: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("error downloading ollama: status %s", resp.Status)
	}

	zipReader := zipstream.NewReader(resp.Body)
	for {
		info, err := zipReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("error reading ollama archive: %w", err)
		}
		if !filepath.IsLocal(info.Name) || strings.ContainsRune(info.Name, '\\') {
			return "", fmt.Errorf("error extracting archive: %s: %w", info.Name, zip.ErrInsecurePath)
		}
		outPath := filepath.Join(outputDir, info.Name)
		if strings.HasSuffix(info.Name, "/") {
			if err = os.MkdirAll(outPath, info.Mode()); err != nil {
				return "", fmt.Errorf("error extracting archive: %s: %w", info.Name, err)
			}
		} else {
			if err = os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
				return "", fmt.Errorf("error extracting archive: %s: failed to create parent: %w", info.Name, err)
			}
			file, err := os.OpenFile(outPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
			if err != nil {
				return "", fmt.Errorf("error extracting archive: %s: %w", info.Name, err)
			}
			n, err := io.Copy(file, zipReader)
			if err != nil {
				return "", fmt.Errorf("error extracting archive: %s: %w", info.Name, err)
			}
			if n < int64(info.UncompressedSize64) {
				return "", fmt.Errorf("error extracting archive: %s: extracted %d of %d bytes", info.Name, n, info.UncompressedSize64)
			}
		}
	}

	// Anti-virus might have locked the executable; try to run `--version` until
	// it succeeds before returning.
	for i := 0; i < 60; i++ {
		err = exec.CommandContext(ctx, executablePath, "--version").Run()
		if err == nil {
			break
		}
		time.Sleep(time.Second)
	}

	succeeded = true

	return executablePath, nil
}
