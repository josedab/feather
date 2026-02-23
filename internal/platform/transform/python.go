package transform

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

// TypePython performs Python-based transforms.
const TypePython Type = "python"

const (
	defaultPythonBinary  = "python3"
	defaultMaxWorkers    = 4
	defaultWorkerTimeout = 30 * time.Second
)

// pythonRequest is the JSON protocol message sent to the Python subprocess.
type pythonRequest struct {
	ID         string                 `json:"id"`
	Code       string                 `json:"code"`
	Inputs     map[string]interface{} `json:"inputs"`
	EntryPoint string                 `json:"entry_point"`
}

// pythonResponse is the JSON protocol message received from the Python subprocess.
type pythonResponse struct {
	ID     string      `json:"id"`
	Result interface{} `json:"result"`
	Error  string      `json:"error"`
}

// pythonWorker represents a single Python subprocess worker.
type pythonWorker struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	mu     sync.Mutex
	alive  bool
}

// PythonExecutor executes Python transforms via subprocess workers.
type PythonExecutor struct {
	pythonBin  string
	maxWorkers int
	timeout    time.Duration
	hotReload  bool

	mu      sync.RWMutex
	workers []*pythonWorker
	pool    chan *pythonWorker

	// codeHash tracks source code per transform for hot-reload.
	codeHash map[string]string
}

// PythonExecutorOption configures a PythonExecutor.
type PythonExecutorOption func(*PythonExecutor)

// WithPythonBinary sets the Python interpreter path.
func WithPythonBinary(bin string) PythonExecutorOption {
	return func(pe *PythonExecutor) { pe.pythonBin = bin }
}

// WithMaxWorkers sets the worker pool size.
func WithMaxWorkers(n int) PythonExecutorOption {
	return func(pe *PythonExecutor) {
		if n > 0 {
			pe.maxWorkers = n
		}
	}
}

// WithTimeout sets the per-execution timeout.
func WithTimeout(d time.Duration) PythonExecutorOption {
	return func(pe *PythonExecutor) {
		if d > 0 {
			pe.timeout = d
		}
	}
}

// WithHotReload enables automatic worker restart on source code changes.
func WithHotReload(enabled bool) PythonExecutorOption {
	return func(pe *PythonExecutor) { pe.hotReload = enabled }
}

// NewPythonExecutor creates a PythonExecutor with the given options.
func NewPythonExecutor(opts ...PythonExecutorOption) *PythonExecutor {
	pe := &PythonExecutor{
		pythonBin:  defaultPythonBinary,
		maxWorkers: defaultMaxWorkers,
		timeout:    defaultWorkerTimeout,
		hotReload:  false,
		codeHash:   make(map[string]string),
	}
	for _, opt := range opts {
		opt(pe)
	}
	return pe
}

// NewPythonExecutorFromConfig creates a PythonExecutor from a transform Config map.
func NewPythonExecutorFromConfig(cfg map[string]interface{}) *PythonExecutor {
	var opts []PythonExecutorOption

	if v, ok := cfg["python_version"].(string); ok && v != "" {
		opts = append(opts, WithPythonBinary("python"+v))
	}
	if v, ok := cfg["timeout"].(string); ok {
		if d, err := time.ParseDuration(v); err == nil {
			opts = append(opts, WithTimeout(d))
		}
	}
	if v, ok := cfg["max_workers"].(float64); ok {
		opts = append(opts, WithMaxWorkers(int(v)))
	}
	if v, ok := cfg["max_workers"].(int); ok {
		opts = append(opts, WithMaxWorkers(v))
	}
	if v, ok := cfg["hot_reload"].(bool); ok {
		opts = append(opts, WithHotReload(v))
	}

	return NewPythonExecutor(opts...)
}

// Validate ensures the Python transform is properly configured.
func (pe *PythonExecutor) Validate(t *Transform) error {
	if t.Expression == "" {
		return fmt.Errorf("validating python transform %q: %w", t.Name, ErrInvalidExpression)
	}
	if t.Type != TypePython {
		return fmt.Errorf("validating python transform %q: expected type %q, got %q", t.Name, TypePython, t.Type)
	}
	if len(t.Inputs) == 0 {
		return fmt.Errorf("validating python transform %q: at least one input is required", t.Name)
	}
	if t.Output == "" {
		return fmt.Errorf("validating python transform %q: output name is required", t.Name)
	}
	return nil
}

