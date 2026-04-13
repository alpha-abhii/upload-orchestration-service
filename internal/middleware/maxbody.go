package middleware

import "net/http"

// MaxBodySize rejects requests with bodies larger than the given limit.
// Without this, an attacker can send a gigabyte JSON body and exhaust memory.
func MaxBodySize(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}