package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Local is a filesystem-backed Storage provider.
type Local struct {
	baseDir   string
	signKey   []byte
	publicURL string
}

// NewLocal creates a Local provider rooted at baseDir. signKey is the
// HMAC key used for signed serve URLs (from SECRETS_ENCRYPTION_KEY /
// AUTHULA_SECRET). publicURL is optional and prepended to signed URLs.
func NewLocal(baseDir string, signKey []byte, publicURL string) (*Local, error) {
	abs, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("local: resolving base dir: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("local: creating base dir: %w", err)
	}
	return &Local{baseDir: abs, signKey: signKey, publicURL: strings.TrimRight(publicURL, "/")}, nil
}

// resolve joins path under baseDir and guards against traversal.
// The boundary check uses a path-separator suffix so sibling prefixes
// (e.g. C:\data\blobs2) cannot satisfy it on Windows.
func (l *Local) resolve(path string) (string, error) {
	clean := filepath.Clean("/" + path)
	full := filepath.Join(l.baseDir, clean)
	if full != l.baseDir && !strings.HasPrefix(full, l.baseDir+string(os.PathSeparator)) {
		return "", fmt.Errorf("local: path escapes base dir: %s", path)
	}
	return full, nil
}

// Upload writes r to path, creating parent directories as needed.
func (l *Local) Upload(ctx context.Context, path string, r io.Reader, contentType string) error {
	full, err := l.resolve(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("local: creating dirs: %w", err)
	}
	tmp := full + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("local: creating file: %w", err)
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("local: writing file: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("local: closing file: %w", err)
	}
	if err := os.Rename(tmp, full); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("local: renaming file: %w", err)
	}
	return nil
}

// Download opens the file at path and returns its size.
func (l *Local) Download(ctx context.Context, path string) (io.ReadCloser, int64, error) {
	full, err := l.resolve(path)
	if err != nil {
		return nil, 0, err
	}
	f, err := os.Open(full)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, fmt.Errorf("local: %w", os.ErrNotExist)
		}
		return nil, 0, fmt.Errorf("local: opening file: %w", err)
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, fmt.Errorf("local: stat file: %w", err)
	}
	return f, st.Size(), nil
}

// GetSignedURL returns an HMAC-signed serve URL: {publicURL}/api/files/serve{path}?exp=&sig=.
// The path is normalized to a leading-slash form — the same string Gin
// exposes via the *path param — so signer and verifier always agree.
func (l *Local) GetSignedURL(ctx context.Context, path string, expiresIn time.Duration) (string, error) {
	if len(l.signKey) == 0 {
		return "", ErrNotSupported
	}
	path = withLeadingSlash(path)
	exp := strconv.FormatInt(time.Now().Add(expiresIn).Unix(), 10)
	mac := hmac.New(sha256.New, l.signKey)
	mac.Write([]byte(path + ":" + exp))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s/api/files/serve%s?exp=%s&sig=%s", l.publicURL, path, exp, sig), nil
}

// VerifySignature checks an HMAC signature for path at exp.
func (l *Local) VerifySignature(path, exp, sig string) bool {
	if len(l.signKey) == 0 {
		return false
	}
	path = withLeadingSlash(path)
	mac := hmac.New(sha256.New, l.signKey)
	mac.Write([]byte(path + ":" + exp))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sig))
}

// withLeadingSlash normalizes an object key (docs/report.pdf) to the URL-path
// form (/docs/report.pdf) covered by the HMAC.
func withLeadingSlash(path string) string {
	if strings.HasPrefix(path, "/") {
		return path
	}
	return "/" + path
}

// Delete removes the file at path.
func (l *Local) Delete(ctx context.Context, path string) error {
	full, err := l.resolve(path)
	if err != nil {
		return err
	}
	if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("local: deleting file: %w", err)
	}
	return nil
}

// Exists reports whether the file exists.
func (l *Local) Exists(ctx context.Context, path string) (bool, error) {
	full, err := l.resolve(path)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(full)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("local: stat file: %w", err)
	}
	return true, nil
}

// List walks the tree under prefix and returns object metadata.
func (l *Local) List(ctx context.Context, prefix string) ([]Object, error) {
	root, err := l.resolve(prefix)
	if err != nil {
		return nil, err
	}
	var out []Object
	err = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(l.baseDir, p)
		if err != nil {
			return err
		}
		out = append(out, Object{
			Path:         filepath.ToSlash(rel),
			Size:         info.Size(),
			LastModified: info.ModTime(),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("local: walking tree: %w", err)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// Quota returns the total bytes used under baseDir; limit is unknown (0).
func (l *Local) Quota(ctx context.Context) (int64, int64, error) {
	var used int64
	objs, err := l.List(ctx, "")
	if err != nil {
		return 0, 0, err
	}
	for _, o := range objs {
		used += o.Size
	}
	return used, 0, nil
}

// SignKey exposes the HMAC key for the serve handler (read-only copy).
func (l *Local) SignKey() []byte {
	return append([]byte(nil), l.signKey...)
}

// BaseDir exposes the resolved root (used by the serve handler).
func (l *Local) BaseDir() string { return l.baseDir }

// hexEncode helper kept for potential checksum use.
func hexEncode(b []byte) string { return hex.EncodeToString(b) }
