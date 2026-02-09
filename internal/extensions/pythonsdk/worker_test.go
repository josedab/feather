package pythonsdk

import (
	"context"
	"testing"
	"time"
)

func TestWorkerPool_StartAndStop(t *testing.T) {
	pool := NewWorkerPool(DefaultWorkerPoolConfig())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := pool.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer pool.Stop()

	workers := pool.Workers()
	if len(workers) == 0 {
		t.Fatal("expected at least one worker")
	}
}

func TestWorkerPool_Execute(t *testing.T) {
	pool := NewWorkerPool(DefaultWorkerPoolConfig())
	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer pool.Stop()

	// Wait for health check to mark worker ready
	time.Sleep(10 * time.Millisecond)
	pool.checkWorkerHealth()

	req := &TransformRequest{
		TransformID: "test-transform",
		EntryPoint:  "transform",
		Inputs:      map[string]interface{}{"x": 42},
		RequestID:   "req-1",
	}

	resp, err := pool.Execute(ctx, req)
	if err != nil {
		t.Fatalf("expected successful execution, got %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success")
	}
	if resp.WorkerID == "" {
		t.Fatal("expected worker ID in response")
	}
}

func TestWorkerPool_HotReload(t *testing.T) {
	pool := NewWorkerPool(DefaultWorkerPoolConfig())
	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer pool.Stop()

	if err := pool.HotReload(ctx, "test-transform"); err != nil {
		t.Fatalf("hot reload failed: %v", err)
	}

	// Disable hot reload and verify error
	disabledCfg := DefaultWorkerPoolConfig()
	disabledCfg.EnableHotReload = false
	pool2 := NewWorkerPool(disabledCfg)
	if err := pool2.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pool2.HotReload(ctx, "test-transform"); err == nil {
		t.Fatal("expected error when hot reload disabled")
	}
}
