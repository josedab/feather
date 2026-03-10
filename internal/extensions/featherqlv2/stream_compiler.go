package featherqlv2

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// StreamPipelineSpec is the compiled pipeline specification from streaming SQL.
type StreamPipelineSpec struct {
	ID            string         `json:"id"`
	SQL           string         `json:"sql"`
	Window        *WindowSpec    `json:"window,omitempty"`
	GroupByKey    string         `json:"group_by_key,omitempty"`
	Aggregation   string         `json:"aggregation,omitempty"`
	SourceStream  string         `json:"source_stream"`
	OutputEntity  string         `json:"output_entity,omitempty"`
	OutputFeature string         `json:"output_feature,omitempty"`
	Watermark     *WatermarkSpec `json:"watermark,omitempty"`
	CompiledAt    time.Time      `json:"compiled_at"`
}

// StreamCompiler compiles streaming SQL to pipeline specifications.
type StreamCompiler struct {
	mu        sync.RWMutex
	parser    *StreamingParser
	pipelines map[string]*StreamPipelineSpec
	nextID    int
}

// NewStreamCompiler creates a new streaming SQL compiler.
func NewStreamCompiler() *StreamCompiler {
	return &StreamCompiler{
		parser:    NewStreamingParser(),
		pipelines: make(map[string]*StreamPipelineSpec),
	}
}

// Compile parses and compiles a streaming SQL statement into a pipeline spec.
func (c *StreamCompiler) Compile(sql string) (*StreamPipelineSpec, error) {
	stmt, err := c.parser.Parse(sql)
	if err != nil {
		return nil, fmt.Errorf("parsing streaming SQL: %w", err)
	}

	if stmt.Type != StmtSelectStream {
		return nil, fmt.Errorf("only SELECT statements can be compiled to pipelines")
	}

	c.mu.Lock()
	c.nextID++
	id := fmt.Sprintf("stream-pipeline-%d", c.nextID)
	c.mu.Unlock()

	spec := &StreamPipelineSpec{
		ID:         id,
		SQL:        sql,
		Window:     stmt.Window,
		Watermark:  stmt.WatermarkSpec,
		CompiledAt: time.Now(),
	}

	// Extract source stream from SELECT ... FROM
	if stmt.Select != nil && stmt.Select.From != "" {
		spec.SourceStream = stmt.Select.From
	}

	// Extract GROUP BY key
	if len(stmt.GroupByKeys) > 0 {
		spec.GroupByKey = stmt.GroupByKeys[0]
	}

	// Detect aggregation from SELECT columns
	if stmt.Select != nil {
		for _, col := range stmt.Select.Columns {
			if col.IsAgg {
				spec.Aggregation = strings.ToLower(col.AggFunc)
				spec.OutputFeature = col.Alias
				if spec.OutputFeature == "" {
					spec.OutputFeature = col.Expression
				}
				break
			}
		}
	}

	if spec.SourceStream == "" {
		spec.SourceStream = "default"
	}

	c.mu.Lock()
	c.pipelines[id] = spec
	c.mu.Unlock()

	return spec, nil
}

// GetPipeline returns a compiled pipeline by ID.
func (c *StreamCompiler) GetPipeline(id string) (*StreamPipelineSpec, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	spec, exists := c.pipelines[id]
	if !exists {
		return nil, fmt.Errorf("pipeline %s not found", id)
	}
	return spec, nil
}

// ListPipelines returns all compiled pipelines.
func (c *StreamCompiler) ListPipelines() []StreamPipelineSpec {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]StreamPipelineSpec, 0, len(c.pipelines))
	for _, spec := range c.pipelines {
		result = append(result, *spec)
	}
	return result
}

// DeletePipeline removes a compiled pipeline.
func (c *StreamCompiler) DeletePipeline(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.pipelines[id]; !exists {
		return fmt.Errorf("pipeline %s not found", id)
	}
	delete(c.pipelines, id)
	return nil
}

func extractAggFunction(expr string) string {
	upper := strings.ToUpper(expr)
	for _, fn := range []string{"COUNT", "SUM", "AVG", "MIN", "MAX"} {
		if strings.Contains(upper, fn+"(") {
			return strings.ToLower(fn)
		}
	}
	return ""
}
