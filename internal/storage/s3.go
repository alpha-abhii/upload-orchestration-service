package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/alpha-abhii/upload-orchestration-service/internal/config"
)

type Store interface {
	InitiateMultipartUpload(ctx context.Context, key string, contentType string) (string, error)
	GeneratePresignedURL(ctx context.Context, key string, uploadID string, partNumber int32, ttl time.Duration) (string, error)
	CompleteMultipartUpload(ctx context.Context, key string, uploadID string, parts []CompletedPart) error
	AbortMultipartUpload(ctx context.Context, key string, uploadID string) error
	ListUploadedParts(ctx context.Context, key string, uploadID string) ([]UploadedPart, error)
}

type CompletedPart struct {
	PartNumber int32  `json:"part_number"`
	ETag       string `json:"etag"`
}

type UploadedPart struct {
	PartNumber int32  `json:"part_number"`
	ETag       string `json:"etag"`
	Size       int64  `json:"size"`
}

type S3Store struct {
	client    *s3.Client
	presigner *s3.PresignClient
	bucket    string
}

func NewS3Store(cfg *config.Config) (*S3Store, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(
		context.Background(),
		awsconfig.WithRegion(cfg.AWSRegion),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				cfg.AWSAccessKey,
				cfg.AWSSecretKey,
				"",
			),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = false
	})

	return &S3Store{
		client:    client,
		presigner: s3.NewPresignClient(client),
		bucket:    cfg.S3Bucket,
	}, nil
}

func (s *S3Store) S3Client() *s3.Client {
	return s.client
}

func (s *S3Store) Bucket() string {
	return s.bucket
}

func (s *S3Store) InitiateMultipartUpload(ctx context.Context, key string, contentType string) (string, error) {
	input := &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
		ServerSideEncryption: types.ServerSideEncryptionAes256,
	}

	result, err := s.client.CreateMultipartUpload(ctx, input)
	if err != nil {
		return "", fmt.Errorf("CreateMultipartUpload failed: %w", err)
	}

	return aws.ToString(result.UploadId), nil
}

func (s *S3Store) GeneratePresignedURL(ctx context.Context, key string, uploadID string, partNumber int32, ttl time.Duration) (string, error) {
	input := &s3.UploadPartInput{
		Bucket:     aws.String(s.bucket),
		Key:        aws.String(key),
		UploadId:   aws.String(uploadID),
		PartNumber: aws.Int32(partNumber),
	}

	result, err := s.presigner.PresignUploadPart(ctx, input,
		s3.WithPresignExpires(ttl),
	)
	if err != nil {
		return "", fmt.Errorf("PresignUploadPart failed for part %d: %w", partNumber, err)
	}

	return result.URL, nil
}

func (s *S3Store) CompleteMultipartUpload(ctx context.Context, key string, uploadID string, parts []CompletedPart) error {
	completed := make([]types.CompletedPart, len(parts))
	for i, p := range parts {
		completed[i] = types.CompletedPart{
			PartNumber: aws.Int32(p.PartNumber),
			ETag:       aws.String(p.ETag),
		}
	}

	input := &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(s.bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completed,
		},
	}

	_, err := s.client.CompleteMultipartUpload(ctx, input)
	if err != nil {
		return fmt.Errorf("CompleteMultipartUpload failed: %w", err)
	}

	return nil
}

func (s *S3Store) AbortMultipartUpload(ctx context.Context, key string, uploadID string) error {
	input := &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(s.bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	}

	_, err := s.client.AbortMultipartUpload(ctx, input)
	if err != nil {
		return fmt.Errorf("AbortMultipartUpload failed: %w", err)
	}

	return nil
}

func (s *S3Store) ListUploadedParts(ctx context.Context, key string, uploadID string) ([]UploadedPart, error) {
	var parts []UploadedPart
	var partNumberMarker *string

	for {
		input := &s3.ListPartsInput{
			Bucket:           aws.String(s.bucket),
			Key:              aws.String(key),
			UploadId:         aws.String(uploadID),
			PartNumberMarker: partNumberMarker,
		}

		result, err := s.client.ListParts(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("ListParts failed: %w", err)
		}

		for _, p := range result.Parts {
			parts = append(parts, UploadedPart{
				PartNumber: aws.ToInt32(p.PartNumber),
				ETag:       aws.ToString(p.ETag),
				Size:       aws.ToInt64(p.Size),
			})
		}

		if !aws.ToBool(result.IsTruncated) {
			break
		}

		partNumberMarker = result.NextPartNumberMarker
	}

	return parts, nil
}