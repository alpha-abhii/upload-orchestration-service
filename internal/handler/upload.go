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