// Execute runs a Python transform by sending the code and inputs to a subprocess
// worker and reading back the JSON result.
func (pe *PythonExecutor) Execute(ctx context.Context, t *Transform, inputs map[string]interface{}) (interface{}, error) {
	if err := pe.Validate(t); err != nil {
		return nil, err
	}

	timeout := pe.timeout
	if v, ok := t.Config["timeout"].(string); ok {
		if d, err := time.ParseDuration(v); err == nil {
			timeout = d
		}
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Hot-reload: restart workers if source code changed.
	if pe.hotReload {
		pe.maybeReload(t)
	}

	if err := pe.ensurePool(); err != nil {
		return nil, fmt.Errorf("starting python workers: %w", err)
	}

	w, err := pe.acquireWorker(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquiring python worker: %w", err)
	}
	defer pe.releaseWorker(w)

	entryPoint := "transform"
	if ep, ok := t.Config["entry_point"].(string); ok && ep != "" {
		entryPoint = ep
	}

	req := pythonRequest{
		ID:         fmt.Sprintf("%s-%d", t.Name, time.Now().UnixNano()),
		Code:       t.Expression,
		Inputs:     inputs,
		EntryPoint: entryPoint,
	}

	resp, err := pe.sendRequest(ctx, w, &req)
	if err != nil {
		// Mark worker as dead so it gets replaced.
		w.mu.Lock()
		w.alive = false
		w.mu.Unlock()
		return nil, fmt.Errorf("executing python transform %q: %w", t.Name, err)
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("python transform %q returned error: %s", t.Name, resp.Error)
	}

	return resp.Result, nil
}

// Start initializes the worker pool. Safe to call multiple times.
func (pe *PythonExecutor) Start() error {
	return pe.ensurePool()
}

// Stop shuts down all Python subprocess workers.
func (pe *PythonExecutor) Stop() {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	for _, w := range pe.workers {
		pe.killWorker(w)
	}
	pe.workers = nil
	pe.pool = nil
}

// WorkerCount returns the number of active workers.
func (pe *PythonExecutor) WorkerCount() int {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	count := 0
	for _, w := range pe.workers {
		w.mu.Lock()
		if w.alive {
			count++
		}
		w.mu.Unlock()
	}
	return count
}

// ensurePool creates the worker pool if it doesn't already exist.
func (pe *PythonExecutor) ensurePool() error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	if pe.pool != nil {
		return nil
	}

	pe.pool = make(chan *pythonWorker, pe.maxWorkers)
	pe.workers = make([]*pythonWorker, 0, pe.maxWorkers)

	for i := 0; i < pe.maxWorkers; i++ {
		w, err := pe.spawnWorker()
		if err != nil {
			// Clean up already-started workers.
			for _, started := range pe.workers {
				pe.killWorker(started)
			}
			pe.workers = nil
			pe.pool = nil
			return fmt.Errorf("spawning python worker %d: %w", i, err)
		}
		pe.workers = append(pe.workers, w)
		pe.pool <- w
	}

	return nil
}

// spawnWorker starts a new Python subprocess that reads JSON requests from stdin
// and writes JSON responses to stdout.
func (pe *PythonExecutor) spawnWorker() (*pythonWorker, error) {
	// The Python wrapper script reads JSON lines from stdin, executes the code,
	// and writes JSON responses to stdout.
	wrapper := `
import sys, json, traceback

def main():
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        try:
            req = json.loads(line)
            code = req.get("code", "")
            inputs = req.get("inputs", {})
            entry_point = req.get("entry_point", "transform")
            req_id = req.get("id", "")

            namespace = {}
            exec(code, namespace)
            func = namespace.get(entry_point)
            if func is None:
                resp = {"id": req_id, "error": f"entry point '{entry_point}' not found", "result": None}
            else:
                result = func(inputs)
                resp = {"id": req_id, "result": result, "error": ""}
        except Exception as e:
            resp = {"id": req.get("id", ""), "error": f"{type(e).__name__}: {e}", "result": None}
        sys.stdout.write(json.dumps(resp) + "\n")
        sys.stdout.flush()

if __name__ == "__main__":
    main()
`

	cmd := exec.Command(pe.pythonBin, "-u", "-c", wrapper) //nolint:gosec // pythonBin is from trusted configuration.
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("starting python process: %w", err)
	}

	return &pythonWorker{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReader(stdout),
		alive:  true,
	}, nil
}

