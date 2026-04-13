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