package featherqlv2

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// NodeType represents the type of an AST node.
type NodeType string

const (
	NodeSelect      NodeType = "SELECT"
	NodeFrom        NodeType = "FROM"
	NodeWhere       NodeType = "WHERE"
	NodeGroupBy     NodeType = "GROUP_BY"
	NodeWindow      NodeType = "WINDOW"
	NodeAggregation NodeType = "AGGREGATION"
	NodeColumn      NodeType = "COLUMN"
	NodeLiteral     NodeType = "LITERAL"
	NodeJoin        NodeType = "JOIN"
)

// ASTNode represents a node in the abstract syntax tree.
type ASTNode struct {
	Type     NodeType   `json:"type"`
	Value    string     `json:"value,omitempty"`
	Children []*ASTNode `json:"children,omitempty"`
	Alias    string     `json:"alias,omitempty"`
}

// ParseResult represents the result of parsing a query.
type ParseResult struct {
	Query   string   `json:"query"`
	AST     *ASTNode `json:"ast"`
	Columns []string `json:"columns"`
	Sources []string `json:"sources"`
	IsValid bool     `json:"is_valid"`
	Errors  []string `json:"errors,omitempty"`
}

// ExecutionStep represents a step in an execution plan.
type ExecutionStep struct {
	ID            int     `json:"id"`
	Operation     string  `json:"operation"`
	Description   string  `json:"description"`
	EstimatedCost float64 `json:"estimated_cost"`
}

// CompiledPipeline represents a compiled query ready for execution.
type CompiledPipeline struct {
	ID         string          `json:"id"`
	Query      string          `json:"query"`
	Steps      []ExecutionStep `json:"steps"`
	OutputCols []string        `json:"output_columns"`
	CompiledAt time.Time       `json:"compiled_at"`
}

// ExecutionResult represents the output of executing a query.
type ExecutionResult struct {
	PipelineID string                   `json:"pipeline_id,omitempty"`
	Columns    []string                 `json:"columns"`
	Rows       []map[string]interface{} `json:"rows"`
	RowCount   int                      `json:"row_count"`
	ExecutedAt time.Time                `json:"executed_at"`
	DurationMs float64                  `json:"duration_ms"`
}

// EngineConfig configures the FeatherQL v2 engine.
type EngineConfig struct {
	MaxPipelines   int `json:"max_pipelines"`
	MaxQueryLength int `json:"max_query_length"`
	MaxResultRows  int `json:"max_result_rows"`
}

// DefaultEngineConfig returns sensible defaults.
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		MaxPipelines:   1000,
		MaxQueryLength: 10000,
		MaxResultRows:  10000,
	}
}

// Engine provides the FeatherQL v2 query engine.
type Engine struct {
	mu        sync.RWMutex
	config    EngineConfig
	pipelines map[string]*CompiledPipeline
}

// NewEngine creates a new FeatherQL v2 engine.
func NewEngine(config EngineConfig) *Engine {
	if config.MaxPipelines == 0 {
		config = DefaultEngineConfig()
	}
	return &Engine{
		config:    config,
		pipelines: make(map[string]*CompiledPipeline),
	}
}

// Parse parses a FeatherQL v2 query using the recursive-descent parser.
func (e *Engine) Parse(query string) ParseResult {
	query = strings.TrimSpace(query)
	result := ParseResult{
		Query:   query,
		IsValid: true,
	}

	if query == "" {
		result.IsValid = false
		result.Errors = append(result.Errors, "empty query")
		return result
	}

	if len(query) > e.config.MaxQueryLength {
		result.IsValid = false
		result.Errors = append(result.Errors, "query exceeds maximum length")
		return result
	}

	// Use the recursive-descent parser
	stmt, err := ParseQuery(query)
	if err != nil {
		result.IsValid = false
		result.Errors = append(result.Errors, err.Error())
		return result
	}

	// Populate ParseResult from the parsed statement
	for _, col := range stmt.Columns {
		name := col.Expression
		if col.Alias != "" {
			name = col.Alias
		}
		result.Columns = append(result.Columns, name)
	}
	result.Sources = append(result.Sources, stmt.From)
	for _, j := range stmt.Joins {
		result.Sources = append(result.Sources, j.Table)
	}

	// Build AST from parsed statement
	selectNode := &ASTNode{Type: NodeSelect}
	for _, col := range stmt.Columns {
		if col.IsAgg {
			selectNode.Children = append(selectNode.Children, &ASTNode{Type: NodeAggregation, Value: col.Expression, Alias: col.Alias})
		} else {
			selectNode.Children = append(selectNode.Children, &ASTNode{Type: NodeColumn, Value: col.Expression, Alias: col.Alias})
		}
	}
	fromNode := &ASTNode{Type: NodeFrom, Value: stmt.From}
	result.AST = &ASTNode{
		Type:     NodeSelect,
		Children: []*ASTNode{selectNode, fromNode},
	}

	if stmt.Where != nil {
		result.AST.Children = append(result.AST.Children, &ASTNode{Type: NodeWhere, Value: stmt.Where.Raw})
	}
	if len(stmt.GroupBy) > 0 {
		result.AST.Children = append(result.AST.Children, &ASTNode{Type: NodeGroupBy, Value: strings.Join(stmt.GroupBy, ", ")})
	}
	if stmt.HasWindow {
		result.AST.Children = append(result.AST.Children, &ASTNode{Type: NodeWindow, Value: "detected"})
	}
	for _, j := range stmt.Joins {
		result.AST.Children = append(result.AST.Children, &ASTNode{Type: NodeJoin, Value: j.Table})
	}

	return result
}

