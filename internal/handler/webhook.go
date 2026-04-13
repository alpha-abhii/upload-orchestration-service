package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// S3EventRecord represents one event from S3.
// S3 sends these when objects are created, deleted, etc.
type S3EventRecord struct {
	EventName string `json:"eventName"` // e.g. "ObjectCreated:CompleteMultipartUpload"
	S3        struct {
		Bucket struct {
			Name string `json:"name"`
		} `json:"bucket"`
		Object struct {
			Key  string `json:"key"`
			Size int64  `json:"size"`
		} `json:"object"`
	} `json:"s3"`
}

// S3Event is the full payload S3 sends to your webhook.
type S3Event struct {
	Records []S3EventRecord `json:"Records"`
}

type WebhookHandler struct{}

func NewWebhookHandler() *WebhookHandler {
	return &WebhookHandler{}
}

func (h *WebhookHandler) HandleS3Event(w http.ResponseWriter, r *http.Request) {
	var event S3Event

	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		slog.Error("failed to decode S3 event", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	for _, record := range event.Records {
		slog.Info("s3 event received",
			"event", record.EventName,
			"bucket", record.S3.Bucket.Name,
			"key", record.S3.Object.Key,
			"size_bytes", record.S3.Object.Size,
			"received_at", time.Now().UTC().Format(time.RFC3339),
		)

		// Only process completed multipart uploads
		if record.EventName == "ObjectCreated:CompleteMultipartUpload" {
			if err := h.processVideoUpload(record); err != nil {
				slog.Error("failed to process video upload",
					"key", record.S3.Object.Key,
					"error", err,
				)
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

// processVideoUpload is called when a video upload completes.
// In production this would enqueue a job to a worker queue (SQS, Redis, etc.)
func (h *WebhookHandler) processVideoUpload(record S3EventRecord) error {
	slog.Info("processing completed video upload",
		"key", record.S3.Object.Key,
		"size_mb", record.S3.Object.Size/1024/1024,
	)

	// PRODUCTION NEXT STEPS:
	// 1. Push to SQS queue → worker picks it up → runs FFmpeg transcoding
	// 2. Generate multiple resolutions: 1080p, 720p, 480p, 360p
	// 3. Generate thumbnail at 5 second mark
	// 4. Update database: uploads table, set status = "processing"
	// 5. When transcoding done, set status = "ready", store CDN URLs

	return nil
}