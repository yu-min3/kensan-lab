package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/kensan/backend/shared/telemetry"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Config holds storage configuration
type Config struct {
	Endpoint   string // minio:9000
	AccessKey  string
	SecretKey  string
	Bucket     string
	UseSSL     bool
	PublicURL  string // http://localhost:9000/kensan-notes
}

// Client provides S3-compatible storage operations
type Client struct {
	minio     *minio.Client
	bucket    string
	publicURL string
}

// NewClient creates a new storage client and ensures the bucket exists
func NewClient(cfg Config) (*Client, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:     credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure:    cfg.UseSSL,
		Transport: telemetry.InstrumentedTransport(nil),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	// Ensure bucket exists
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket existence: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("failed to create bucket %s: %w", cfg.Bucket, err)
		}
	}

	return &Client{
		minio:     client,
		bucket:    cfg.Bucket,
		publicURL: cfg.PublicURL,
	}, nil
}

// Upload uploads content to storage
func (c *Client) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	_, err := c.minio.PutObject(ctx, c.bucket, key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("failed to upload object: %w", err)
	}
	return nil
}

// Download downloads content from storage
func (c *Client) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := c.minio.GetObject(ctx, c.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	return obj, nil
}

// Delete deletes content from storage
func (c *Client) Delete(ctx context.Context, key string) error {
	err := c.minio.RemoveObject(ctx, c.bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}
	return nil
}

// GetPresignedUploadURL generates a presigned URL for uploading
func (c *Client) GetPresignedUploadURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	presignedURL, err := c.minio.PresignedPutObject(ctx, c.bucket, key, expiry)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned upload URL: %w", err)
	}
	return presignedURL.String(), nil
}

// GetPresignedDownloadURL generates a presigned URL for downloading
func (c *Client) GetPresignedDownloadURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	reqParams := make(url.Values)
	presignedURL, err := c.minio.PresignedGetObject(ctx, c.bucket, key, expiry, reqParams)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned download URL: %w", err)
	}
	return presignedURL.String(), nil
}

// GetPublicURL returns the public URL for an object
func (c *Client) GetPublicURL(key string) string {
	return fmt.Sprintf("%s/%s", c.publicURL, key)
}

// Exists checks if an object exists
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	_, err := c.minio.StatObject(ctx, c.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		errResponse := minio.ToErrorResponse(err)
		if errResponse.Code == "NoSuchKey" {
			return false, nil
		}
		return false, fmt.Errorf("failed to check object existence: %w", err)
	}
	return true, nil
}

// GenerateKey generates a storage key for a note content
func GenerateKey(noteID, contentID, extension string) string {
	return fmt.Sprintf("notes/%s/%s%s", noteID, contentID, extension)
}

// GetExtensionForContentType returns file extension for content type
func GetExtensionForContentType(contentType string) string {
	switch contentType {
	case "markdown":
		return ".md"
	case "drawio":
		return ".drawio"
	case "image":
		return ".png" // Default, actual extension from filename
	case "pdf":
		return ".pdf"
	case "code":
		return ".txt"
	default:
		return ""
	}
}
