package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
)

func fetch() error {
	sig, err := getVerification(resolveLatestURL(permURL))
	if err != nil {
		return fmt.Errorf("could not fetch verification %w", err)
	}

	tarball, err := getBinary(resolveLatestURL(permURL))
	if err != nil {
		return fmt.Errorf("could not fetch binary: %w", err)
	}

	fingerprint, err := verify(tarball, sig)
	if err != nil {
		return fmt.Errorf("bad downloads: %w", err)
	}

	if err := createCacheState(stateFile, permURL, fingerprint); err != nil {
		return fmt.Errorf("error creating state file: %w", err)
	}

	return nil
}

func getVerification(url *url.URL) (string, error) {
	ascURL := url.String() + ".asc"
	filename, _, _ := parseURL(url)
	cachedir, err := userCacheDir()
	if err != nil {
		return "", err
	}
	if err := ensureDir(cachedir, 0o700); err != nil {
		return "", err
	}

	filename = filepath.Join(cachedir, filename)
	filename = filename + ".asc"
	if err := downloadFile(filename, ascURL); err != nil {
		return filename, fmt.Errorf("getting verification failed: %w", err)
	}
	return filename, nil
}

func getBinary(url *url.URL) (string, error) {
	filename, _, _ := parseURL(url)
	cachedir, err := userCacheDir()
	if err != nil {
		return "", err
	}
	if err := ensureDir(cachedir, 0o700); err != nil {
		return "", err
	}

	filename = filepath.Join(cachedir, filename)
	if err := downloadFile(filename, url.String()); err != nil {
		return filename, fmt.Errorf("getting binary failed: %w", err)
	}
	return filename, nil
}

func downloadFile(filepath, url string) error {
	outfile, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("could not create file: %w", err)
	}

	resp, err := http.Get(url)
	if err != nil {
		log.Println(err)
		outfile.Close()
		return err
	}

	_, err = io.Copy(outfile, resp.Body)
	if err != nil {
		outfile.Close()
		resp.Body.Close()
		return err
	}

	if err := outfile.Close(); err != nil {
		return err
	}

	if err := resp.Body.Close(); err != nil {
		return err
	}
	return nil
}

func resolveLatestURL(permURL string) *url.URL {
	resp, err := http.Get(permURL)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	return resp.Request.URL
}

func parseURL(url *url.URL) (filename, name, version string) {
	re := regexp.MustCompile(`(firefox(?:-developer)?)-([0-9a-zA-Z.\-]+)\.tar\.xz$`)
	matches := re.FindStringSubmatch(url.String())
	if len(matches) < 3 {
		return "", "", ""
	}

	return matches[0], matches[1], matches[2]
}
