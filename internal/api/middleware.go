package api

import (
	"net/http"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code                    // ghi nhớ lại
	r.ResponseWriter.WriteHeader(code) // rồi chuyển tiếp cho bản gốc
}

func (h *Handler) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		h.logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", float64(time.Since(start).Microseconds())/1000,
		)
	})
}

func (h *Handler) recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				h.logger.Error("panic",
					"method", r.Method,
					"path", r.URL.Path,
					"panic", rec,
				)
				writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "lỗi hệ thống"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
