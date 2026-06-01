// Package s3 implements the storage.ObjectStore port over the AWS S3 API.
//
// We use aws-sdk-go-v2 with a configurable endpoint (per ADR 0007) so
// the same adapter works against MinIO in dev, AWS S3 in prod, and
// Cloudflare R2 in self-hosted setups. Bucket names are passed in by
// the caller; the adapter never knows them.
package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"

	"github.com/tomeku/doclens/services/shared/storage"
)

// Config carries everything the adapter needs at construction time.
type Config struct {
	// Endpoint is the S3 API URL. Empty means use the AWS default.
	// MinIO/R2 require explicit endpoints (e.g. http://localhost:9000).
	Endpoint string
	// Region is the bucket region. MinIO accepts any value; R2 uses
	// "auto"; AWS uses the real region.
	Region string
	// AccessKeyID and SecretAccessKey are static credentials. In
	// production AWS environments leave them blank to use the default
	// credential chain (IRSA, instance role).
	AccessKeyID     string
	SecretAccessKey string
	// UsePathStyle forces "host/bucket/key" addressing rather than
	// "bucket.host/key". Required for MinIO; harmless on AWS.
	UsePathStyle bool
}

// Adapter is the S3-backed ObjectStore.
type Adapter struct {
	client    *s3.Client
	presigner *s3.PresignClient
}

// New returns an Adapter ready to use.
func New(ctx context.Context, cfg Config) (*Adapter, error) {
	loadOpts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
	}
	if cfg.AccessKeyID != "" {
		loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("s3: load config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.UsePathStyle
	})

	return &Adapter{
		client:    client,
		presigner: s3.NewPresignClient(client),
	}, nil
}

// PresignPut implements storage.ObjectStore.
func (a *Adapter) PresignPut(ctx context.Context, bucket, key string, opts storage.PresignPutOptions) (storage.PresignedURL, error) {
	ttl := capTTL(opts.TTL)

	in := &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}
	if opts.ContentType != "" {
		in.ContentType = aws.String(opts.ContentType)
	}
	if opts.ContentLength > 0 {
		in.ContentLength = aws.Int64(opts.ContentLength)
	}

	signed, err := a.presigner.PresignPutObject(ctx, in, s3.WithPresignExpires(ttl))
	if err != nil {
		return storage.PresignedURL{}, fmt.Errorf("s3: presign put: %w", err)
	}

	return storage.PresignedURL{
		URL:       signed.URL,
		ExpiresAt: time.Now().Add(ttl),
	}, nil
}

// PresignGet implements storage.ObjectStore.
func (a *Adapter) PresignGet(ctx context.Context, bucket, key string, ttl time.Duration) (storage.PresignedURL, error) {
	ttl = capTTL(ttl)

	signed, err := a.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return storage.PresignedURL{}, fmt.Errorf("s3: presign get: %w", err)
	}

	return storage.PresignedURL{
		URL:       signed.URL,
		ExpiresAt: time.Now().Add(ttl),
	}, nil
}

// Head implements storage.ObjectStore.
func (a *Adapter) Head(ctx context.Context, bucket, key string) (storage.ObjectInfo, error) {
	out, err := a.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NotFound" {
			return storage.ObjectInfo{}, storage.ErrNotFound
		}
		return storage.ObjectInfo{}, fmt.Errorf("s3: head: %w", err)
	}

	info := storage.ObjectInfo{}
	if out.ContentLength != nil {
		info.ByteSize = *out.ContentLength
	}
	if out.ETag != nil {
		info.ETag = *out.ETag
	}
	if out.LastModified != nil {
		info.LastModified = *out.LastModified
	}
	if out.ContentType != nil {
		info.ContentType = *out.ContentType
	}
	return info, nil
}

// Delete implements storage.ObjectStore. Deleting a missing object is a
// no-op, mirroring S3's DELETE semantics.
func (a *Adapter) Delete(ctx context.Context, bucket, key string) error {
	_, err := a.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("s3: delete: %w", err)
	}
	return nil
}

func capTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 || ttl > storage.MaxPresignTTL {
		return storage.MaxPresignTTL
	}
	return ttl
}


// Get implements storage.ObjectStore.
func (a *Adapter) Get(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	out, err := a.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && (apiErr.ErrorCode() == "NoSuchKey" || apiErr.ErrorCode() == "NotFound") {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("s3: get: %w", err)
	}
	return out.Body, nil
}

// Put implements storage.ObjectStore.
func (a *Adapter) Put(ctx context.Context, bucket, key string, body io.Reader, opts storage.PutOptions) error {
	in := &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   body,
	}
	if opts.ContentType != "" {
		in.ContentType = aws.String(opts.ContentType)
	}
	if opts.ContentLength > 0 {
		in.ContentLength = aws.Int64(opts.ContentLength)
	}
	if _, err := a.client.PutObject(ctx, in); err != nil {
		return fmt.Errorf("s3: put: %w", err)
	}
	return nil
}
