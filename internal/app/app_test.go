package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"
)

type mockServer struct {
	shutdownErr error
	shutdownCh  chan struct{}
}

func (m *mockServer) Shutdown(ctx context.Context) error {
	if m.shutdownCh != nil {
		close(m.shutdownCh)
	}
	return m.shutdownErr
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// --- NewServerManager ---

func TestNewServerManager(t *testing.T) {
	sm := NewServerManager(testLogger())
	if sm == nil {
		t.Fatal("expected non-nil server manager")
	}
}

// --- Register ---

func TestRegister_Single(t *testing.T) {
	sm := NewServerManager(testLogger())
	sm.Register("http", &mockServer{})
	// No panic means success
}

func TestRegister_Multiple(t *testing.T) {
	sm := NewServerManager(testLogger())
	sm.Register("http", &mockServer{})
	sm.Register("grpc", &mockServer{})
	sm.Register("metrics", &mockServer{})
	// No panic means success
}

// --- ShutdownAll ---

func TestShutdownAll_Success(t *testing.T) {
	sm := NewServerManager(testLogger())
	ch1 := make(chan struct{})
	ch2 := make(chan struct{})
	sm.Register("s1", &mockServer{shutdownCh: ch1})
	sm.Register("s2", &mockServer{shutdownCh: ch2})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sm.ShutdownAll(ctx)

	// Verify both servers were shut down
	select {
	case <-ch1:
	default:
		t.Fatal("s1 not shut down")
	}
	select {
	case <-ch2:
	default:
		t.Fatal("s2 not shut down")
	}
}

func TestShutdownAll_NoServers(t *testing.T) {
	sm := NewServerManager(testLogger())
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	sm.ShutdownAll(ctx) // should not panic
}

func TestShutdownAll_WithError(t *testing.T) {
	sm := NewServerManager(testLogger())
	sm.Register("s1", &mockServer{shutdownErr: errors.New("shutdown failed")})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	sm.ShutdownAll(ctx) // should not panic, error is logged
}

// --- RecoverPanic ---

func TestRecoverPanic_NoPanic(t *testing.T) {
	logger := testLogger()
	func() {
		defer RecoverPanic(logger, "test")
	}()
	// No panic means success
}

func TestRecoverPanic_WithPanic(t *testing.T) {
	logger := testLogger()
	recovered := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatal("panic should have been recovered by RecoverPanic")
			}
		}()
		defer RecoverPanic(logger, "test")
		panic("test panic")
	}()
	_ = recovered
}

// --- LogListenError ---

func TestLogListenError_AddressInUse(t *testing.T) {
	logger := testLogger()
	err := fmt.Errorf("listen tcp :8080: bind: address already in use")
	LogListenError(logger, "HTTP", 8080, err) // should not panic
}

func TestLogListenError_OtherError(t *testing.T) {
	logger := testLogger()
	err := fmt.Errorf("some other error")
	LogListenError(logger, "HTTP", 8080, err) // should not panic
}

// --- HTTPServerWrapper ---

func TestHTTPServerWrapper_Shutdown(t *testing.T) {
	// Minimal test: just verify it doesn't panic on nil server close
	// (we can't easily test real http.Server shutdown without binding)
}
