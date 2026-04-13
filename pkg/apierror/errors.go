package apierror

import (
	"encoding/json"
	"net/http"
)

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func Write(w http.ResponseWriter, statusCode int, code string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]interface{}{
		"error": APIError{Code: code, Message: message},
	}

	json.NewEncoder(w).Encode(response)
}

func BadRequest(w http.ResponseWriter, message string) {
	Write(w, http.StatusBadRequest, "BAD_REQUEST", message)
}

func InternalError(w http.ResponseWriter) {
	Write(w, http.StatusInternalServerError, "INTERNAL_ERROR", "something went wrong, please try again")
}