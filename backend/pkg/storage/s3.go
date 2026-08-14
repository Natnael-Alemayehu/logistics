package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type S3Storage struct {
	client *s3.Client
	bucket string
	region string
}

func NewS3Storage(bucket, region, endpoint, accessKey, secretKey string, useSSL bool) (*S3Storage, error) {
	if bucket == "" {
		return nil, errors.New("bucket name is required")
	}
	if region == "" {
		region = "us-east-1"
	}

	var opts []func(*config.LoadOptions) error

	if accessKey != "" && secretKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		))
	}

	cfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	clientOpts := []func(*s3.Options){
		func(o *s3.Options) {
			o.Region = region
		},
	}

	if endpoint != "" {
		clientOpts = append(clientOpts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
			if !useSSL && strings.HasPrefix(endpoint, "http://") {
				o.UsePathStyle = true
			}
		})
	}

	client := s3.NewFromConfig(cfg, clientOpts...)

	return &S3Storage{
		client: client,
		bucket: bucket,
		region: region,
	}, nil
}

func (s *S3Storage) Upload(ctx context.Context, path string, reader io.Reader, contentType string) (string, error) {
	if path == "" {
		return "", ErrInvalidPath
	}

	path = strings.TrimPrefix(path, "/")

	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(path),
		Body:        reader,
		ContentType: aws.String(contentType),
	}

	_, err := s.client.PutObject(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	return path, nil
}

func (s *S3Storage) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	if path == "" {
		return nil, ErrInvalidPath
	}

	path = strings.TrimPrefix(path, "/")

	input := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	}

	result, err := s.client.GetObject(ctx, input)
	if err != nil {
		var noSuchKey *types.NoSuchKey
		if errors.As(err, &noSuchKey) {
			return nil, ErrFileNotFound
		}
		return nil, fmt.Errorf("failed to download from S3: %w", err)
	}

	return result.Body, nil
}

func (s *S3Storage) Delete(ctx context.Context, path string) error {
	if path == "" {
		return ErrInvalidPath
	}

	path = strings.TrimPrefix(path, "/")

	input := &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	}

	_, err := s.client.DeleteObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete from S3: %w", err)
	}

	return nil
}

func (s *S3Storage) GetURL(ctx context.Context, path string) (string, error) {
	if path == "" {
		return "", ErrInvalidPath
	}

	path = strings.TrimPrefix(path, "/")

	presignClient := s3.NewPresignClient(s.client)

	input := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	}

	presignedReq, err := presignClient.PresignGetObject(ctx, input, s3.WithPresignExpires(15*time.Minute))
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return presignedReq.URL, nil
}

func (s *S3Storage) GetPublicURL(path string) string {
	path = strings.TrimPrefix(path, "/")
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, s.region, url.PathEscape(path))
}