// Compile compiles a query into an execution pipeline.
func (e *Engine) Compile(id, query string) (*CompiledPipeline, error) {
	parsed := e.Parse(query)
	if !parsed.IsValid {
		return nil, fmt.Errorf("%w: %s", ErrParseFailed, strings.Join(parsed.Errors, "; "))
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.pipelines) >= e.config.MaxPipelines {
		return nil, fmt.Errorf("max pipelines reached (%d)", e.config.MaxPipelines)
	}

	// Generate execution plan
	steps := []ExecutionStep{
		{ID: 1, Operation: "SCAN", Description: fmt.Sprintf("Scan source: %s", strings.Join(parsed.Sources, ", ")), EstimatedCost: 1.0},
	}

	upper := strings.ToUpper(query)
	if strings.Contains(upper, "WHERE") {
		steps = append(steps, ExecutionStep{ID: 2, Operation: "FILTER", Description: "Apply WHERE predicates", EstimatedCost: 0.5})
	}
	if strings.Contains(upper, "WINDOW") || strings.Contains(upper, "OVER") {
		steps = append(steps, ExecutionStep{ID: len(steps) + 1, Operation: "WINDOW", Description: "Compute window aggregations", EstimatedCost: 2.0})
	}
	if strings.Contains(upper, "GROUP") {
		steps = append(steps, ExecutionStep{ID: len(steps) + 1, Operation: "GROUP", Description: "Group by key", EstimatedCost: 1.5})
	}
	steps = append(steps, ExecutionStep{ID: len(steps) + 1, Operation: "PROJECT", Description: "Project output columns", EstimatedCost: 0.1})

	pipeline := &CompiledPipeline{
		ID:         id,
		Query:      query,
		Steps:      steps,
		OutputCols: parsed.Columns,
		CompiledAt: time.Now(),
	}

	e.pipelines[id] = pipeline
	return pipeline, nil
}

// Execute runs a query and returns results.
func (e *Engine) Execute(query string) (*ExecutionResult, error) {
	parsed := e.Parse(query)
	if !parsed.IsValid {
		return nil, fmt.Errorf("%w: %s", ErrParseFailed, strings.Join(parsed.Errors, "; "))
	}

	start := time.Now()

	// Generate sample results based on the query
	result := &ExecutionResult{
		Columns:    parsed.Columns,
		Rows:       make([]map[string]interface{}, 0),
		ExecutedAt: time.Now(),
	}

	// For demonstration, return metadata about what would execute
	row := make(map[string]interface{})
	for _, col := range parsed.Columns {
		row[col] = nil
	}
	row["_sources"] = parsed.Sources
	row["_status"] = "executed"
	result.Rows = append(result.Rows, row)
	result.RowCount = len(result.Rows)
	result.DurationMs = float64(time.Since(start).Microseconds()) / 1000.0

	return result, nil
}

// GetPipeline returns a compiled pipeline.
func (e *Engine) GetPipeline(id string) (*CompiledPipeline, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	p, exists := e.pipelines[id]
	if !exists {
		return nil, ErrPipelineNotFound
	}
	return p, nil
}

// ListPipelines returns all compiled pipelines.
func (e *Engine) ListPipelines() []CompiledPipeline {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]CompiledPipeline, 0, len(e.pipelines))
	for _, p := range e.pipelines {
		result = append(result, *p)
	}
	return result
}

// DeletePipeline removes a compiled pipeline.
func (e *Engine) DeletePipeline(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.pipelines[id]; !exists {
		return ErrPipelineNotFound
	}
	delete(e.pipelines, id)
	return nil
}
