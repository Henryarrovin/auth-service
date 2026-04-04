package middleware

import (
	"context"
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

// UnaryLogger logs every gRPC request/response with correlation ID.
func UnaryLogger(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()

		correlationID := extractGRPCCorrelationID(ctx)
		if correlationID == "" {
			correlationID = uuid.NewString()
		}

		enriched := logger.With(
			zap.String("correlation_id", correlationID),
			zap.String("method", info.FullMethod),
			zap.String("transport", "grpc"),
		)
		ctx = InjectLogger(ctx, enriched)
		ctx = context.WithValue(ctx, CorrelationIDKey, correlationID)

		enriched.Info("grpc request", zap.Any("request", req))

		resp, err := handler(ctx, req)

		code := codes.OK
		if s, ok := status.FromError(err); ok {
			code = s.Code()
		}

		enriched.Info("grpc response",
			zap.String("code", code.String()),
			zap.Duration("duration", time.Since(start)),
			zap.Any("response", resp),
			zap.Error(err),
		)

		return resp, err
	}
}

// UnaryRecovery catches panics and turns them into gRPC INTERNAL errors.
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

// HTTPLogger logs every HTTP request/response with correlation ID.
func HTTPLogger(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			correlationID := r.Header.Get(CorrelationIDKey)
			if correlationID == "" {
				correlationID = uuid.NewString()
			}

			enriched := logger.With(
				zap.String("correlation_id", correlationID),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("remote_addr", r.RemoteAddr),
				zap.String("transport", "http"),
			)

			ctx := InjectLogger(r.Context(), enriched)
			ctx = context.WithValue(ctx, CorrelationIDKey, correlationID)
			r = r.WithContext(ctx)

			w.Header().Set(CorrelationIDKey, correlationID)

			enriched.Info("http request")

			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(rw, r)

			enriched.Info("http response",
				zap.Int("status", rw.statusCode),
				zap.Duration("duration", time.Since(start)),
			)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
