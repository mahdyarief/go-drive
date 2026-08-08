package drive

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	gdrive "google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// Config holds the OAuth user-flow credentials used to talk to Google Drive.
type Config struct {
	ClientID     string
	ClientSecret string
	RefreshToken string
	FolderID     string
}

// Result berisi identitas file setelah upload sukses.
type Result struct {
	FileID string
	Link   string // https://drive.google.com/file/d/{id}/view
}

var (
	configMu sync.RWMutex
	cfg      = envConfig()

	clientOnce sync.Once
	client     *gdrive.Service
	clientErr  error
)

// envConfig seeds the initial config from environment variables so the
// existing .env-based setup keeps working until the admin overrides it in UI.
func envConfig() Config {
	return Config{
		ClientID:     os.Getenv("GOOGLE_DRIVE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_DRIVE_CLIENT_SECRET"),
		RefreshToken: os.Getenv("GOOGLE_DRIVE_REFRESH_TOKEN"),
		FolderID:     os.Getenv("GOOGLE_DRIVE_FOLDER_ID"),
	}
}

// SetConfig replaces the drive credentials and forces the next getClient call
// to rebuild the gdrive.Service (the oauth2 lib refreshes access tokens itself).
func SetConfig(c Config) {
	configMu.Lock()
	cfg = c
	configMu.Unlock()

	clientOnce = sync.Once{}
	client = nil
	clientErr = nil
}

// GetConfig returns the currently active drive config.
func GetConfig() Config {
	configMu.RLock()
	defer configMu.RUnlock()
	return cfg
}

// getClient builds the Drive service lazily from OAuth user-flow credentials
// (refresh token obtained once via cmd/gdrive-auth or the admin Settings UI).
// The client is shared across requests; the oauth2 lib silently refreshes the
// access token.
func getClient() (*gdrive.Service, error) {
	clientOnce.Do(func() {
		c := GetConfig()
		if c.ClientID == "" || c.ClientSecret == "" || c.RefreshToken == "" || c.FolderID == "" {
			clientErr = fmt.Errorf("google drive not configured (set GOOGLE_DRIVE_CLIENT_ID, GOOGLE_DRIVE_CLIENT_SECRET, GOOGLE_DRIVE_REFRESH_TOKEN, and GOOGLE_DRIVE_FOLDER_ID)")
			log.Printf("[drive] %v", clientErr)
			return
		}
		conf := &oauth2.Config{
			ClientID:     c.ClientID,
			ClientSecret: c.ClientSecret,
			Endpoint:     google.Endpoint,
			Scopes:       []string{gdrive.DriveFileScope, gdrive.DriveReadonlyScope},
		}
		ts := conf.TokenSource(context.Background(), &oauth2.Token{RefreshToken: c.RefreshToken})
		srv, err := gdrive.NewService(context.Background(), option.WithTokenSource(ts))
		if err != nil {
			clientErr = fmt.Errorf("creating google drive client: %w", err)
			log.Printf("[drive] %v", clientErr)
			return
		}
		client = srv
	})
	return client, clientErr
}

// UploadProof uploads a file to the configured Drive folder, sets it readable
// by anyone with the link, and returns its ID + shareable link.
func UploadProof(ctx context.Context, r io.Reader, name string) (*Result, error) {
	srv, err := getClient()
	if err != nil {
		return nil, err
	}
	file, err := srv.Files.Create(&gdrive.File{
		Name:    name,
		Parents: []string{GetConfig().FolderID},
	}).Media(r).Do()
	if err != nil {
		err = fmt.Errorf("uploading file to google drive: %w", err)
		log.Printf("[drive] %v", err)
		return nil, err
	}
	if _, err := srv.Permissions.Create(file.Id, &gdrive.Permission{
		Role: "reader",
		Type: "anyone",
	}).Do(); err != nil {
		err = fmt.Errorf("setting google drive file permission: %w", err)
		log.Printf("[drive] %v", err)
		return nil, err
	}
	return &Result{
		FileID: file.Id,
		Link:   fmt.Sprintf("https://drive.google.com/file/d/%s/view", file.Id),
	}, nil
}

// StorageQuota returns the account's drive storage usage. Requires the
// drive.readonly scope, so an OAuth re-auth is needed for tokens minted
// before that scope was added.
func StorageQuota(ctx context.Context) (*gdrive.AboutStorageQuota, error) {
	srv, err := getClient()
	if err != nil {
		return nil, err
	}
	about, err := srv.About.Get().Fields("storageQuota").Do()
	if err != nil {
		return nil, fmt.Errorf("fetching google drive storage quota: %w", err)
	}
	return about.StorageQuota, nil
}
