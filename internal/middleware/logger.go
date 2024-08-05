package middleware

import (
	"net/http"
)

// Logger middleware logs the details of incoming requests.
func LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := GetRequestIDFromContext(r.Context())
		GetLogger().Debug(requestID, "Incoming request: %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
