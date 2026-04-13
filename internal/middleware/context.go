package middleware

import (
	"context"
	"net/http"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

type contextKey string

const RequestIDKey contextKey = "request_id"

// InjectRequestID pulls the request ID from chi middleware
// and injects it into the context so service layer can use it.
func InjectRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := chimiddleware.GetReqID(r.Context())
		ctx := context.WithValue(r.Context(), RequestIDKey, reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID retrieves the request ID from context.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return "unknown"
}