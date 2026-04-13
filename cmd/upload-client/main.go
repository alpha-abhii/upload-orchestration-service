package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const (
	serverURL  = "http://localhost:8080"
	chunkSize  = 5 * 1024 * 1024 // 5MB
	maxRetries = 3
	maxWorkers = 3 // upload 3 chunks in parallel
)

type initiateResponse struct {
	UploadID   string `json:"upload_id"`
	Key        string `json:"key"`
	TotalParts int    `json:"total_parts"`
	ChunkSize  int64  `json:"chunk_size"`
}

type presignedURLResponse struct {
	URLs map[string]string `json:"urls"`
}

type completedPart struct {
	PartNumber int32  `json:"part_number"`
	ETag       string `json:"etag"`
}

type uploadResult struct {
	partNumber int32
	etag       string
	err        error
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: upload-client <filepath>")
	}

	filePath := os.Args[1]

	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("failed to open file: %v", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		log.Fatalf("failed to stat file: %v", err)
	}

	fileSize := stat.Size()
	log.Printf("uploading file: %s (%.2f MB)", filePath, float64(fileSize)/1024/1024)

	// Step 1: Initiate
	initResp, err := initiateUpload(filePath, fileSize)
	if err != nil {
		log.Fatalf("failed to initiate upload: %v", err)
	}
	log.Printf("upload initiated: id=%s parts=%d", initResp.UploadID, initResp.TotalParts)

	// Step 2: Get all presigned URLs
	partNumbers := make([]int32, initResp.TotalParts)
	for i := range partNumbers {
		partNumbers[i] = int32(i + 1)
	}

	urls, err := getPresignedURLs(initResp.UploadID, initResp.Key, partNumbers)
	if err != nil {
		log.Fatalf("failed to get presigned urls: %v", err)
	}
	log.Printf("got %d presigned URLs", len(urls))

	// Step 3: Upload chunks in parallel
	results, err := uploadChunks(file, fileSize, initResp, urls)
	if err != nil {
		log.Printf("upload failed, aborting: %v", err)
		abortUpload(initResp.UploadID, initResp.Key)
		os.Exit(1)
	}

	// Step 4: Complete
	if err := completeUpload(initResp.UploadID, initResp.Key, results); err != nil {
		log.Fatalf("failed to complete upload: %v", err)
	}

	log.Printf("upload complete! key=%s", initResp.Key)
}

func initiateUpload(filename string, fileSize int64) (*initiateResponse, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"filename":        filepath.Base(filename),
		"content_type":    "video/mp4",
		"file_size_bytes": fileSize,
	})

	resp, err := http.Post(serverURL+"/api/v1/upload/initiate", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result initiateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func getPresignedURLs(uploadID, key string, partNumbers []int32) (map[string]string, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"upload_id":    uploadID,
		"key":          key,
		"part_numbers": partNumbers,
	})

	resp, err := http.Post(serverURL+"/api/v1/upload/presigned-urls", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result presignedURLResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.URLs, nil
}

func uploadChunks(file *os.File, fileSize int64, initResp *initiateResponse, urls map[string]string) ([]completedPart, error) {
	// jobs channel feeds work to workers
	// results channel collects outcomes
	jobs := make(chan int32, initResp.TotalParts)
	results := make(chan uploadResult, initResp.TotalParts)

	// Start worker pool
	var wg sync.WaitGroup
	for w := 0; w < maxWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for partNum := range jobs {
				etag, err := uploadPartWithRetry(file, fileSize, partNum, urls[fmt.Sprintf("%d", partNum)])
				results <- uploadResult{partNumber: partNum, etag: etag, err: err}
			}
		}()
	}

	// Send all jobs
	for i := 1; i <= initResp.TotalParts; i++ {
		jobs <- int32(i)
	}
	close(jobs)

	// Wait for all workers then close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var parts []completedPart
	for r := range results {
		if r.err != nil {
			return nil, fmt.Errorf("part %d failed: %w", r.partNumber, r.err)
		}
		log.Printf("part %d uploaded, etag=%s", r.partNumber, r.etag)
		parts = append(parts, completedPart{PartNumber: r.partNumber, ETag: r.etag})
	}

	sort.Slice(parts, func(i, j int) bool {
		return parts[i].PartNumber < parts[j].PartNumber
	})

	return parts, nil
}

func uploadPartWithRetry(file *os.File, fileSize int64, partNumber int32, url string) (string, error) {
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		etag, err := uploadPart(file, fileSize, partNumber, url)
		if err == nil {
			return etag, nil
		}

		lastErr = err
		log.Printf("part %d attempt %d failed: %v, retrying...", partNumber, attempt, err)
		time.Sleep(time.Duration(attempt) * time.Second) // exponential backoff
	}

	return "", fmt.Errorf("part %d failed after %d attempts: %w", partNumber, maxRetries, lastErr)
}

func uploadPart(file *os.File, fileSize int64, partNumber int32, url string) (string, error) {
	offset := int64(partNumber-1) * chunkSize

	size := int64(chunkSize)
	if offset+size > fileSize {
		size = fileSize - offset
	}

	chunk := make([]byte, size)
	if _, err := file.ReadAt(chunk, offset); err != nil && err != io.EOF {
		return "", fmt.Errorf("failed to read chunk: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(chunk))
	if err != nil {
		return "", err
	}
	req.ContentLength = size

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	etag := resp.Header.Get("ETag")
	if etag == "" {
		return "", fmt.Errorf("missing ETag in response")
	}

	return etag, nil
}

func completeUpload(uploadID, key string, parts []completedPart) error {
	body, _ := json.Marshal(map[string]interface{}{
		"upload_id": uploadID,
		"key":       key,
		"parts":     parts,
	})

	resp, err := http.Post(serverURL+"/api/v1/upload/complete", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("complete failed: %s", string(body))
	}

	return nil
}

func abortUpload(uploadID, key string) {
	body, _ := json.Marshal(map[string]interface{}{
		"upload_id": uploadID,
		"key":       key,
	})

	req, _ := http.NewRequest(http.MethodDelete, serverURL+"/api/v1/upload/abort", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	http.DefaultClient.Do(req)
	log.Println("upload aborted")
}