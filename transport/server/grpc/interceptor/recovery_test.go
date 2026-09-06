package interceptor

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockLogger struct {
	mu           sync.Mutex
	errorCalls   []string
	lastPayloads []logger.LogPayload
}

func (m *mockLogger) Debug(context.Context, string, logger.LogPayload) {}
func (m *mockLogger) Info(context.Context, string, logger.LogPayload)  {}
func (m *mockLogger) Warn(context.Context, string, logger.LogPayload)  {}
func (m *mockLogger) Error(ctx context.Context, msg string, payload logger.LogPayload) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorCalls = append(m.errorCalls, msg)
	m.lastPayloads = append(m.lastPayloads, payload)
}

func TestRecovery_Unary_PanicString(t *testing.T) {
	mockLog := &mockLogger{}
	interceptor := Recovery(mockLog, "test-app")

	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/PanicMethod",
	}

	handler := func(ctx context.Context, req any) (any, error) {
		panic("database connection exploded")
	}

	resp, err := interceptor(context.Background(), struct{}{}, info, handler)
	if resp != nil {
		t.Fatalf("expected nil resp, got: %v", resp)
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Internal {
		t.Fatalf("expected codes.Internal, got: %v", err)
	}

	mockLog.mu.Lock()
	defer mockLog.mu.Unlock()

	if len(mockLog.errorCalls) != 1 {
		t.Fatalf("expected 1 error log call, got %d", len(mockLog.errorCalls))
	}
	if !strings.Contains(mockLog.errorCalls[0], "/test.Service/PanicMethod") {
		t.Fatalf("expected log msg to mention method, got: %s", mockLog.errorCalls[0])
	}

	payload := mockLog.lastPayloads[0]
	errDetails, ok := payload.Error.(map[string]any)
	if !ok {
		t.Fatalf("expected Error payload to be map[string]any, got %T", payload.Error)
	}
	if errDetails["panic"] != "database connection exploded" {
		t.Fatalf("unexpected panic message: %v", errDetails["panic"])
	}
	if errDetails["file"] == "" {
		t.Fatal("expected file location to be populated in error payload")
	}
	if errDetails["stack"] == "" {
		t.Fatal("expected stack trace to be populated in error payload")
	}
}

func TestRecovery_Unary_PanicError(t *testing.T) {
	mockLog := &mockLogger{}
	interceptor := Recovery(mockLog, "test-app")

	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/PanicErrorMethod",
	}

	handler := func(ctx context.Context, req any) (any, error) {
		panic(errors.New("nil pointer dereference"))
	}

	resp, err := interceptor(context.Background(), struct{}{}, info, handler)
	if resp != nil {
		t.Fatalf("expected nil resp, got: %v", resp)
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Internal {
		t.Fatalf("expected codes.Internal, got: %v", err)
	}

	mockLog.mu.Lock()
	defer mockLog.mu.Unlock()

	if len(mockLog.errorCalls) != 1 {
		t.Fatalf("expected 1 error log call, got %d", len(mockLog.errorCalls))
	}
}

type mockServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (m *mockServerStream) Context() context.Context {
	return m.ctx
}

func TestRecoveryStream_Panic(t *testing.T) {
	mockLog := &mockLogger{}
	streamInterceptor := RecoveryStream(mockLog, "test-app")

	info := &grpc.StreamServerInfo{
		FullMethod: "/test.Service/StreamMethod",
	}

	handler := func(srv any, ss grpc.ServerStream) error {
		panic("stream pipeline crushed")
	}

	ss := &mockServerStream{ctx: context.Background()}
	err := streamInterceptor(struct{}{}, ss, info, handler)
	if err == nil {
		t.Fatal("expected stream error, got nil")
	}

	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Internal {
		t.Fatalf("expected codes.Internal, got: %v", err)
	}

	mockLog.mu.Lock()
	defer mockLog.mu.Unlock()

	if len(mockLog.errorCalls) != 1 {
		t.Fatalf("expected 1 error log call, got %d", len(mockLog.errorCalls))
	}

	payload := mockLog.lastPayloads[0]
	errDetails, ok := payload.Error.(map[string]any)
	if !ok {
		t.Fatalf("expected Error payload to be map[string]any, got %T", payload.Error)
	}
	if errDetails["panic"] != "stream pipeline crushed" {
		t.Fatalf("unexpected panic message: %v", errDetails["panic"])
	}
	if errDetails["file"] == "" {
		t.Fatal("expected file location to be populated in stream error payload")
	}
}
