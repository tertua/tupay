package storage

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/tertua/tupay/pkg/configs"
)

// s3Storage stores files in an S3-compatible bucket via minio-go.
type s3Storage struct {
	client    *minio.Client
	bucket    string
	publicURL string
}

func openS3(cfg configs.StorageConfig) (*s3Storage, error) {
	client, err := minio.New(cfg.S3Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.S3AccessKey, cfg.S3SecretKey, ""),
		Secure: cfg.S3UseSSL,
		Region: cfg.S3Region,
	})
	if err != nil {
		return nil, err
	}
	s := &s3Storage{client: client, bucket: cfg.S3Bucket, publicURL: cfg.PublicURL}
	if err := s.ensureBucket(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *s3Storage) ensureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{})
}

func (s *s3Storage) Put(ctx context.Context, key string, data io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, data, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

func (s *s3Storage) Get(ctx context.Context, key string) (rc io.ReadCloser, contentType string, err error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", err
	}
	info, err := obj.Stat()
	if err != nil {
		_ = obj.Close()
		return nil, "", err
	}
	ct := info.ContentType
	if ct == "" {
		ct = contentTypeOf(key)
	}
	return obj, ct, nil
}

func (s *s3Storage) Delete(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}

func (s *s3Storage) URLFor(key string, visibility Visibility) string {
	if visibility == Public && s.publicURL != "" {
		return s.publicURL + "/" + s.bucket + "/" + strings.TrimPrefix(key, "/")
	}
	return key
}

// ObjectURL exposes the configured public base for docs and logs.
func (s *s3Storage) ObjectURL() string {
	return fmt.Sprintf("%s/%s/", strings.TrimRight(s.publicURL, "/"), s.bucket)
}
