package middleware

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const CorrelationIDKey = "x-correlation-id"

type bodyLogWriter struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

func (w *bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *bodyLogWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func HTTPLogger(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			correlationID := r.Header.Get(CorrelationIDKey)
			if correlationID == "" {
				correlationID = uuid.NewString()
			}
			w.Header().Set(CorrelationIDKey, correlationID)

			var bodyBytes []byte
			if r.Body != nil {
				bodyBytes, _ = io.ReadAll(r.Body)
			}
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

			blw := &bodyLogWriter{
				ResponseWriter: w,
				body:           bytes.NewBufferString(""),
				statusCode:     http.StatusOK,
			}

			enriched := logger.With(
				zap.String("correlation_id", correlationID),
				zap.String("transport", "http"),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("remote_addr", r.RemoteAddr),
			)
			ctx := InjectLogger(r.Context(), enriched)
			ctx = context.WithValue(ctx, CorrelationIDKey, correlationID)
			r = r.WithContext(ctx)

			enriched.Info("http request",
				zap.ByteString("request_body", sanitizeBody(bodyBytes)),
			)

			next.ServeHTTP(blw, r)

			enriched.Info("http response",
				zap.Int("status", blw.statusCode),
				zap.Duration("latency", time.Since(start)),
				zap.ByteString("response_body", blw.body.Bytes()),
			)
		})
	}
}

func UnaryLogger(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()

		correlationID := extractGRPCCorrelationID(ctx)
		if correlationID == "" {
			correlationID = uuid.NewString()
		}

		enriched := logger.With(
			zap.String("correlation_id", correlationID),
			zap.String("transport", "grpc"),
			zap.String("method", info.FullMethod),
		)
		ctx = InjectLogger(ctx, enriched)
		ctx = context.WithValue(ctx, CorrelationIDKey, correlationID)

		enriched.Info("grpc request",
			zap.Any("request", req),
		)

		resp, err := handler(ctx, req)

		code := codes.OK
		if s, ok := status.FromError(err); ok {
			code = s.Code()
		}

		enriched.Info("grpc response",
			zap.String("code", code.String()),
			zap.Duration("latency", time.Since(start)),
			zap.Any("response", resp),
			zap.Error(err),
		)

		return resp, err
	}
}

func UnaryRecovery(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("panic recovered",
					zap.Any("panic", r),
					zap.String("stack", string(debug.Stack())),
				)
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}

func extractGRPCCorrelationID(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	values := md.Get(CorrelationIDKey)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func sanitizeBody(body []byte) []byte {
	result := bytes.ReplaceAll(body, []byte(`"password"`), []byte(`"password":"***"`))
	return result
}
