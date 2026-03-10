package wasmudf

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// CompilerConfig configures the UDF compiler.
type CompilerConfig struct {
	MaxModuleSize    int64 `json:"max_module_size_bytes" yaml:"max_module_size_bytes"`
	EnableValidation bool  `json:"enable_validation" yaml:"enable_validation"`
}

// DefaultCompilerConfig returns sensible defaults.
func DefaultCompilerConfig() CompilerConfig {
	return CompilerConfig{
		MaxModuleSize:    50 * 1024 * 1024, // 50MB
		EnableValidation: true,
	}
}

// CompileRequest represents a UDF compilation request.
type CompileRequest struct {
	Name     string `json:"name"`
	Language string `json:"language"` // rust, go, assemblyscript
	Source   string `json:"source"`   // source code or path
	Version  string `json:"version"`
}

// CompileResult contains the result of a compilation.
type CompileResult struct {
	Name       string    `json:"name"`
	Language   string    `json:"language"`
	Version    string    `json:"version"`
	WasmSize   int64     `json:"wasm_size_bytes"`
	Checksum   string    `json:"checksum"`
	Status     string    `json:"status"` // "compiled", "failed"
	Error      string    `json:"error,omitempty"`
	CompiledAt time.Time `json:"compiled_at"`
}

// Compiler manages UDF compilation from source to WASM.
type Compiler struct {
	mu     sync.RWMutex
	config CompilerConfig
	cache  map[string]*CompileResult
}

// NewCompiler creates a new UDF compiler.
func NewCompiler(config CompilerConfig) *Compiler {
	return &Compiler{
		config: config,
		cache:  make(map[string]*CompileResult),
	}
}

// Compile validates and registers a WASM module.
// In production, this would shell out to rustc/tinygo/asc for actual compilation.
func (c *Compiler) Compile(req CompileRequest) (*CompileResult, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("module name is required")
	}
	if req.Language == "" {
		return nil, fmt.Errorf("language is required")
	}

	// Validate language
	validLanguage := false
	for _, lang := range SupportedLanguages() {
		if lang.Name == req.Language {
			validLanguage = true
			break
		}
	}
	if !validLanguage {
		return nil, fmt.Errorf("unsupported language: %s", req.Language)
	}

	// Generate a deterministic WASM representation from source
	wasmBytes := []byte(fmt.Sprintf("(module %s %s %s)", req.Name, req.Language, req.Source))
	if int64(len(wasmBytes)) > c.config.MaxModuleSize {
		return &CompileResult{
			Name:     req.Name,
			Language: req.Language,
			Status:   "failed",
			Error:    fmt.Sprintf("module size %d exceeds maximum %d", len(wasmBytes), c.config.MaxModuleSize),
		}, fmt.Errorf("module too large")
	}

	hash := sha256.Sum256(wasmBytes)
	result := &CompileResult{
		Name:       req.Name,
		Language:   req.Language,
		Version:    req.Version,
		WasmSize:   int64(len(wasmBytes)),
		Checksum:   fmt.Sprintf("%x", hash),
		Status:     "compiled",
		CompiledAt: time.Now(),
	}

	c.mu.Lock()
	c.cache[req.Name+"@"+req.Version] = result
	c.mu.Unlock()

	return result, nil
}

// GetCompileResult retrieves a cached compile result.
func (c *Compiler) GetCompileResult(name, version string) (*CompileResult, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result, exists := c.cache[name+"@"+version]
	if !exists {
		return nil, fmt.Errorf("compile result not found for %s@%s", name, version)
	}
	return result, nil
}

// ListCompiled returns all compiled modules.
func (c *Compiler) ListCompiled() []CompileResult {
	c.mu.RLock()
	defer c.mu.RUnlock()
	results := make([]CompileResult, 0, len(c.cache))
	for _, r := range c.cache {
		results = append(results, *r)
	}
	return results
}

// CompileAndRegister compiles a module and registers it in the runtime.
func (c *Compiler) CompileAndRegister(runtime *Runtime, req CompileRequest) (*CompileResult, error) {
	result, err := c.Compile(req)
	if err != nil {
		return nil, err
	}

	wasmBytes := []byte(fmt.Sprintf("(module %s %s %s)", req.Name, req.Language, req.Source))
	mod := Module{
		ID:       req.Name,
		Name:     req.Name,
		Language: req.Language,
		Version:  req.Version,
		WasmBytes: wasmBytes,
	}

	if err := runtime.RegisterModule(mod); err != nil {
		// If already exists, try update
		if updateErr := runtime.UpdateModule(req.Name, mod); updateErr != nil {
			return nil, fmt.Errorf("registering module: %w (update also failed: %v)", err, updateErr)
		}
	}

	return result, nil
}
