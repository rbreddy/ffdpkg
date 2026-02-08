package main

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ulikunitz/xz"
)

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

	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
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
