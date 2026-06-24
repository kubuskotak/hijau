package server

import (
	"log"
	"net/http"
	"time"
)

// flushRecorder captures the status code while forwarding http.Flusher (and
// Unwrap), so SSE streaming still works behind the logging middleware —
// espresso's stream handler does a direct w.(http.Flusher) assertion, which a
// non-flushing status wrapper would defeat.
type flushRecorder struct {
	http.ResponseWriter
	status int
}

func (r *flushRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *flushRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *flushRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// loggingMiddleware logs each request and preserves streaming support.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		rec := &flushRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, req)
		log.Printf("%s %s %d %s", req.Method, req.URL.Path, rec.status, time.Since(start).Round(time.Millisecond))
	})
}
