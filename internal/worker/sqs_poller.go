// Package worker contains background workers that run alongside the HTTP server.
package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// S3EventRecord mirrors the structure S3 sends to SQS.
type S3EventRecord struct {
	EventName string `json:"eventName"`
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

type S3Event struct {
	Records []S3EventRecord `json:"Records"`
}

// SQSPoller polls an SQS queue for S3 events and processes them.
type SQSPoller struct {
	client   *sqs.Client
	queueURL string
}

// NewSQSPoller creates a new SQSPoller.
func NewSQSPoller(client *sqs.Client, queueURL string) *SQSPoller {
	return &SQSPoller{
		client:   client,
		queueURL: queueURL,
	}
}

// Start begins polling the SQS queue in a loop.
// It runs until the context is cancelled — perfect for graceful shutdown.
func (p *SQSPoller) Start(ctx context.Context) {
	slog.Info("sqs poller started", "queue", p.queueURL)

	for {
		select {
		case <-ctx.Done():
			slog.Info("sqs poller stopped")
			return
		default:
			p.poll(ctx)
		}
	}
}

// poll performs a single long-poll request to SQS.
// WaitTimeSeconds=10 means SQS holds the connection open for up to 10 seconds
// if there are no messages — reduces empty responses and API costs.
func (p *SQSPoller) poll(ctx context.Context) {
	result, err := p.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(p.queueURL),
		MaxNumberOfMessages: 10,
		WaitTimeSeconds:     10, // long polling
	})
	if err != nil {
		// Context cancelled — normal shutdown
		if ctx.Err() != nil {
			return
		}
		slog.Error("failed to receive SQS messages", "error", err)
		time.Sleep(5 * time.Second) // back off on error
		return
	}

	for _, msg := range result.Messages {
		p.processMessage(ctx, msg)
	}
}

// processMessage handles a single SQS message.
// It always deletes the message after processing — even on error.
// WHY: If we don't delete, SQS will redeliver it after the visibility timeout.
// For permanent errors (malformed JSON), we'd loop forever.
// In production, use a Dead Letter Queue (DLQ) for failed messages.
func (p *SQSPoller) processMessage(ctx context.Context, msg types.Message) {
	defer p.deleteMessage(ctx, msg.ReceiptHandle)

	if msg.Body == nil {
		return
	}

	var event S3Event
	if err := json.Unmarshal([]byte(*msg.Body), &event); err != nil {
		slog.Error("failed to parse SQS message", "error", err, "body", *msg.Body)
		return
	}

	for _, record := range event.Records {
		slog.Info("s3 event received via SQS",
			"event", record.EventName,
			"bucket", record.S3.Bucket.Name,
			"key", record.S3.Object.Key,
			"size_bytes", record.S3.Object.Size,
		)

		if record.EventName == "ObjectCreated:CompleteMultipartUpload" {
			slog.Info("processing completed video upload",
				"key", record.S3.Object.Key,
				"size_mb", record.S3.Object.Size/1024/1024,
			)
			// In production: push to a job queue for transcoding,
			// thumbnail generation, virus scanning, etc.
		}
	}
}

// deleteMessage removes a processed message from the queue.
func (p *SQSPoller) deleteMessage(ctx context.Context, receiptHandle *string) {
	_, err := p.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(p.queueURL),
		ReceiptHandle: receiptHandle,
	})
	if err != nil {
		slog.Error("failed to delete SQS message", "error", err)
	}
}