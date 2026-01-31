package main

import (
	"archive/tar"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/ulikunitz/xz"
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

const (
	permURL      = "https://download.mozilla.org/?product=firefox-devedition-latest&os=linux64&lang=en-US"
	stateFile    = "state.json"
	stateFileDir = "/var/lib/ffpkg/"
	installDir   = "/opt/firefox-developer-edition/"
)

func main() {
	cmd := flag.String("cmd", "", "one of fetch, verify, install")
	cacheDir := flag.String("cache", "", "path to user cache directory")
	tarball := flag.String("tarball", "", "path to tarball")
	sig := flag.String("sig", "", "path to signature")
	flag.Parse()

	var err error

	switch *cmd {
	case "fetch":
		if err = fetch(); err != nil {
			log.Fatal("could not fetch: %w", err)
		}

	case "verify":
		if *tarball == "" || *sig == "" {
			log.Fatal("must provide --tarball and --sig for verify")
		}
		_, err = verify(*tarball, *sig)
		if err != nil {
			log.Fatal("verification failed: %w", err)
		}

	case "install":
		if *cacheDir == "" {
			log.Fatal("install requires --cache")
		}
		if err := install(*cacheDir); err != nil {
			log.Fatal("could not install: %w", err)
		}

	default:
		log.Fatal("must specify --cmd=fetch|verify|install")
	}
}

func resolveLatestURL(permURL string) *url.URL {
	resp, err := http.Get(permURL)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	return resp.Request.URL
}

// TODO: update func to create or update state
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
	if err := os.WriteFile(stateFileTemp, b, 0644); err != nil {
		return fmt.Errorf("could not write file: %w", err)
	}
	fmt.Println(stateFileTemp)
	return nil
}

func parseURL(url *url.URL) (filename, name, version string) {
	re := regexp.MustCompile(`(firefox(?:-developer)?)-([0-9a-zA-Z.\-]+)\.tar\.xz$`)
	matches := re.FindStringSubmatch(url.String())
	if len(matches) < 3 {
		return "", "", ""
	}

	return matches[0], matches[1], matches[2]
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

func verify(filepath, signaturepath string) (string, error) {
	fmt.Println(filepath, signaturepath)
	cmd := exec.Command("gpg", "--verify", signaturepath, filepath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("signature check failed: %w", err)
	}

	re := regexp.MustCompile(`using [A-Z]+ key ([0-9A-F]+)`)
	matches := re.FindStringSubmatch(string(out))
	if len(matches) < 2 {
		return "", fmt.Errorf("cannot parse key fingerprint from gpg output")
	}
	fingerprint := matches[1]

	return fingerprint, nil
}

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

func install(cachedir string) error {
	if err := writeState(stateFileDir, cachedir); err != nil {
		return fmt.Errorf("could not write state %w", err)
	}
	fmt.Println("wrote state")

	tarball, _, _ := parseURL(resolveLatestURL(permURL))
	tarball = filepath.Join(cachedir, tarball)
	fmt.Println("the tarball file path", tarball)
	if err := extractTarXzAtomic(tarball, installDir); err != nil {
		return fmt.Errorf("error extracting: %w", err)
	}

	return nil
}

func writeState(stateFileDir, stateFileCacheDir string) error {
	if err := os.MkdirAll(stateFileDir, 0755); err != nil {
		return fmt.Errorf("could not create directory: %w", err)
	}

	if err := copyStateAtomic(stateFileCacheDir, stateFileDir, stateFile); err != nil {
		return err
	}

	return nil
}

func extractTarXzAtomic(tarPath, finalDir string) error {
	parent := filepath.Dir(finalDir)
	if err := ensureDir(parent, 0o755); err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("/opt", ".ffpkg-*")
	if err != nil {
		return fmt.Errorf("could not create temp dir %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := extractTarXz(tarPath, tmpDir); err != nil {
		return err
	}

	// Remove old install if it exists
	if err := os.RemoveAll(finalDir); err != nil {
		return err
	}

	// Atomic replace
	if err := os.Rename(tmpDir, finalDir); err != nil {
		return fmt.Errorf("atomic rename failed: %w", err)
	}

	return nil
}

func extractTarXz(tarXzPath, destDir string) error {
	f, err := os.Open(tarXzPath)
	if err != nil {
		return err
	}
	defer f.Close()

	xzr, err := xz.NewReader(f)
	if err != nil {
		return err
	}

	tr := tar.NewReader(xzr)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, hdr.Name)

		switch hdr.Typeflag {

		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
				return err
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}

			out, err := os.OpenFile(
				target,
				os.O_CREATE|os.O_TRUNC|os.O_WRONLY,
				os.FileMode(hdr.Mode),
			)
			if err != nil {
				return err
			}

			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()

		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(hdr.Linkname, target); err != nil && !os.IsExist(err) {
				return err
			}

		default:
			// ignore devices, etc.
		}
	}

	return nil
}

func userCacheDir() (string, error) {
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "ffpkg"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache/ffpkg"), nil
}

func copyStateAtomic(srcDir, dstDir, filename string) error {
	src := filepath.Join(srcDir, filename)
	tmp := filepath.Join(dstDir, filename+".tmp")
	dst := filepath.Join(dstDir, filename)

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}

	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}

	if err := out.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmp, dst); err != nil {
		return err
	}

	dir, err := os.Open(dstDir)
	if err != nil {
		return err
	}
	defer dir.Close()

	if err := dir.Sync(); err != nil {
		return err
	}

	return nil
}

func ensureDir(path string, mode os.FileMode) error {
	return os.MkdirAll(path, mode)
}
