package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Verification struct {
	Method         string `json:"method"`
	KeyFingerprint string `json:"key_fingerprint"`
}

type FirefoxDev struct {
	Name        string       `json:"name"`
	Version     string       `json:"version"`
	InstalledAt string       `json:"installed_at"`
	Verified    Verification `json:"verified"`
}

func createCacheState(stateFile, permURL, fingerprint string) error {
	cacheStateFileDir, err := userCacheDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(cacheStateFileDir, 0o700); err != nil {
		return fmt.Errorf("cannot create cache dir: %w", err)
	}

	_, name, version := parseURL(resolveLatestURL(permURL))
	firefoxDev := FirefoxDev{
		Name:        name,
		Version:     version,
		InstalledAt: "/opt/firefox-developer-edition/",
		Verified: Verification{
			Method:         "gpg",
			KeyFingerprint: fingerprint,
		},
	}
	b, err := json.Marshal(firefoxDev)
	if err != nil {
		return fmt.Errorf("could not parse JSON: %w", err)
	}

	stateFileTemp := filepath.Join(cacheStateFileDir, stateFile)
	if err := os.WriteFile(stateFileTemp, b, 0o644); err != nil {
		return fmt.Errorf("could not write file: %w", err)
	}
	fmt.Println(stateFileTemp)
	return nil
}

func writeState(stateFileDir, stateFileCacheDir string) error {
	if err := os.MkdirAll(stateFileDir, 0o755); err != nil {
		return fmt.Errorf("could not create directory: %w", err)
	}

	if err := copyStateAtomic(stateFileCacheDir, stateFileDir, stateFile); err != nil {
		return err
	}

	return nil
}
