// Package main downloads ollama and runs it.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	checkURL = "http://localhost:11434/api/tags"
)

type Mode string

const (
	ModeInstall   Mode = "install"   // Install ollama to the given location.
	ModeUninstall Mode = "uninstall" // Uninstall ollama that we have installed.
	ModeLocate    Mode = "locate"    // Locate the install; if not found, print the default install location.
)

var (
	mode           = ModeInstall
	allModes       = []Mode{ModeInstall, ModeUninstall, ModeLocate}
	releaseVersion = flag.String("release", "latest", "release to download when installing")
	installPath    = flag.String("install-path", "", "directory to install ollama to")
	pullModel      = flag.String("model", "tinyllama", "model to pull on install; set to empty string to skip")
)

func main() {
	ctx := context.Background()
	log.SetFlags(log.LUTC | log.Ldate | log.Ltime)
	flag.Func("mode", fmt.Sprintf("operation mode; one of %+v (default %q)", allModes, mode), func(s string) error {
		switch s {
		case string(ModeInstall):
			mode = ModeInstall
		case string(ModeUninstall):
			mode = ModeUninstall
		case string(ModeLocate):
			mode = ModeLocate
		default:
			return fmt.Errorf("unexpected mode %s: should be one of %+v", s, allModes)
		}
		return nil
	})
	flag.Parse()

	switch mode {
	case ModeInstall:
		log.Printf("Installing ollama...")
		if err := install(ctx); err != nil {
			log.Fatal(err)
		}
	case ModeUninstall:
		log.Printf("Uninstalling ollama...")
		if err := uninstallOllama(ctx); err != nil {
			log.Fatal(err)
		}
	case ModeLocate:
		if err := printLocation(ctx); err != nil {
			log.Fatal(err)
		}
	}
}

func install(ctx context.Context) error {
	log.Printf("Checking if %s returns a valid response...", checkURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, nil)
	if err != nil {
		return fmt.Errorf("failed to check Ollama: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err == nil && resp.StatusCode < 400 {
		defer resp.Body.Close()
		log.Printf("Ollama seems to be running correctly.")
		return nil
	}

	executablePath := findExecutable(ctx)
	if executablePath == "" {
		if *installPath == "" {
			// If install path is not given, use the default install location.
			*installPath, err = getInstallLocation(ctx)
			if err != nil {
				return fmt.Errorf("failed to get default install location: %w", err)
			}
		}

		if err = saveInstallLocation(ctx, *installPath); err != nil {
			return fmt.Errorf("failed to save install location: %w", err)
		}
		executablePath, err = installOllama(ctx, *releaseVersion, *installPath)
		if err != nil {
			// If we fail to install, clear the state; don't catch errors here.
			_ = saveInstallLocation(ctx, "")
			return fmt.Errorf("failed to install ollama: %w", err)
		}
	}

	// Do not wait for serveProc to complete.
	serveProc := exec.Command(executablePath, "serve")
	serveProc.Stdout = os.Stdout
	serveProc.Stderr = os.Stderr
	if err = serveProc.Start(); err != nil {
		return fmt.Errorf("failed to start ollama server: %v", err)
	}

	log.Printf("Waiting for %s to succeed...", checkURL)
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, nil)
		if err != nil {
			return fmt.Errorf("failed to check Ollama: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil && resp.StatusCode < 400 {
			resp.Body.Close()
			break
		}
		time.Sleep(time.Second)
	}

	if *pullModel != "" {
		pullProc := exec.CommandContext(ctx, executablePath, "pull", *pullModel)
		pullProc.Stdout = os.Stdout
		pullProc.Stderr = os.Stderr
		if err = pullProc.Run(); err != nil {
			return fmt.Errorf("failed to pull %s: %v", *pullModel, err)
		}
	}

	return nil
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

// Get the path of the file that stores the install location.
func getInstallLocationFilePath(ctx context.Context) (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to find executable path: %w", err)
	}
	extensionDir := filepath.Dir(filepath.Dir(executable))
	return filepath.Join(extensionDir, "install-location.txt"), nil
}

// Get the location ollama was installed in, or the default install location if
// it is not currently installed.  Note that this does not return the ollama
// location if it was installed externally.
func getInstallLocation(ctx context.Context) (string, error) {
	locationPath, err := getInstallLocationFilePath(ctx)
	if err != nil {
		return "", err
	}
	previousLocation, err := os.ReadFile(locationPath)
	if err == nil {
		if _, err = os.Stat(string(previousLocation)); err == nil {
			return string(previousLocation), nil
		}
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("failed to locate previous installation: %w", err)
	}
	// No stored location; return "ollama" next to the location file.
	return filepath.Join(filepath.Dir(locationPath), "ollama"), nil
}

func saveInstallLocation(ctx context.Context, location string) error {
	locationPath, err := getInstallLocationFilePath(ctx)
	if err != nil {
		return err
	}
	if err = os.WriteFile(locationPath, []byte(location), 0o644); err != nil {
		return fmt.Errorf("failed to write install location: %w", err)
	}
	return nil
}

// Print the current install location, or the default install location if there
// is no current install.
func printLocation(ctx context.Context) error {
	location, err := getInstallLocation(ctx)
	if err != nil {
		return err
	}
	log.SetFlags(0)
	log.Printf("%s", location)
	return nil
}
