package drive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	gdrive "google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

// isNotFound reports whether err is a Google API 404 (file missing).
func isNotFound(err error) bool {
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) {
		return apiErr.Code == 404
	}
	return false
}

// Store is a per-account Google Drive storage backend. Unlike the
// package-level API (single global folder), each Store carries its own OAuth
// credentials and root folder, enabling multi-account storage pools where
// each Drive account is one Store instance.
type Store struct {
	mu     sync.RWMutex
	cfg    Config
	svc    *gdrive.Service
	svcErr error
}

// NewStore creates a Drive backend for one account. clientID/clientSecret
// come from the shared OAuth app; refreshToken + folderID are per-account.
func NewStore(clientID, clientSecret, refreshToken, folderID string) *Store {
	return &Store{cfg: Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RefreshToken: refreshToken,
		FolderID:     folderID,
	}}
}

// client builds (lazily) the gdrive.Service for this store's credentials.
func (s *Store) client(ctx context.Context) (*gdrive.Service, error) {
	s.mu.RLock()
	if s.svc != nil || s.svcErr != nil {
		c, err := s.svc, s.svcErr
		s.mu.RUnlock()
		return c, err
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.svc != nil || s.svcErr != nil {
		return s.svc, s.svcErr
	}

	cfg := s.cfg
	if cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.RefreshToken == "" {
		s.svcErr = fmt.Errorf("google drive not configured for this store")
		return nil, s.svcErr
	}

	oauthCfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{gdrive.DriveFileScope, gdrive.DriveReadonlyScope},
	}
	tok := &oauth2.Token{RefreshToken: cfg.RefreshToken}
	httpClient := oauthCfg.Client(ctx, tok)
	svc, err := gdrive.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		s.svcErr = fmt.Errorf("creating drive service: %w", err)
		return nil, s.svcErr
	}
	s.svc = svc
	return svc, nil
}

// Upload streams r into the store's root folder with the given display name.
func (s *Store) Upload(ctx context.Context, displayName string, r io.Reader, contentType string) (*Result, error) {
	svc, err := s.client(ctx)
	if err != nil {
		return nil, err
	}
	f := &gdrive.File{
		Name:    displayName,
		Parents: []string{s.cfg.FolderID},
	}
	created, err := svc.Files.Create(f).Media(r).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("drive upload: %w", err)
	}
	return &Result{FileID: created.Id, Link: fmt.Sprintf("https://drive.google.com/file/d/%s/view", created.Id)}, nil
}

// Download streams the file content for the given Drive file ID.
func (s *Store) Download(ctx context.Context, fileID string) (io.ReadCloser, int64, error) {
	svc, err := s.client(ctx)
	if err != nil {
		return nil, 0, err
	}
	resp, err := svc.Files.Get(fileID).Download()
	if err != nil {
		return nil, 0, fmt.Errorf("drive download: %w", err)
	}
	return resp.Body, resp.ContentLength, nil
}

// Delete removes the file from Drive.
func (s *Store) Delete(ctx context.Context, fileID string) error {
	svc, err := s.client(ctx)
	if err != nil {
		return err
	}
	if err := svc.Files.Delete(fileID).Context(ctx).Do(); err != nil {
		return fmt.Errorf("drive delete: %w", err)
	}
	return nil
}

// Exists reports whether the Drive file exists.
func (s *Store) Exists(ctx context.Context, fileID string) (bool, error) {
	svc, err := s.client(ctx)
	if err != nil {
		return false, err
	}
	_, err = svc.Files.Get(fileID).Fields("id").Context(ctx).Do()
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("drive exists: %w", err)
	}
	return true, nil
}

// List returns the Drive files directly inside the given parent folder.
func (s *Store) List(ctx context.Context, parentID string) ([]gdrive.File, error) {
	svc, err := s.client(ctx)
	if err != nil {
		return nil, err
	}
	var out []gdrive.File
	pageToken := ""
	for {
		call := svc.Files.List().
			Q(fmt.Sprintf("'%s' in parents and trashed=false", parentID)).
			Fields("nextPageToken, files(id, name, size, mimeType, createdTime)").
			PageSize(1000)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		list, err := call.Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("drive list: %w", err)
		}
		for _, f := range list.Files {
			out = append(out, *f)
		}
		if list.NextPageToken == "" {
			break
		}
		pageToken = list.NextPageToken
	}
	return out, nil
}

// Service returns the lazily-built Drive service (used by the storage
// adapter for webContentLink lookups).
func (s *Store) Service(ctx context.Context) (*gdrive.Service, error) {
	return s.client(ctx)
}

// RootFolderID returns the configured root folder for this store.
func (s *Store) RootFolderID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.FolderID
}

// Quota returns the account's storage usage via About.storageQuota — used by
// pool placement to pick the account with the most free space.
func (s *Store) Quota(ctx context.Context) (*gdrive.AboutStorageQuota, error) {
	svc, err := s.client(ctx)
	if err != nil {
		return nil, err
	}
	about, err := svc.About.Get().Fields("storageQuota").Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("drive quota: %w", err)
	}
	return about.StorageQuota, nil
}
