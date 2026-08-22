// Package storage defines the storage-provider abstraction used by the
// file store. Providers: local filesystem, S3-compatible (S3/R2/MinIO/...),
// and Google Drive. A workspace attaches one or more stores; blob_locations
// records which store holds each blob.
package storage

import (
	"context"
	"io"
	"time"
)

// Object is a single stored object returned by List.
type Object struct {
	// Path is the provider object key used for Download/Delete/Exists.
	Path string
	// Name is the human-readable file name when it differs from Path
	// (e.g. GDrive keys are opaque file IDs; Name carries the real name).
	Name         string
	Size         int64
	LastModified time.Time
}

// Storage is the provider-agnostic interface. Providers that cannot support
// an operation (e.g. Google Drive has no true signed URLs) return
// ErrNotSupported so callers can fall back to server-side streaming.
type Storage interface {
	// Upload streams r to path with the given content type.
	Upload(ctx context.Context, path string, r io.Reader, contentType string) error
	// Download returns the object body, its size (-1 if unknown), and error.
	Download(ctx context.Context, path string) (io.ReadCloser, int64, error)
	// GetSignedURL returns a time-limited URL to the object, or ErrNotSupported.
	GetSignedURL(ctx context.Context, path string, expiresIn time.Duration) (string, error)
	// Delete removes the object.
	Delete(ctx context.Context, path string) error
	// Exists reports whether the object exists.
	Exists(ctx context.Context, path string) (bool, error)
	// List returns objects under prefix ("" = all). Providers without
	// prefix listing return ErrNotSupported.
	List(ctx context.Context, prefix string) ([]Object, error)
	// Quota returns used/limit bytes for the backend (0,0 = unknown).
	Quota(ctx context.Context) (used, limit int64, err error)
	// Ping verifies the backend is reachable and the credentials are valid.
	// It is what the stores UI's connection test uses.
	Ping(ctx context.Context) error
}

// ErrNotSupported is returned when a provider cannot perform an operation.
var ErrNotSupported = &UnsupportedError{"operation not supported by storage provider"}

// UnsupportedError carries a message for the not-supported case.
type UnsupportedError struct{ msg string }

func (e *UnsupportedError) Error() string { return e.msg }
