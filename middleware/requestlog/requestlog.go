//go:build !solution

package requestlog

import (
	"net/http"

	"github.com/felixge/httpsnoop"
	"github.com/gofrs/uuid"
	"go.uber.org/zap"
)

func Log(l *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID, _ := uuid.NewV4()

			l.Info("request started",
				zap.String("request_id", requestID.String()),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
			)

			panicked := true
			defer func() {
				if panicked {
					l.Info("request panicked",
						zap.String("request_id", requestID.String()),
						zap.String("method", r.Method),
						zap.String("path", r.URL.Path),
					)
					panic(recover())
				}
			}()

			metrics := httpsnoop.CaptureMetrics(next, w, r)
			panicked = false

			l.Info("request finished",
				zap.String("request_id", requestID.String()),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Duration("duration", metrics.Duration),
				zap.Int("status_code", metrics.Code),
			)
		})
	}
}
