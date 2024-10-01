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
	"unsafe"

	"github.com/xenking/zipstream"
	"golang.org/x/sys/windows"
)

func findExecutable(ctx context.Context) string {
	var potentialLocations []string

	if installLocation, err := getInstallLocation(ctx); err == nil {
		executablePath := filepath.Join(installLocation, "ollama.exe")
		potentialLocations = append(potentialLocations, executablePath)
	}

	programsDir, err := windows.KnownFolderPath(windows.FOLDERID_UserProgramFiles, windows.KF_FLAG_DEFAULT)
	if err == nil {
		// See Ollama setup source code:
		// https://github.com/ollama/ollama/blob/03608cb46ecdccaf8c340c9390626a9d8fcc3c6b/app/ollama.iss#L33
		// https://github.com/ollama/ollama/blob/03608cb46ecdccaf8c340c9390626a9d8fcc3c6b/app/ollama.iss#L92
		potentialLocations = append(potentialLocations, filepath.Join(programsDir, "Ollama", "ollama.exe"))
	}

	for _, location := range potentialLocations {
		if _, err = os.Stat(location); err == nil {
			// Found an existing ollama
			return location
		}
	}
	return ""
}

func installOllama(ctx context.Context, release, installPath string) (string, error) {
	succeeded := false
	executablePath := filepath.Join(installPath, "ollama.exe")

	if _, err := os.Stat(executablePath); err == nil {
		return executablePath, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("failed to check ollama executable: %w", err)
	}

	defer func() {
		if !succeeded {
			// On failure, remove partially extracted files.
			_ = os.RemoveAll(installPath)
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
		outPath := filepath.Join(installPath, info.Name)
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

func uninstallOllama(ctx context.Context) error {
	installDir, err := getInstallLocation(ctx)
	if err != nil {
		return fmt.Errorf("failed to find ollama install: %w", err)
	}
	executablePath := filepath.Join(installDir, "ollama.exe")
	if err = terminateProcess(ctx, executablePath); err != nil {
		return fmt.Errorf("error terminating existing ollama process: %w", err)
	}

	err = os.RemoveAll(installDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return nil
}

// terminateProcess terminates the ollama process; this is required because on
// Windows running processes cannot be deleted.
func terminateProcess(ctx context.Context, executablePath string) error {

	ollamaInfo, err := os.Stat(executablePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("error examining ollama executable: %w", err)
	}

	pids := make([]uint32, 4096)
	// Try EnumProcesses until the number of pids returned is less than the
	// buffer size.
	for {
		var bytesReturned uint32
		err := windows.EnumProcesses(pids, &bytesReturned)
		if err != nil || len(pids) < 1 {
			return fmt.Errorf("failed to enumerate processes: %w", err)
		}
		pidsReturned := uintptr(bytesReturned) / unsafe.Sizeof(pids[0])
		if pidsReturned < uintptr(len(pids)) {
			// Remember to truncate the pids to only the valid set.
			pids = pids[:pidsReturned]
			break
		}
		pids = make([]uint32, len(pids)*2)
	}

	for _, pid := range pids {
		// Do each iteration in a function so defer statements run faster.
		err := (func() error {
			hProc, err := windows.OpenProcess(
				windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_TERMINATE,
				false,
				pid)
			if err != nil {
				log.Printf("Ignoring error opening process %d: %s", pid, err)
				return nil
			}
			defer windows.CloseHandle(hProc)

			nameBuf := make([]uint16, 1024)
			for {
				bufSize := uint32(len(nameBuf))
				err = windows.QueryFullProcessImageName(hProc, 0, &nameBuf[0], &bufSize)
				if err != nil {
					return fmt.Errorf("error getting process %d executable: %w", pid, err)
				}
				if int(bufSize) < len(nameBuf) {
					break
				}
				nameBuf = make([]uint16, len(nameBuf)*2)
			}
			executablePath := windows.UTF16ToString(nameBuf)
			executableInfo, err := os.Stat(executablePath)
			if err != nil {
				return nil
			}
			if os.SameFile(ollamaInfo, executableInfo) {
				if err = windows.TerminateProcess(hProc, 0); err != nil {
					return fmt.Errorf("failed to terminate pid %d (%s): %w", pid, executablePath, err)
				}
			}

			return nil
		})()
		if err != nil {
			log.Printf("%s", err)
		}
	}

	return nil
}
