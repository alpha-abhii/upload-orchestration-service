package middleware

import (
	"context"
	"net/http"
	"time"
)

// RouteTimeout returns a middleware that cancels the request context
// after the given duration. This prevents slow S3 calls or clients
// from holding goroutines forever under high load.
func RouteTimeout(duration time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), duration)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}