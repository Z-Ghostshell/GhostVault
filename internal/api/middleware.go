package api

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/z-ghostshell/ghostvault/internal/config"
)

type ctxKey string

const requestIDKey ctxKey = "requestID"

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-ID")
		if rid == "" {
			rid = uuid.NewString()
		}
		ctx := context.WithValue(r.Context(), requestIDKey, rid)
		w.Header().Set("X-Request-ID", rid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func MaxBytes(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, n)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// MaxBytesTuning reads the body limit from the current runtime tuning (hot-reload).
func MaxBytesTuning(ts *config.TuningState) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, ts.Current().MaxBodyBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				WriteProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "panic recovered", "internal")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func Timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	code   int
	header bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.header {
		s.code = code
		s.header = true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.header {
		s.code = http.StatusOK
		s.header = true
	}
	return s.ResponseWriter.Write(b)
}

// AccessLog emits one line per request (method, path, status, duration, request id, remote addr).
func AccessLog(enabled bool) func(http.Handler) http.Handler {
	if !enabled {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, code: http.StatusOK}
			next.ServeHTTP(rec, r)
			rid, _ := r.Context().Value(requestIDKey).(string)
			log.Printf("http %s %s %d %s remote=%q request_id=%s",
				r.Method, r.URL.Path, rec.code, time.Since(start).Round(time.Millisecond), r.RemoteAddr, rid)
		})
	}
}

// AccessLogTuning enables access logging per request based on current runtime tuning.
func AccessLogTuning(ts *config.TuningState) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !ts.Current().HTTPAccessLog {
				next.ServeHTTP(w, r)
				return
			}
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, code: http.StatusOK}
			next.ServeHTTP(rec, r)
			rid, _ := r.Context().Value(requestIDKey).(string)
			log.Printf("http %s %s %d %s remote=%q request_id=%s",
				r.Method, r.URL.Path, rec.code, time.Since(start).Round(time.Millisecond), r.RemoteAddr, rid)
		})
	}
}
