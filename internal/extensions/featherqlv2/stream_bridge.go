package featherqlv2

import (
	"fmt"
	"time"
)

// StreamComputeConfig is the output format for bridging to the streamcompute engine.
// This mirrors streamcompute.PipelineConfig without creating a dependency.
type StreamComputeConfig struct {
	ID            string        `json:"id"`
	Description   string        `json:"description"`
	WindowType    string        `json:"window_type"`    // "tumbling", "sliding", "session"
	WindowSize    time.Duration `json:"window_size"`
	WindowSlide   time.Duration `json:"window_slide,omitempty"`
	WindowGap     time.Duration `json:"window_gap,omitempty"`
	GroupByKey    string        `json:"group_by_key,omitempty"`
	Aggregation   string        `json:"aggregation,omitempty"` // "count", "sum", "avg", "min", "max"
	OutputEntity  string        `json:"output_entity,omitempty"`
	OutputFeature string        `json:"output_feature,omitempty"`
}

// ToStreamComputeConfig converts a StreamPipelineSpec into a config suitable
// for the streamcompute engine.
func (s *StreamPipelineSpec) ToStreamComputeConfig() (*StreamComputeConfig, error) {
	cfg := &StreamComputeConfig{
		ID:            s.ID,
		Description:   fmt.Sprintf("Compiled from: %s", s.SQL),
		GroupByKey:    s.GroupByKey,
		Aggregation:   s.Aggregation,
		OutputEntity:  s.SourceStream,
		OutputFeature: s.OutputFeature,
	}

	if s.Window != nil {
		cfg.WindowType = s.Window.Type
		cfg.WindowSize = s.Window.Size
		cfg.WindowSlide = s.Window.Slide
		cfg.WindowGap = s.Window.Gap
	} else {
		// Default to tumbling window if no window specified
		cfg.WindowType = "tumbling"
		cfg.WindowSize = 1 * time.Minute
	}

	if cfg.Aggregation == "" {
		cfg.Aggregation = "count"
	}

	return cfg, nil
}

// CompileToStreamCompute parses SQL and returns a streamcompute-compatible config.
func (c *StreamCompiler) CompileToStreamCompute(sql string) (*StreamComputeConfig, error) {
	spec, err := c.Compile(sql)
	if err != nil {
		return nil, fmt.Errorf("compiling SQL: %w", err)
	}
	return spec.ToStreamComputeConfig()
}
