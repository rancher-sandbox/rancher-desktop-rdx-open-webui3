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
	"slices"

	"golang.org/x/sys/unix"
)

const (
	CTL_KERN      = "kern"
	KERN_PROCARGS = 38
)

func init() {
	installOllama = installOllamaDarwin
	uninstallOllama = uninstallOllamaDarwin
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

func uninstallOllamaDarwin(ctx context.Context) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to find home: %w", err)
	}
	executablePath := filepath.Join(home, ".ollama", "ollama")

	if err = terminateProcess(ctx, executablePath); err != nil {
		return fmt.Errorf("error terminating existing ollama process: %w", err)
	}
	err = os.Remove(executablePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return nil
}

func terminateProcess(ctx context.Context, executablePath string) error {
	executableInfo, err := os.Stat(executablePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("failed to get executable info: %w", err)
	}

	procs, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return fmt.Errorf("failed to list processes: %w", err)
	}
	for _, proc := range procs {
		pid := int(proc.Proc.P_pid)
		buf, err := unix.SysctlRaw(CTL_KERN, KERN_PROCARGS, pid)
		if err != nil {
			if !errors.Is(err, unix.EINVAL) {
				log.Printf("Failed to get command line of pid %d: %s", pid, err)
			}
			continue
		}
		// The buffer starts with a null-terminated executable path, plus
		// command line arguments and things.
		index := slices.Index(buf, 0)
		if index < 0 {
			// If we have unexpected data, don't fall over.
			continue
		}
		procPath := string(buf[:index])
		procInfo, err := os.Stat(procPath)
		if err != nil {
			continue
		}
		if os.SameFile(executableInfo, procInfo) {
			process, err := os.FindProcess(pid)
			if err != nil {
				continue
			}
			err = process.Signal(unix.SIGTERM)
			if err == nil {
				log.Printf("Terminated process %d", pid)
			} else if !errors.Is(err, unix.EINVAL) {
				log.Printf("Ignoring failure to terminate pid %d: %s", pid, err)
			}
		}
	}
	return nil
}
