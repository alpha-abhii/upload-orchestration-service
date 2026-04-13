package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/alpha-abhii/upload-orchestration-service/internal/service"
	"github.com/alpha-abhii/upload-orchestration-service/pkg/apierror"
)

type UploadHandler struct {
	svc *service.UploadService
}

func NewUploadHandler(svc *service.UploadService) *UploadHandler {
	return &UploadHandler{svc: svc}
}

func (h *UploadHandler) Initiate(w http.ResponseWriter, r *http.Request) {
	var req service.InitiateUploadRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest(w, "invalid request body")
		return
	}

	resp, err := h.svc.Initiate(r.Context(), req)
	if err != nil {
		slog.Error("initiate upload failed", "error", err)
		apierror.BadRequest(w, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (h *UploadHandler) GetPresignedURLs(w http.ResponseWriter, r *http.Request) {
	var req service.PresignedURLRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest(w, "invalid request body")
		return
	}

	resp, err := h.svc.GetPresignedURLs(r.Context(), req)
	if err != nil {
		slog.Error("get presigned urls failed", "error", err)
		apierror.BadRequest(w, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (h *UploadHandler) Complete(w http.ResponseWriter, r *http.Request) {
	var req service.CompleteUploadRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest(w, "invalid request body")
		return
	}

	resp, err := h.svc.Complete(r.Context(), req)
	if err != nil {
		slog.Error("complete upload failed", "error", err)
		apierror.BadRequest(w, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (h *UploadHandler) Abort(w http.ResponseWriter, r *http.Request) {
	var req service.AbortUploadRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest(w, "invalid request body")
		return
	}

	if err := h.svc.Abort(r.Context(), req); err != nil {
		slog.Error("abort upload failed", "error", err)
		apierror.BadRequest(w, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *UploadHandler) GetUploadStatus(w http.ResponseWriter, r *http.Request) {
	uploadID := r.URL.Query().Get("upload_id")
	key := r.URL.Query().Get("key")

	if uploadID == "" || key == "" {
		apierror.BadRequest(w, "upload_id and key are required query parameters")
		return
	}

	resp, err := h.svc.GetUploadStatus(r.Context(), service.UploadStatusRequest{
		UploadID: uploadID,
		Key:      key,
	})
	if err != nil {
		slog.Error("get upload status failed", "error", err)
		apierror.BadRequest(w, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (h *UploadHandler) GetDownloadURL(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")

	if key == "" {
		apierror.BadRequest(w, "key is required query parameter")
		return
	}

	resp, err := h.svc.GetDownloadURL(r.Context(), service.GetDownloadURLRequest{
		Key: key,
	})
	if err != nil {
		slog.Error("get download url failed", "error", err)
		apierror.BadRequest(w, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}