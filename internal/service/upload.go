package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/alpha-abhii/upload-orchestration-service/internal/config"
	"github.com/alpha-abhii/upload-orchestration-service/internal/storage"
)

type UploadService struct {
	store  storage.Store
	config *config.Config
}

func NewUploadService(store storage.Store, cfg *config.Config) *UploadService {
	return &UploadService{store: store, config: cfg}
}

type InitiateUploadRequest struct {
	Filename      string `json:"filename"`
	ContentType   string `json:"content_type"`
	FileSizeBytes int64  `json:"file_size_bytes"`
}

type InitiateUploadResponse struct {
	UploadID   string `json:"upload_id"`
	Key        string `json:"key"`
	TotalParts int    `json:"total_parts"`
	ChunkSize  int64  `json:"chunk_size"`
}

func (s *UploadService) Initiate(ctx context.Context, req InitiateUploadRequest) (*InitiateUploadResponse, error) {
	if req.FileSizeBytes <= 0 {
		return nil, fmt.Errorf("file_size_bytes must be greater than 0")
	}

	if err := s.validateContentType(req.ContentType); err != nil {
		return nil, err
	}

	if err := s.validateFilename(req.Filename); err != nil {
		return nil, err
	}

	chunkSize := int64(5 * 1024 * 1024)
	totalParts := int(math.Ceil(float64(req.FileSizeBytes) / float64(chunkSize)))

	key := fmt.Sprintf("uploads/%s_%d", sanitizeFilename(req.Filename), time.Now().UnixNano())

	uploadID, err := s.store.InitiateMultipartUpload(ctx, key, req.ContentType)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate upload: %w", err)
	}

	return &InitiateUploadResponse{
		UploadID:   uploadID,
		Key:        key,
		TotalParts: totalParts,
		ChunkSize:  chunkSize,
	}, nil
}

func (s *UploadService) validateContentType(contentType string) error {
	allowed := []string{"video/mp4", "video/quicktime", "video/x-msvideo", "video/webm"}
	for _, a := range allowed {
		if a == contentType {
			return nil
		}
	}
	return fmt.Errorf("content_type %q is not allowed", contentType)
}

func (s *UploadService) validateFilename(filename string) error {
	if filename == "" {
		return fmt.Errorf("filename is required")
	}
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return fmt.Errorf("filename contains invalid characters")
	}
	return nil
}

func sanitizeFilename(filename string) string {
	replacer := strings.NewReplacer(" ", "_", "/", "_", "\\", "_")
	return replacer.Replace(filename)
}

type PresignedURLRequest struct {
	UploadID    string  `json:"upload_id"`
	Key         string  `json:"key"`
	PartNumbers []int32 `json:"part_numbers"`
}

type PresignedURLResponse struct {
	URLs map[string]string `json:"urls"`
}

type CompleteUploadRequest struct {
	UploadID string                  `json:"upload_id"`
	Key      string                  `json:"key"`
	Parts    []storage.CompletedPart `json:"parts"`
}

type CompleteUploadResponse struct {
	Key     string `json:"key"`
	Message string `json:"message"`
}

type AbortUploadRequest struct {
	UploadID string `json:"upload_id"`
	Key      string `json:"key"`
}

func (s *UploadService) GetPresignedURLs(ctx context.Context, req PresignedURLRequest) (*PresignedURLResponse, error) {
	if req.UploadID == "" {
		return nil, fmt.Errorf("upload_id is required")
	}
	if req.Key == "" {
		return nil, fmt.Errorf("key is required")
	}
	if len(req.PartNumbers) == 0 {
		return nil, fmt.Errorf("part_numbers must not be empty")
	}

	ttl := time.Duration(15) * time.Minute
	urls := make(map[string]string, len(req.PartNumbers))

	for _, partNum := range req.PartNumbers {
		url, err := s.store.GeneratePresignedURL(ctx, req.Key, req.UploadID, partNum, ttl)
		if err != nil {
			return nil, fmt.Errorf("failed to presign part %d: %w", partNum, err)
		}
		urls[fmt.Sprintf("%d", partNum)] = url
	}

	return &PresignedURLResponse{URLs: urls}, nil
}

func (s *UploadService) Complete(ctx context.Context, req CompleteUploadRequest) (*CompleteUploadResponse, error) {
	if req.UploadID == "" {
		return nil, fmt.Errorf("upload_id is required")
	}
	if req.Key == "" {
		return nil, fmt.Errorf("key is required")
	}
	if len(req.Parts) == 0 {
		return nil, fmt.Errorf("parts must not be empty")
	}

	for i, part := range req.Parts {
		if part.ETag == "" {
			return nil, fmt.Errorf("part at index %d is missing etag", i)
		}
	}

	if err := s.store.CompleteMultipartUpload(ctx, req.Key, req.UploadID, req.Parts); err != nil {
		return nil, fmt.Errorf("failed to complete upload: %w", err)
	}

	return &CompleteUploadResponse{
		Key:     req.Key,
		Message: "upload completed successfully",
	}, nil
}

func (s *UploadService) Abort(ctx context.Context, req AbortUploadRequest) error {
	if req.UploadID == "" {
		return fmt.Errorf("upload_id is required")
	}
	if req.Key == "" {
		return fmt.Errorf("key is required")
	}

	if err := s.store.AbortMultipartUpload(ctx, req.Key, req.UploadID); err != nil {
		return fmt.Errorf("failed to abort upload: %w", err)
	}

	return nil
}