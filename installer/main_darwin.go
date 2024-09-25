package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func init() {
	installOllama = installOllamaDarwin
}

func installOllamaDarwin(ctx context.Context, release string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to find home: %w", err)
	}
	executablePath := filepath.Join(home, ".ollama", "ollama")

	if _, err := os.Stat(executablePath); err == nil {
		return executablePath, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("failed to check ollama executable: %w", err)
	}

	assetURL, err := getReleaseAssetURL(ctx, release, "ollama-darwin")
	if err != nil {
		return "", err
	}

	log.Printf("Downloading ollama from %s...", assetURL)

	// For darwin, Ollama is a single executable.
	if err = os.MkdirAll(filepath.Dir(executablePath), 0o755); err != nil {
		return "", fmt.Errorf("failed to create ollama directory: %w", err)
	}
	file, err := os.OpenFile(executablePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return "", fmt.Errorf("failed to create executable: %w", err)
	}
	defer file.Close()
	succeeded := false
	defer func() {
		if !succeeded {
			os.Remove(executablePath)
		}
	}()

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
	length, err := io.Copy(file, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to write ollama: %w", err)
	}
	if resp.ContentLength > 0 {
		if length < resp.ContentLength {
			return "", fmt.Errorf("partial read downloading ollama")
		}
	}
	if err = file.Chmod(0o755); err != nil {
		return "", fmt.Errorf("failed to change ollama file mode: %w", err)
	}
	succeeded = true

	return executablePath, nil
}
