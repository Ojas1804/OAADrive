// Package storage wraps an S3-compatible client (pointed at SeaweedFS's S3
// gateway) for uploading, downloading, and presigning object URLs.
package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Buckets used by the app, matching the ones created by setup.sh.
const (
	BucketPhotos    = "photos"
	BucketDocuments = "documents"
	BucketBackups   = "backups"
)

// Client wraps the S3 API and presign clients for a single SeaweedFS endpoint.
type Client struct {
	api     *s3.Client
	presign *s3.PresignClient
}

// NewClient builds a client against an S3-compatible endpoint such as
// SeaweedFS's S3 gateway (host:port, no scheme, e.g. "seaweedfs:8333").
// Path-style addressing is required since SeaweedFS does not support
// virtual-hosted-style bucket URLs.
func NewClient(endpoint, accessKey, secretKey string) (*Client, error) {
	if endpoint == "" {
		return nil, fmt.Errorf("storage endpoint is required")
	}

	api := s3.New(s3.Options{
		Region:       "us-east-1",
		Credentials:  credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		BaseEndpoint: aws.String("http://" + endpoint),
		UsePathStyle: true,
	})

	return &Client{api: api, presign: s3.NewPresignClient(api)}, nil
}

func (c *Client) Upload(ctx context.Context, bucket, key string, body io.Reader, size int64, contentType string) error {
	_, err := c.api.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
	})
	return err
}

func (c *Client) Delete(ctx context.Context, bucket, key string) error {
	_, err := c.api.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err
}

func (c *Client) PresignGet(ctx context.Context, bucket, key string, ttl time.Duration) (string, error) {
	out, err := c.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", err
	}
	return out.URL, nil
}

func (c *Client) PresignPut(ctx context.Context, bucket, key string, ttl time.Duration) (string, error) {
	out, err := c.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", err
	}
	return out.URL, nil
}
