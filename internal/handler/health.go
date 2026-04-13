package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Pinger interface {
	HeadBucket(ctx context.Context, params *s3.HeadBucketInput, optFns ...func(*s3.Options)) (*s3.HeadBucketOutput, error)
}

type HealthHandler struct {
	s3Client S3Pinger
	bucket   string
}

func NewHealthHandler(s3Client S3Pinger, bucket string) *HealthHandler {
	return &HealthHandler{s3Client: s3Client, bucket: bucket}
}

type healthResponse struct {
	Status     string            `json:"status"`
	Components map[string]string `json:"components"`
	Timestamp  string            `json:"timestamp"`
}

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	components := make(map[string]string)
	overallStatus := "ok"

	// Check S3 connectivity with a 3 second timeout
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	_, err := h.s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(h.bucket),
	})
	if err != nil {
		components["s3"] = "unhealthy: " + err.Error()
		overallStatus = "degraded"
	} else {
		components["s3"] = "healthy"
	}

	statusCode := http.StatusOK
	if overallStatus != "ok" {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(healthResponse{
		Status:     overallStatus,
		Components: components,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	})
}