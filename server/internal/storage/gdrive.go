package storage

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"go-drive/server/internal/drive"
)

// GDrive adapts drive.Store (one Google account) to the Storage interface.
// Drive has no true signed URLs and no prefix listing — GetSignedURL returns
// the file's webContentLink when available, and List lists a folder's
// children via the parent folder ID (the object key is the Drive file ID).
type GDrive struct {
	store *drive.Store
}

// NewGDrive builds a Drive-backed Storage for one account.
func NewGDrive(clientID, clientSecret, refreshToken, folderID string) (*GDrive, error) {
	if clientID == "" || clientSecret == "" || refreshToken == "" {
		return nil, fmt.Errorf("gdrive: client id, client secret, and refresh token are required")
	}
	return &GDrive{store: drive.NewStore(clientID, clientSecret, refreshToken, folderID)}, nil
}

// Upload streams r into the store's root folder. The path's basename is used
// as the Drive file name (Drive has no true folder hierarchy in the
// object-key sense).
func (g *GDrive) Upload(ctx context.Context, path string, r io.Reader, contentType string) error {
	name := path
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		name = path[idx+1:]
	}
	if _, err := g.store.Upload(ctx, name, r, contentType); err != nil {
		return err
	}
	return nil
}

// Download streams the file identified by the Drive file ID in path.
func (g *GDrive) Download(ctx context.Context, path string) (io.ReadCloser, int64, error) {
	return g.store.Download(ctx, path)
}

// GetSignedURL returns the Drive webContentLink for the file, or
// ErrNotSupported when the file isn't link-shared.
func (g *GDrive) GetSignedURL(ctx context.Context, path string, expiresIn time.Duration) (string, error) {
	svc, err := g.store.Service(ctx)
	if err != nil {
		return "", err
	}
	f, err := svc.Files.Get(path).Fields("webContentLink").Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("gdrive: get web content link: %w", err)
	}
	if f.WebContentLink == "" {
		return "", ErrNotSupported
	}
	return f.WebContentLink, nil
}

// Delete removes the file from Drive.
func (g *GDrive) Delete(ctx context.Context, path string) error {
	return g.store.Delete(ctx, path)
}

// Exists reports whether the Drive file exists.
func (g *GDrive) Exists(ctx context.Context, path string) (bool, error) {
	return g.store.Exists(ctx, path)
}

// List returns the children of the folder whose ID is path ("" = root).
func (g *GDrive) List(ctx context.Context, prefix string) ([]Object, error) {
	parentID := prefix
	if parentID == "" {
		parentID = g.store.RootFolderID()
	}
	files, err := g.store.List(ctx, parentID)
	if err != nil {
		return nil, err
	}
	out := make([]Object, 0, len(files))
	for _, f := range files {
		size := int64(0)
		if f.Size != 0 {
			size = f.Size
		}
		out = append(out, Object{Path: f.Id, Name: f.Name, Size: size})
	}
	return out, nil
}

// Quota returns the account's Drive usage/limit.
func (g *GDrive) Quota(ctx context.Context) (int64, int64, error) {
	q, err := g.store.Quota(ctx)
	if err != nil {
		return 0, 0, err
	}
	return q.UsageInDrive, q.Limit, nil
}