// sendRequest sends a JSON request to a worker and reads the JSON response.
func (pe *PythonExecutor) sendRequest(ctx context.Context, w *pythonWorker, req *pythonRequest) (*pythonResponse, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.alive {
		return nil, fmt.Errorf("worker is not alive")
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}
	data = append(data, '\n')

	type result struct {
		resp *pythonResponse
		err  error
	}
	ch := make(chan result, 1)

	go func() {
		if _, werr := w.stdin.Write(data); werr != nil {
			ch <- result{err: fmt.Errorf("writing to worker stdin: %w", werr)}
			return
		}
		line, rerr := w.stdout.ReadBytes('\n')
		if rerr != nil {
			ch <- result{err: fmt.Errorf("reading from worker stdout: %w", rerr)}
			return
		}
		var resp pythonResponse
		if jerr := json.Unmarshal(line, &resp); jerr != nil {
			ch <- result{err: fmt.Errorf("unmarshaling response: %w", jerr)}
			return
		}
		ch <- result{resp: &resp}
	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("python execution canceled: %w", ctx.Err())
	case r := <-ch:
		return r.resp, r.err
	}
}

// acquireWorker obtains a worker from the pool, respecting context cancellation.
func (pe *PythonExecutor) acquireWorker(ctx context.Context) (*pythonWorker, error) {
	pe.mu.RLock()
	pool := pe.pool
	pe.mu.RUnlock()

	if pool == nil {
		return nil, fmt.Errorf("worker pool not initialized")
	}

	select {
	case w := <-pool:
		w.mu.Lock()
		alive := w.alive
		w.mu.Unlock()
		if !alive {
			// Replace dead worker.
			nw, err := pe.spawnWorker()
			if err != nil {
				return nil, fmt.Errorf("replacing dead worker: %w", err)
			}
			return nw, nil
		}
		return w, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("acquiring worker: %w", ctx.Err())
	}
}

// releaseWorker returns a worker to the pool or replaces it if dead.
func (pe *PythonExecutor) releaseWorker(w *pythonWorker) {
	pe.mu.RLock()
	pool := pe.pool
	pe.mu.RUnlock()

	if pool == nil {
		return
	}

	w.mu.Lock()
	alive := w.alive
	w.mu.Unlock()

	if !alive {
		pe.killWorker(w)
		nw, err := pe.spawnWorker()
		if err != nil {
			return
		}
		w = nw
	}

	select {
	case pool <- w:
	default:
		pe.killWorker(w)
	}
}

// killWorker terminates a Python subprocess.
func (pe *PythonExecutor) killWorker(w *pythonWorker) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.alive = false
	if w.stdin != nil {
		_ = w.stdin.Close()
	}
	if w.cmd != nil && w.cmd.Process != nil {
		_ = w.cmd.Process.Kill()
		w.cmd.Wait() //nolint:errcheck
	}
}

// maybeReload checks if the source code for a transform has changed and restarts
// workers if it has.
func (pe *PythonExecutor) maybeReload(t *Transform) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	prev, exists := pe.codeHash[t.Name]
	if exists && prev == t.Expression {
		return
	}
	pe.codeHash[t.Name] = t.Expression

	if !exists {
		// First time seeing this transform; no reload needed.
		return
	}

	// Restart all workers.
	pe.restartWorkersLocked()
}

// restartWorkersLocked drains the pool and spawns fresh workers.
// Caller must hold pe.mu write lock.
func (pe *PythonExecutor) restartWorkersLocked() {
	if pe.pool == nil {
		return
	}

	// Kill existing workers.
	for _, w := range pe.workers {
		pe.killWorker(w)
	}

	// Drain pool channel.
	close(pe.pool)
	pe.pool = make(chan *pythonWorker, pe.maxWorkers)
	pe.workers = make([]*pythonWorker, 0, pe.maxWorkers)

	for i := 0; i < pe.maxWorkers; i++ {
		w, err := pe.spawnWorker()
		if err != nil {
			continue
		}
		pe.workers = append(pe.workers, w)
		pe.pool <- w
	}
}
