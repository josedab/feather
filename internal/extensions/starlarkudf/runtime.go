package starlarkudf

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

// UDFStatus indicates the lifecycle state of a UDF.
type UDFStatus string

const (
	StatusActive     UDFStatus = "active"
	StatusDisabled   UDFStatus = "disabled"
	StatusError      UDFStatus = "error"
)

// UDFType classifies the UDF execution backend.
type UDFType string

const (
	TypeExpression UDFType = "expression"  // inline expression evaluator
	TypeSidecar    UDFType = "sidecar"     // gRPC sidecar for full Python
)

// UDF represents a registered user-defined function.
type UDF struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Expression  string            `json:"expression"`
	Type        UDFType           `json:"type"`
	Version     int               `json:"version"`
	InputSchema map[string]string `json:"input_schema"`
	OutputType  string            `json:"output_type"`
	Status      UDFStatus         `json:"status"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// ExecutionResult captures the outcome of running a UDF.
type ExecutionResult struct {
	UDFID      string      `json:"udf_id"`
	Success    bool        `json:"success"`
	Value      interface{} `json:"value"`
	DurationUs int64       `json:"duration_us"`
	Error      string      `json:"error,omitempty"`
}

// RegistryConfig configures the UDF registry.
type RegistryConfig struct {
	MaxUDFs          int           `json:"max_udfs"`
	MaxExprLength    int           `json:"max_expr_length"`
	ExecutionTimeout time.Duration `json:"execution_timeout"`
	EnableHotReload  bool          `json:"enable_hot_reload"`
}

// DefaultRegistryConfig returns sensible defaults.
func DefaultRegistryConfig() RegistryConfig {
	return RegistryConfig{
		MaxUDFs:          1000,
		MaxExprLength:    4096,
		ExecutionTimeout: 5 * time.Millisecond,
		EnableHotReload:  true,
	}
}

// Registry manages UDF registration, versioning, and execution.
type Registry struct {
	mu         sync.RWMutex
	config     RegistryConfig
	udfs       map[string]*UDF
	execCount  int64
	execErrors int64
	totalUs    int64
}

// NewRegistry creates a new UDF registry.
func NewRegistry(cfg RegistryConfig) *Registry {
	if cfg.MaxUDFs <= 0 {
		cfg.MaxUDFs = 1000
	}
	if cfg.MaxExprLength <= 0 {
		cfg.MaxExprLength = 4096
	}
	return &Registry{
		config: cfg,
		udfs:   make(map[string]*UDF),
	}
}

// Register adds or updates a UDF. If the UDF exists, its version is incremented.
func (r *Registry) Register(udf UDF) (*UDF, error) {
	if udf.Name == "" {
		return nil, fmt.Errorf("udf name must not be empty")
	}
	if udf.Expression == "" && udf.Type != TypeSidecar {
		return nil, fmt.Errorf("expression must not be empty for expression UDFs")
	}
	if len(udf.Expression) > r.config.MaxExprLength {
		return nil, fmt.Errorf("expression exceeds max length (%d > %d)", len(udf.Expression), r.config.MaxExprLength)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.udfs) >= r.config.MaxUDFs {
		if _, exists := r.udfs[udf.Name]; !exists {
			return nil, fmt.Errorf("max UDFs (%d) reached", r.config.MaxUDFs)
		}
	}

	now := time.Now()
	if existing, ok := r.udfs[udf.Name]; ok {
		udf.Version = existing.Version + 1
		udf.CreatedAt = existing.CreatedAt
	} else {
		udf.Version = 1
		udf.CreatedAt = now
	}

	if udf.ID == "" {
		udf.ID = udf.Name
	}
	if udf.Type == "" {
		udf.Type = TypeExpression
	}
	udf.Status = StatusActive
	udf.UpdatedAt = now

	r.udfs[udf.Name] = &udf
	return &udf, nil
}

// Get returns a UDF by name.
func (r *Registry) Get(name string) (*UDF, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	udf, ok := r.udfs[name]
	if !ok {
		return nil, fmt.Errorf("udf %q not found", name)
	}
	cp := *udf
	return &cp, nil
}

// List returns all registered UDFs.
func (r *Registry) List() []*UDF {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*UDF, 0, len(r.udfs))
	for _, u := range r.udfs {
		cp := *u
		result = append(result, &cp)
	}
	return result
}

// Remove removes a UDF by name.
func (r *Registry) Remove(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.udfs[name]; !ok {
		return fmt.Errorf("udf %q not found", name)
	}
	delete(r.udfs, name)
	return nil
}

// Execute runs a UDF with the given input values.
func (r *Registry) Execute(name string, inputs map[string]interface{}) (*ExecutionResult, error) {
	r.mu.RLock()
	udf, ok := r.udfs[name]
	if !ok {
		r.mu.RUnlock()
		return nil, fmt.Errorf("udf %q not found", name)
	}
	if udf.Status != StatusActive {
		r.mu.RUnlock()
		return nil, fmt.Errorf("udf %q is not active (status: %s)", name, udf.Status)
	}
	expr := udf.Expression
	r.mu.RUnlock()

	start := time.Now()
	value, err := EvalExpression(expr, inputs)
	dur := time.Since(start)

	r.mu.Lock()
	r.execCount++
	r.totalUs += dur.Microseconds()
	if err != nil {
		r.execErrors++
	}
	r.mu.Unlock()

	result := &ExecutionResult{
		UDFID:      name,
		Success:    err == nil,
		Value:      value,
		DurationUs: dur.Microseconds(),
	}
	if err != nil {
		result.Error = err.Error()
	}
	return result, nil
}

// Stats returns registry statistics.
type RegistryStats struct {
	TotalUDFs      int     `json:"total_udfs"`
	ActiveUDFs     int     `json:"active_udfs"`
	TotalExecs     int64   `json:"total_executions"`
	TotalErrors    int64   `json:"total_errors"`
	AvgExecutionUs float64 `json:"avg_execution_us"`
}

// Stats returns aggregate statistics.
func (r *Registry) Stats() RegistryStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	active := 0
	for _, u := range r.udfs {
		if u.Status == StatusActive {
			active++
		}
	}

	avgUs := 0.0
	if r.execCount > 0 {
		avgUs = float64(r.totalUs) / float64(r.execCount)
	}

	return RegistryStats{
		TotalUDFs:      len(r.udfs),
		ActiveUDFs:     active,
		TotalExecs:     r.execCount,
		TotalErrors:    r.execErrors,
		AvgExecutionUs: avgUs,
	}
}

// EvalExpression evaluates a Python-like expression with variable substitution.
// Supports: arithmetic (+, -, *, /), comparison (>, <, ==, !=, >=, <=),
// conditionals (if/else), string ops (upper, lower, len), and math functions
// (abs, min, max, round, sqrt, log).
func EvalExpression(expr string, vars map[string]interface{}) (interface{}, error) {
	// Substitute variables
	resolved := expr
	for k, v := range vars {
		resolved = strings.ReplaceAll(resolved, k, formatValue(v))
	}

	resolved = strings.TrimSpace(resolved)
	if resolved == "" {
		return nil, fmt.Errorf("empty expression")
	}

	return evalSimple(resolved)
}

// evalSimple evaluates a simple expression supporting basic arithmetic, comparisons,
// function calls, and ternary expressions.
func evalSimple(expr string) (interface{}, error) {
	expr = strings.TrimSpace(expr)

	// Handle ternary: <value> if <cond> else <other>
	if idx := strings.Index(expr, " if "); idx > 0 {
		if elseIdx := strings.Index(expr, " else "); elseIdx > idx {
			trueVal := strings.TrimSpace(expr[:idx])
			cond := strings.TrimSpace(expr[idx+4 : elseIdx])
			falseVal := strings.TrimSpace(expr[elseIdx+6:])

			condResult, err := evalSimple(cond)
			if err != nil {
				return nil, fmt.Errorf("evaluating condition: %w", err)
			}
			if isTruthy(condResult) {
				return evalSimple(trueVal)
			}
			return evalSimple(falseVal)
		}
	}

	// Handle function calls
	for _, fn := range []string{"abs", "min", "max", "round", "sqrt", "log", "upper", "lower", "len", "float", "int", "str"} {
		prefix := fn + "("
		if strings.HasPrefix(expr, prefix) && strings.HasSuffix(expr, ")") {
			arg := strings.TrimSpace(expr[len(prefix) : len(expr)-1])
			return evalBuiltinFunc(fn, arg)
		}
	}

	// Handle comparison operators
	for _, op := range []string{">=", "<=", "!=", "==", ">", "<"} {
		if idx := strings.Index(expr, op); idx > 0 {
			left, err := evalSimple(expr[:idx])
			if err != nil {
				return nil, err
			}
			right, err := evalSimple(expr[idx+len(op):])
			if err != nil {
				return nil, err
			}
			return evalComparison(left, right, op)
		}
	}

	// Handle arithmetic: + and - (lowest precedence)
	// Find the last + or - not inside parentheses
	depth := 0
	lastAddSub := -1
	for i := len(expr) - 1; i >= 0; i-- {
		switch expr[i] {
		case ')':
			depth++
		case '(':
			depth--
		case '+', '-':
			if depth == 0 && i > 0 {
				lastAddSub = i
			}
		}
		if lastAddSub > 0 {
			break
		}
	}
	if lastAddSub > 0 {
		left, err := evalSimple(expr[:lastAddSub])
		if err != nil {
			return nil, err
		}
		right, err := evalSimple(expr[lastAddSub+1:])
		if err != nil {
			return nil, err
		}
		lf, lok := toFloat(left)
		rf, rok := toFloat(right)
		if lok && rok {
			if expr[lastAddSub] == '+' {
				return lf + rf, nil
			}
			return lf - rf, nil
		}
		if expr[lastAddSub] == '+' {
			return fmt.Sprintf("%v%v", left, right), nil
		}
	}

	// Handle * and /
	depth = 0
	lastMulDiv := -1
	for i := len(expr) - 1; i >= 0; i-- {
		switch expr[i] {
		case ')':
			depth++
		case '(':
			depth--
		case '*', '/':
			if depth == 0 && i > 0 {
				lastMulDiv = i
			}
		}
		if lastMulDiv > 0 {
			break
		}
	}
	if lastMulDiv > 0 {
		left, err := evalSimple(expr[:lastMulDiv])
		if err != nil {
			return nil, err
		}
		right, err := evalSimple(expr[lastMulDiv+1:])
		if err != nil {
			return nil, err
		}
		lf, lok := toFloat(left)
		rf, rok := toFloat(right)
		if !lok || !rok {
			return nil, fmt.Errorf("non-numeric operand for %c", expr[lastMulDiv])
		}
		if expr[lastMulDiv] == '*' {
			return lf * rf, nil
		}
		if rf == 0 {
			return nil, fmt.Errorf("division by zero")
		}
		return lf / rf, nil
	}

	// Handle parenthesized expressions
	if strings.HasPrefix(expr, "(") && strings.HasSuffix(expr, ")") {
		return evalSimple(expr[1 : len(expr)-1])
	}

	// Handle string literals
	if (strings.HasPrefix(expr, "'") && strings.HasSuffix(expr, "'")) ||
		(strings.HasPrefix(expr, "\"") && strings.HasSuffix(expr, "\"")) {
		return expr[1 : len(expr)-1], nil
	}

	// Handle booleans
	if expr == "True" || expr == "true" {
		return true, nil
	}
	if expr == "False" || expr == "false" {
		return false, nil
	}
	if expr == "None" || expr == "null" {
		return nil, nil
	}

	// Handle numbers
	if f, err := strconv.ParseFloat(expr, 64); err == nil {
		return f, nil
	}

	return expr, nil
}

func evalBuiltinFunc(fn, arg string) (interface{}, error) {
	val, err := evalSimple(arg)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fn, err)
	}

	switch fn {
	case "abs":
		f, ok := toFloat(val)
		if !ok {
			return nil, fmt.Errorf("abs: non-numeric argument")
		}
		return math.Abs(f), nil
	case "sqrt":
		f, ok := toFloat(val)
		if !ok {
			return nil, fmt.Errorf("sqrt: non-numeric argument")
		}
		return math.Sqrt(f), nil
	case "log":
		f, ok := toFloat(val)
		if !ok {
			return nil, fmt.Errorf("log: non-numeric argument")
		}
		return math.Log(f), nil
	case "round":
		f, ok := toFloat(val)
		if !ok {
			return nil, fmt.Errorf("round: non-numeric argument")
		}
		return math.Round(f), nil
	case "float":
		f, ok := toFloat(val)
		if !ok {
			return nil, fmt.Errorf("float: cannot convert to float")
		}
		return f, nil
	case "int":
		f, ok := toFloat(val)
		if !ok {
			return nil, fmt.Errorf("int: cannot convert to int")
		}
		return float64(int64(f)), nil
	case "str":
		return fmt.Sprintf("%v", val), nil
	case "len":
		s, ok := val.(string)
		if !ok {
			return nil, fmt.Errorf("len: argument must be a string")
		}
		return float64(len(s)), nil
	case "upper":
		s, ok := val.(string)
		if !ok {
			return nil, fmt.Errorf("upper: argument must be a string")
		}
		return strings.ToUpper(s), nil
	case "lower":
		s, ok := val.(string)
		if !ok {
			return nil, fmt.Errorf("lower: argument must be a string")
		}
		return strings.ToLower(s), nil
	case "min", "max":
		// For single arg, just return the value
		f, ok := toFloat(val)
		if !ok {
			return nil, fmt.Errorf("%s: non-numeric argument", fn)
		}
		return f, nil
	}

	return nil, fmt.Errorf("unknown function %q", fn)
}

func evalComparison(left, right interface{}, op string) (interface{}, error) {
	lf, lok := toFloat(left)
	rf, rok := toFloat(right)

	if lok && rok {
		switch op {
		case ">":
			return lf > rf, nil
		case "<":
			return lf < rf, nil
		case ">=":
			return lf >= rf, nil
		case "<=":
			return lf <= rf, nil
		case "==":
			return lf == rf, nil
		case "!=":
			return lf != rf, nil
		}
	}

	// String comparison
	ls := fmt.Sprintf("%v", left)
	rs := fmt.Sprintf("%v", right)
	switch op {
	case "==":
		return ls == rs, nil
	case "!=":
		return ls != rs, nil
	}

	return nil, fmt.Errorf("cannot compare %v %s %v", left, op, right)
}

func isTruthy(v interface{}) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case float64:
		return val != 0
	case string:
		return val != ""
	default:
		return true
	}
}

func toFloat(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case bool:
		if val {
			return 1.0, true
		}
		return 0.0, true
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f, true
		}
		return 0, false
	}
	return 0, false
}

func formatValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return "'" + val + "'"
	case float64:
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// SidecarConfig holds configuration for the gRPC Python sidecar.
type SidecarConfig struct {
	Address       string        `json:"address"`
	TimeoutMs     int           `json:"timeout_ms"`
	MaxConcurrent int           `json:"max_concurrent"`
	Enabled       bool          `json:"enabled"`
}

// DefaultSidecarConfig returns defaults for the Python sidecar.
func DefaultSidecarConfig() SidecarConfig {
	return SidecarConfig{
		Address:       "localhost:50060",
		TimeoutMs:     5000,
		MaxConcurrent: 16,
		Enabled:       false,
	}
}
