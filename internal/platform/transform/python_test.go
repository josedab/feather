package transform

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/feather-store/feather/internal/core/domain"
)

func validPythonTransform() *Transform {
	return &Transform{
		Name:       "test_transform",
		Type:       TypePython,
		Expression: "def transform(inputs):\n    return inputs['a'] + inputs['b']",
		Inputs:     []string{"a", "b"},
		Output:     "result",
		OutputType: domain.DataTypeFloat64,
		Config:     map[string]interface{}{},
		Enabled:    true,
		Mode:       ModeOnRead,
	}
}

func TestPythonExecutor_Validate(t *testing.T) {
	tests := []struct {
		name      string
		transform *Transform
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid transform",
			transform: validPythonTransform(),
			wantErr:   false,
		},
		{
			name: "empty expression",
			transform: &Transform{
				Name:   "empty_expr",
				Type:   TypePython,
				Inputs: []string{"a"},
				Output: "result",
			},
			wantErr: true,
			errMsg:  "invalid expression",
		},
		{
			name: "wrong type",
			transform: &Transform{
				Name:       "wrong_type",
				Type:       TypeArithmetic,
				Expression: "def transform(inputs): return 1",
				Inputs:     []string{"a"},
				Output:     "result",
			},
			wantErr: true,
			errMsg:  "expected type",
		},
		{
			name: "no inputs",
			transform: &Transform{
				Name:       "no_inputs",
				Type:       TypePython,
				Expression: "def transform(inputs): return 1",
				Output:     "result",
			},
			wantErr: true,
			errMsg:  "at least one input",
		},
		{
			name: "no output",
			transform: &Transform{
				Name:       "no_output",
				Type:       TypePython,
				Expression: "def transform(inputs): return 1",
				Inputs:     []string{"a"},
			},
			wantErr: true,
			errMsg:  "output name is required",
		},
	}

	pe := NewPythonExecutor()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pe.Validate(tt.transform)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestPythonExecutor_NewDefaults(t *testing.T) {
	pe := NewPythonExecutor()
	if pe.pythonBin != defaultPythonBinary {
		t.Errorf("pythonBin = %q, want %q", pe.pythonBin, defaultPythonBinary)
	}
	if pe.maxWorkers != defaultMaxWorkers {
		t.Errorf("maxWorkers = %d, want %d", pe.maxWorkers, defaultMaxWorkers)
	}
	if pe.timeout != defaultWorkerTimeout {
		t.Errorf("timeout = %v, want %v", pe.timeout, defaultWorkerTimeout)
	}
	if pe.hotReload {
		t.Error("hotReload should default to false")
	}
}

func TestPythonExecutor_Options(t *testing.T) {
	pe := NewPythonExecutor(
		WithPythonBinary("/usr/bin/python3.11"),
		WithMaxWorkers(8),
		WithTimeout(10*time.Second),
		WithHotReload(true),
	)

	if pe.pythonBin != "/usr/bin/python3.11" {
		t.Errorf("pythonBin = %q, want %q", pe.pythonBin, "/usr/bin/python3.11")
	}
	if pe.maxWorkers != 8 {
		t.Errorf("maxWorkers = %d, want 8", pe.maxWorkers)
	}
	if pe.timeout != 10*time.Second {
		t.Errorf("timeout = %v, want 10s", pe.timeout)
	}
	if !pe.hotReload {
		t.Error("hotReload should be true")
	}
}

func TestPythonExecutor_OptionsIgnoreInvalid(t *testing.T) {
	pe := NewPythonExecutor(
		WithMaxWorkers(-1),
		WithTimeout(-5*time.Second),
	)
	if pe.maxWorkers != defaultMaxWorkers {
		t.Errorf("maxWorkers = %d, want default %d", pe.maxWorkers, defaultMaxWorkers)
	}
	if pe.timeout != defaultWorkerTimeout {
		t.Errorf("timeout = %v, want default %v", pe.timeout, defaultWorkerTimeout)
	}
}

func TestPythonExecutor_FromConfig(t *testing.T) {
	tests := []struct {
		name       string
		config     map[string]interface{}
		wantBin    string
		wantMax    int
		wantReload bool
	}{
		{
			name:       "empty config",
			config:     map[string]interface{}{},
			wantBin:    defaultPythonBinary,
			wantMax:    defaultMaxWorkers,
			wantReload: false,
		},
		{
			name: "full config",
			config: map[string]interface{}{
				"python_version": "3.11",
				"timeout":        "5s",
				"max_workers":    float64(16),
				"hot_reload":     true,
			},
			wantBin:    "python3.11",
			wantMax:    16,
			wantReload: true,
		},
		{
			name: "max_workers as int",
			config: map[string]interface{}{
				"max_workers": 8,
			},
			wantBin:    defaultPythonBinary,
			wantMax:    8,
			wantReload: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pe := NewPythonExecutorFromConfig(tt.config)
			if pe.pythonBin != tt.wantBin {
				t.Errorf("pythonBin = %q, want %q", pe.pythonBin, tt.wantBin)
			}
			if pe.maxWorkers != tt.wantMax {
				t.Errorf("maxWorkers = %d, want %d", pe.maxWorkers, tt.wantMax)
			}
			if pe.hotReload != tt.wantReload {
				t.Errorf("hotReload = %v, want %v", pe.hotReload, tt.wantReload)
			}
		})
	}
}

