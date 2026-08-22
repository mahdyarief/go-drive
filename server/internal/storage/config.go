package storage

// Config carries the credentials and settings needed to build a Storage
// provider. It is the Go equivalent of Locker's createStorageFromConfig:
// the handler layer populates it from a stores row + decrypted store_secrets.
type Config struct {
	Provider  string // "local" | "s3" | "r2" | "b2" | "wasabi" | "spaces" | "hetzner" | "idrivee2" | "storj" | "gdrive"
	BaseDir   string // local: root directory on disk
	PublicURL string // local/s3: base URL for signed URLs (optional)

	Bucket     string // s3: bucket name
	Region     string // s3: region (default us-east-1)
	Endpoint   string // s3: custom endpoint (MinIO, R2, Backblaze, ...)
	AccessKey  string // s3: access key id
	SecretKey  string // s3: secret access key
	RootPrefix string // s3/local: optional prefix prepended to object keys

	ClientID     string // gdrive: OAuth app client id (shared across accounts)
	ClientSecret string // gdrive: OAuth app client secret
	RefreshToken string // gdrive: per-account refresh token
	FolderID     string // gdrive: per-account root folder id
}
