// Package main downloads ollama and runs it.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"
)

const (
	checkURL = "http://localhost:11434/api/tags"
)

var (
	releaseVersion = flag.String("release", "latest", "release to download")
	// Install ollama if not already installed, and return the executable name.
	installOllama func(ctx context.Context, release string) (string, error)
)

func main() {
	ctx := context.Background()
	flag.Parse()

	log.Printf("Checking if %s returns a valid response...", checkURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, nil)
	if err != nil {
		log.Fatalf("Failed to check Ollama: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err == nil && resp.StatusCode < 400 {
		defer resp.Body.Close()
		log.Printf("Ollama seems to be running correctly.")
		return
	}

	executablePath, err := installOllama(ctx, *releaseVersion)
	if err != nil {
		log.Fatalf("Failed to install ollama: %v", err)
	}

	// Do not wait for serveProc to complete.
	serveProc := exec.Command(executablePath, "serve")
	serveProc.Stdout = os.Stdout
	serveProc.Stderr = os.Stderr
	if err = serveProc.Start(); err != nil {
		log.Fatalf("Failed to start ollama server: %v", err)
	}

	log.Printf("Waiting for %s to succeed...", checkURL)
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, nil)
		if err != nil {
			log.Fatalf("Failed to check Ollama: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil && resp.StatusCode < 400 {
			resp.Body.Close()
			break
		}
		time.Sleep(time.Second)
	}

	pullProc := exec.CommandContext(ctx, executablePath, "pull", "tinyllama")
	pullProc.Stdout = os.Stdout
	pullProc.Stderr = os.Stderr
	if err = pullProc.Run(); err != nil {
		log.Fatalf("Failed to pull tinyllama: %v", err)
	}
}

type releaseInfo struct {
	AssetsURL string `json:"assets_url"`
}

type assetInfo struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// getReleaseAssetURL returns the download URL for a specific asset in a release.
func getReleaseAssetURL(ctx context.Context, release, assetName string) (string, error) {
	releaseURL := fmt.Sprintf("https://api.github.com/repos/ollama/ollama/releases/tags/%s", release)
	if release == "latest" {
		releaseURL = "https://api.github.com/repos/ollama/ollama/releases/latest"
	}
	releaseReq, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to find release: %w", err)
	}
	releaseResp, err := http.DefaultClient.Do(releaseReq)
	if err != nil {
		return "", fmt.Errorf("failed to find release: %w", err)
	}
	defer releaseResp.Body.Close()
	if releaseResp.StatusCode >= 300 {
		return "", fmt.Errorf("failed to find release: unexpected status %s", releaseResp.Status)
	}
	releaseBody, err := io.ReadAll(releaseResp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to find release: reading response: %w", err)
	}
	var releaseInfo releaseInfo
	if err = json.Unmarshal(releaseBody, &releaseInfo); err != nil {
		return "", fmt.Errorf("failed to find release: error unmarshaling response: %w", err)
	}

	assetsReq, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseInfo.AssetsURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to find assets: %w", err)
	}
	assetsResp, err := http.DefaultClient.Do(assetsReq)
	if err != nil {
		return "", fmt.Errorf("failed to find assets: %w", err)
	}
	defer assetsResp.Body.Close()
	if assetsResp.StatusCode >= 300 {
		return "", fmt.Errorf("failed to find assets: unexpected status %s", assetsResp.Status)
	}
	assetsBody, err := io.ReadAll(assetsResp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to find assets: reading response: %w", err)
	}
	var assets []assetInfo
	if err = json.Unmarshal(assetsBody, &assets); err != nil {
		return "", fmt.Errorf("failed to find assets: error unmarshaling response: %w", err)
	}

	for _, asset := range assets {
		if asset.Name == assetName {
			return asset.URL, nil
		}
	}

	return "", fmt.Errorf("failed to find asset %q in release %q", assetName, release)
}
