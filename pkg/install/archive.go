package install

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cavaliergopher/grab/v3"
)

// FetchExtract downloads url to cacheDir, optionally verifies the SHA256
// checksum, extracts the archive (tar.gz, tar.xz, or zip) into a staging
// directory, and atomically renames it to destDir.
//
// Progress is written to stderr.
func FetchExtract(ctx context.Context, url, cacheDir, destDir, sha256sum string) error {
	// 1. Download
	archivePath, err := download(ctx, url, cacheDir)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}

	// 2. Verify
	if sha256sum != "" {
		if err := verifySHA256(archivePath, sha256sum); err != nil {
			return fmt.Errorf("checksum: %w", err)
		}
	}

	// 3. Extract to staging
	staging := destDir + ".staging"
	os.RemoveAll(staging) // clean up any previous attempt
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return err
	}
	if err := extract(archivePath, staging); err != nil {
		os.RemoveAll(staging)
		return fmt.Errorf("extract: %w", err)
	}

	// 4. Atomic rename
	if err := os.Rename(staging, destDir); err != nil {
		os.RemoveAll(staging)
		return fmt.Errorf("install: %w", err)
	}

	return nil
}

// download fetches url into cacheDir using grab.
func download(ctx context.Context, url, cacheDir string) (string, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", err
	}

	req, err := grab.NewRequest(cacheDir, url)
	if err != nil {
		return "", err
	}
	req = req.WithContext(ctx)

	resp := grab.DefaultClient.Do(req)
	fmt.Fprintf(os.Stderr, "Downloading %s\n", url)

	// Wait for completion.
	if err := resp.Err(); err != nil {
		return "", err
	}

	fmt.Fprintf(os.Stderr, "Downloaded %s (%d bytes)\n", resp.Filename, resp.Size())
	return resp.Filename, nil
}

// verifySHA256 checks the file matches the expected hex-encoded SHA256.
func verifySHA256(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != expected {
		return fmt.Errorf("sha256 mismatch: got %s, want %s", got, expected)
	}
	return nil
}

// extract extracts a tar.gz, tar.xz, or zip archive into dest.
func extract(archivePath, dest string) error {
	switch {
	case strings.HasSuffix(archivePath, ".zip"):
		return extractZip(archivePath, dest)
	default:
		// .tar.gz, .tar.xz (stdlib handles gzip; xz via external or no compression)
		return extractTar(archivePath, dest)
	}
}

func extractTar(archivePath, dest string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	var r io.Reader = f

	// Handle gzip if the filename or content suggests it.
	if strings.HasSuffix(archivePath, ".gz") || strings.HasSuffix(archivePath, ".tgz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer gz.Close()
		r = gz
	}

	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Strip the top-level directory name (e.g. "node-v22.14.0-linux-x64/")
		rel := stripTopDir(hdr.Name)

		target := filepath.Join(dest, rel)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		case tar.TypeSymlink:
			os.Symlink(hdr.Linkname, target) // best-effort
		}
	}
	return nil
}

func extractZip(archivePath, dest string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		rel := stripTopDir(f.Name)
		target := filepath.Join(dest, rel)

		if f.FileInfo().IsDir() {
			os.MkdirAll(target, 0o755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}

		in, err := f.Open()
		if err != nil {
			return err
		}

		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			in.Close()
			return err
		}

		_, err = io.Copy(out, in)
		in.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// isValidSHA256 returns true if s is a 64-character lowercase hex string.
func isValidSHA256(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// stripTopDir strips the first path component. e.g. "node-v22.14.0/bin/node" → "bin/node".
func stripTopDir(p string) string {
	parts := strings.SplitN(p, string(filepath.Separator), 2)
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}
