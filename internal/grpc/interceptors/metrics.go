package interceptors

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"lavka/internal/logger"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const RequestIDMetadataKey = "x-request-id"

type grpcMetricKey struct {
	method string
	code   string
}

type grpcDurationStat struct {
	sum   float64
	count uint64
}

// GRPCMetricsRegistry собирает Prometheus-метрики по gRPC-вызовам микросервиса (хиты, ошибки, тайминги).
type GRPCMetricsRegistry struct {
	serviceName string
	mu          sync.RWMutex
	requests    map[grpcMetricKey]uint64
	durations   map[string]grpcDurationStat
}

func NewGRPCMetricsRegistry(serviceName string) *GRPCMetricsRegistry {
	return &GRPCMetricsRegistry{
		serviceName: serviceName,
		requests:    make(map[grpcMetricKey]uint64),
		durations:   make(map[string]grpcDurationStat),
	}
}

// UnaryServerInterceptor извлекает x-request-id, логирует вызов и записывает Prometheus-метрики.
func (m *GRPCMetricsRegistry) UnaryServerInterceptor(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()

		reqID := ""
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if vals := md.Get(RequestIDMetadataKey); len(vals) > 0 {
				reqID = vals[0]
			}
		}
		if reqID == "" {
			reqID = uuid.NewString()
		}
		ctx = logger.WithRequestID(ctx, reqID)

		resp, err := handler(ctx, req)
		dur := time.Since(start)

		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}

		m.mu.Lock()
		m.requests[grpcMetricKey{method: info.FullMethod, code: code.String()}]++
		st := m.durations[info.FullMethod]
		st.sum += dur.Seconds()
		st.count++
		m.durations[info.FullMethod] = st
		m.mu.Unlock()

		reqLog := logger.FromContext(ctx, log)
		if err != nil {
			reqLog.Warn("grpc request finished with error",
				slog.String("service", m.serviceName),
				slog.String("method", info.FullMethod),
				slog.String("code", code.String()),
				slog.Duration("duration", dur),
				slog.String("error", err.Error()),
			)
		} else {
			reqLog.Info("grpc request",
				slog.String("service", m.serviceName),
				slog.String("method", info.FullMethod),
				slog.String("code", code.String()),
				slog.Duration("duration", dur),
			)
		}

		return resp, err
	}
}

// OutgoingRequestIDContext прокидывает request_id из текущего контекста в gRPC outgoing metadata.
func OutgoingRequestIDContext(ctx context.Context) context.Context {
	if reqID := logger.RequestIDFromContext(ctx); reqID != "" {
		return metadata.AppendToOutgoingContext(ctx, RequestIDMetadataKey, reqID)
	}
	return ctx
}

// ServeHTTP отдаёт метрики в формате Prometheus на /metrics.
func (m *GRPCMetricsRegistry) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	var b strings.Builder
	b.WriteString("# HELP lavka_grpc_requests_total Total gRPC requests by service, method and status code.\n")
	b.WriteString("# TYPE lavka_grpc_requests_total counter\n")

	keys := make([]grpcMetricKey, 0, len(m.requests))
	for k := range m.requests {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].method != keys[j].method {
			return keys[i].method < keys[j].method
		}
		return keys[i].code < keys[j].code
	})

	for _, k := range keys {
		fmt.Fprintf(&b, "lavka_grpc_requests_total{service=%q,method=%q,code=%q} %d\n",
			m.serviceName, k.method, k.code, m.requests[k])
	}

	b.WriteString("# HELP lavka_grpc_errors_total Total failed gRPC requests.\n")
	b.WriteString("# TYPE lavka_grpc_errors_total counter\n")
	for _, k := range keys {
		if k.code != codes.OK.String() {
			fmt.Fprintf(&b, "lavka_grpc_errors_total{service=%q,method=%q,code=%q} %d\n",
				m.serviceName, k.method, k.code, m.requests[k])
		}
	}

	b.WriteString("# HELP lavka_grpc_request_duration_seconds gRPC request latency sum and count.\n")
	b.WriteString("# TYPE lavka_grpc_request_duration_seconds summary\n")
	durKeys := make([]string, 0, len(m.durations))
	for k := range m.durations {
		durKeys = append(durKeys, k)
	}
	sort.Strings(durKeys)
	for _, method := range durKeys {
		st := m.durations[method]
		fmt.Fprintf(&b, "lavka_grpc_request_duration_seconds_sum{service=%q,method=%q} %.6f\n",
			m.serviceName, method, st.sum)
		fmt.Fprintf(&b, "lavka_grpc_request_duration_seconds_count{service=%q,method=%q} %d\n",
			m.serviceName, method, st.count)
	}

	_, _ = w.Write([]byte(b.String()))
}
