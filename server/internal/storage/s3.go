package storage

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3 is an S3-compatible Storage provider. Custom endpoints make it work
// with AWS S3, Cloudflare R2, MinIO, Backblaze B2, Wasabi, GCS compat, etc.
type S3 struct {
	client    *s3.Client
	bucket    string
	region    string
	publicURL string
	presigner *s3.PresignClient
}

// NewS3 builds an S3 provider from cfg. If Endpoint is empty, it uses the
// AWS default endpoint resolution (real S3). Otherwise it points at the
// custom endpoint (MinIO/R2/etc.) with path-style addressing for MinIO-style
// servers.
func NewS3(ctx context.Context, cfg Config) (*S3, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3: bucket is required")
	}
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}

	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
	}
	if cfg.Endpoint != "" {
		opts = append(opts, awsconfig.WithBaseEndpoint(normalizeEndpoint(cfg.Endpoint)))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("s3: loading config: %w", err)
	}

	clientOpts := []func(*s3.Options){}
	if cfg.Endpoint != "" {
		// Custom endpoints (MinIO, R2) use path-style addressing.
		clientOpts = append(clientOpts, func(o *s3.Options) {
			o.UsePathStyle = true
		})
	}

	s := &S3{
		client:    s3.NewFromConfig(awsCfg, clientOpts...),
		bucket:    cfg.Bucket,
		region:    region,
		publicURL: strings.TrimRight(cfg.PublicURL, "/"),
	}
	s.presigner = s3.NewPresignClient(s.client)
	return s, nil
}

// key applies the optional root prefix to a storage path.
func (s *S3) key(path string) string {
	return strings.TrimLeft(path, "/")
}

// Upload streams r to the given key.
func (s *S3) Upload(ctx context.Context, path string, r io.Reader, contentType string) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(s.key(path)),
		Body:        r,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("s3: upload: %w", err)
	}
	return nil
}

// Download fetches the object and returns its body + size.
func (s *S3) Download(ctx context.Context, path string) (io.ReadCloser, int64, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key(path)),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("s3: download: %w", err)
	}
	size := int64(-1)
	if out.ContentLength != nil {
		size = *out.ContentLength
	}
	return out.Body, size, nil
}

// GetSignedURL returns a presigned GET URL for the object.
func (s *S3) GetSignedURL(ctx context.Context, path string, expiresIn time.Duration) (string, error) {
	req, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key(path)),
	}, func(o *s3.PresignOptions) {
		o.Expires = expiresIn
	})
	if err != nil {
		return "", fmt.Errorf("s3: presign: %w", err)
	}
	return req.URL, nil
}

// Delete removes the object.
func (s *S3) Delete(ctx context.Context, path string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key(path)),
	})
	if err != nil {
		return fmt.Errorf("s3: delete: %w", err)
	}
	return nil
}

// Exists checks object existence via HeadObject (or a cheap list fallback).
func (s *S3) Exists(ctx context.Context, path string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key(path)),
	})
	if err != nil {
		var nsk *types.NotFound
		if errAs(err, &nsk) {
			return false, nil
		}
		return false, fmt.Errorf("s3: head: %w", err)
	}
	return true, nil
}

// List returns objects under prefix (S3 ListObjectsV2 paginated).
func (s *S3) List(ctx context.Context, prefix string) ([]Object, error) {
	var out []Object
	delimiter := ""
	continuation := (*string)(nil)
	for {
		outList, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(s.bucket),
			Prefix:            aws.String(prefix),
			Delimiter:         aws.String(delimiter),
			ContinuationToken: continuation,
		})
		if err != nil {
			return nil, fmt.Errorf("s3: list: %w", err)
		}
		for _, obj := range outList.Contents {
			size := int64(0)
			if obj.Size != nil {
				size = *obj.Size
			}
			lastMod := time.Time{}
			if obj.LastModified != nil {
				lastMod = *obj.LastModified
			}
			out = append(out, Object{
				Path:         *obj.Key,
				Size:         size,
				LastModified: lastMod,
			})
		}
		if outList.IsTruncated != nil && *outList.IsTruncated && outList.NextContinuationToken != nil {
			continuation = outList.NextContinuationToken
			continue
		}
		break
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// Quota reports 0/0 for S3 (usage not available without extra API calls).
func (s *S3) Quota(ctx context.Context) (int64, int64, error) {
	return 0, 0, nil
}

// Ping verifies bucket and credentials via the S3 HeadBucket API, so a
// wrong bucket name (e.g. a typo in the stores form) fails the connection
// test instead of passing silently.
func (s *S3) Ping(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if err != nil {
		return fmt.Errorf("s3: ping: %w", err)
	}
	return nil
}

// normalizeEndpoint ensures an endpoint has a scheme. AWS SDK requires
// endpoints to include http:// or https://, but users often enter bare
// hostnames like "s3.us-east-005.backblazeb2.com".
func normalizeEndpoint(endpoint string) string {
	if !strings.Contains(endpoint, "://") {
		return "https://" + endpoint
	}
	return endpoint
}

// errAs is a small helper so we can avoid importing errors in this file
// (kept local to keep the diff minimal).
func errAs(err error, target **types.NotFound) bool {
	for err != nil {
		if _, ok := err.(*types.NotFound); ok {
			*target = err.(*types.NotFound)
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
