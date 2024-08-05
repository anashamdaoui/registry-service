package middleware

import (
	"net/http"
)

// ErrorHandler middleware handles errors during request processing.
func ErrorHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		if status := w.Header().Get("X-Error"); status != "" {
			requestID := GetRequestIDFromContext(r.Context())
			GetLogger().Debug(requestID, "Error encountered: %s %s", r.Method, status)
		}
	})
}
