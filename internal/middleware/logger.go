package middleware

import (
	"bytes"
	"io"
	"net/http"
)

// Logger middleware logs the details of incoming requests.
func LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := GetRequestIDFromContext(r.Context())
		// Read the request body
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Unable to read body", http.StatusBadRequest)
			bodyBytes = []byte("Unable to read body")
		}
		defer r.Body.Close()

		// Convert the body to a string and print it
		bodyString := string(bodyBytes)
		GetLogger().Debug(requestID, "Incoming request from %s : %s %s %s", r.RemoteAddr, r.Method, r.URL.Path, bodyString)

		// Reset r.Body so that other parts of the program can read it again.
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		next.ServeHTTP(w, r)
	})
}