func TestPythonExecutor_ExecuteValidationFailure(t *testing.T) {
	pe := NewPythonExecutor()
	ctx := context.Background()

	// Empty expression should fail validation before reaching workers.
	_, err := pe.Execute(ctx, &Transform{
		Name:   "bad",
		Type:   TypePython,
		Inputs: []string{"a"},
		Output: "out",
	}, map[string]interface{}{"a": 1})

	if err == nil {
		t.Fatal("Execute() should fail for invalid transform")
	}
}

func TestPythonExecutor_StopIdempotent(t *testing.T) {
	pe := NewPythonExecutor()

	// Stop without Start should not panic.
	pe.Stop()
	pe.Stop()

	if pe.WorkerCount() != 0 {
		t.Errorf("WorkerCount() = %d, want 0 after Stop", pe.WorkerCount())
	}
}

func TestPythonExecutor_WorkerCountNoPool(t *testing.T) {
	pe := NewPythonExecutor()
	if pe.WorkerCount() != 0 {
		t.Errorf("WorkerCount() = %d, want 0 before Start", pe.WorkerCount())
	}
}

func TestPythonExecutor_AcquireWorkerNoPool(t *testing.T) {
	pe := NewPythonExecutor()
	ctx := context.Background()

	_, err := pe.acquireWorker(ctx)
	if err == nil {
		t.Fatal("acquireWorker() should fail when pool is nil")
	}
}

func TestPythonExecutor_AcquireWorkerContextCancelled(t *testing.T) {
	pe := NewPythonExecutor(WithMaxWorkers(1))

	// Create pool with an empty channel (no workers available).
	pe.mu.Lock()
	pe.pool = make(chan *pythonWorker, 1) // empty — no workers to acquire
	pe.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := pe.acquireWorker(ctx)
	if err == nil {
		t.Fatal("acquireWorker() should fail when context is canceled")
	}
}

func TestPythonExecutor_ReleaseWorkerNoPool(t *testing.T) {
	pe := NewPythonExecutor()
	w := &pythonWorker{alive: true}

	// Should not panic when pool is nil.
	pe.releaseWorker(w)
}

func TestPythonExecutor_KillWorker(t *testing.T) {
	w := &pythonWorker{alive: true}
	pe := NewPythonExecutor()

	pe.killWorker(w)

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.alive {
		t.Error("worker should be dead after killWorker")
	}
}

func TestPythonExecutor_MaybeReloadFirstSeen(t *testing.T) {
	pe := NewPythonExecutor(WithHotReload(true))
	tr := validPythonTransform()

	// First time — should just record, not reload.
	pe.maybeReload(tr)

	pe.mu.RLock()
	hash, exists := pe.codeHash[tr.Name]
	pe.mu.RUnlock()

	if !exists {
		t.Fatal("codeHash should be set after first maybeReload")
	}
	if hash != tr.Expression {
		t.Errorf("codeHash = %q, want %q", hash, tr.Expression)
	}
}

func TestPythonExecutor_MaybeReloadSameCode(t *testing.T) {
	pe := NewPythonExecutor(WithHotReload(true))
	tr := validPythonTransform()

	pe.maybeReload(tr)
	// Second call with same code should not trigger reload.
	pe.maybeReload(tr)

	pe.mu.RLock()
	hash := pe.codeHash[tr.Name]
	pe.mu.RUnlock()

	if hash != tr.Expression {
		t.Errorf("codeHash should remain %q", tr.Expression)
	}
}

func TestPythonExecutor_TypePythonConstant(t *testing.T) {
	if TypePython != "python" {
		t.Errorf("TypePython = %q, want %q", TypePython, "python")
	}
}

func TestPythonExecutor_SendRequestDeadWorker(t *testing.T) {
	pe := NewPythonExecutor()
	w := &pythonWorker{alive: false}

	_, err := pe.sendRequest(context.Background(), w, &pythonRequest{ID: "test"})
	if err == nil {
		t.Fatal("sendRequest() should fail for dead worker")
	}
}

func TestPythonExecutor_RestartWorkersNoPool(t *testing.T) {
	pe := NewPythonExecutor()
	pe.mu.Lock()
	pe.restartWorkersLocked() // should not panic with nil pool
	pe.mu.Unlock()
}
