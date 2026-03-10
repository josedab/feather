package featherqlv2

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// StreamingStatement represents a parsed streaming SQL statement.
type StreamingStatement struct {
	Type          StreamingStmtType `json:"type"`
	StreamName    string            `json:"stream_name,omitempty"`
	Schema        map[string]string `json:"schema,omitempty"`
	Select        *SelectStatement  `json:"select,omitempty"`
	Window        *WindowSpec       `json:"window,omitempty"`
	EmitMode      string            `json:"emit_mode,omitempty"` // "changes", "final"
	GroupByKeys   []string          `json:"group_by_keys,omitempty"`
	HavingClause  string            `json:"having_clause,omitempty"`
	WatermarkSpec *WatermarkSpec    `json:"watermark,omitempty"`
	Raw           string            `json:"raw"`
}

// StreamingStmtType identifies the streaming statement type.
type StreamingStmtType string

const (
	StmtCreateStream StreamingStmtType = "CREATE_STREAM"
	StmtDropStream   StreamingStmtType = "DROP_STREAM"
	StmtSelectStream StreamingStmtType = "SELECT_STREAM"
	StmtInsertStream StreamingStmtType = "INSERT_STREAM"
)

// WindowSpec describes a windowing operation.
type WindowSpec struct {
	Type  string        `json:"type"` // "tumbling", "sliding", "session"
	Size  time.Duration `json:"size"`
	Slide time.Duration `json:"slide,omitempty"` // for sliding windows
	Gap   time.Duration `json:"gap,omitempty"`   // for session windows
}

// WatermarkSpec configures event-time watermarking.
type WatermarkSpec struct {
	Column   string        `json:"column"`
	MaxDelay time.Duration `json:"max_delay"`
}

// StreamingParser extends the base parser with streaming SQL support.
type StreamingParser struct {
	mu sync.RWMutex
}

// NewStreamingParser creates a new streaming SQL parser.
func NewStreamingParser() *StreamingParser {
	return &StreamingParser{}
}

// Parse parses a streaming SQL statement.
func (p *StreamingParser) Parse(sql string) (*StreamingStatement, error) {
	normalized := strings.TrimSpace(sql)
	upper := strings.ToUpper(normalized)

	switch {
	case strings.HasPrefix(upper, "CREATE STREAM"):
		return p.parseCreateStream(normalized)
	case strings.HasPrefix(upper, "DROP STREAM"):
		return p.parseDropStream(normalized)
	case strings.Contains(upper, "TUMBLING") || strings.Contains(upper, "SLIDING") || strings.Contains(upper, "SESSION") || strings.Contains(upper, "EMIT"):
		return p.parseStreamSelect(normalized)
	default:
		stmt, err := ParseQuery(normalized)
		if err != nil {
			return nil, fmt.Errorf("parsing streaming SQL: %w", err)
		}
		return &StreamingStatement{
			Type:   StmtSelectStream,
			Select: stmt,
			Raw:    normalized,
		}, nil
	}
}

func (p *StreamingParser) parseCreateStream(sql string) (*StreamingStatement, error) {
	upper := strings.ToUpper(sql)
	rest := strings.TrimSpace(sql[len("CREATE STREAM"):])

	// Extract stream name (word before the opening parenthesis)
	parenIdx := strings.Index(rest, "(")
	if parenIdx < 0 {
		// CREATE STREAM without schema
		streamName := strings.TrimSpace(rest)
		if streamName == "" {
			return nil, fmt.Errorf("stream name is required")
		}
		return &StreamingStatement{
			Type:       StmtCreateStream,
			StreamName: strings.TrimRight(streamName, ";"),
			Schema:     make(map[string]string),
			Raw:        sql,
		}, nil
	}

	streamName := strings.TrimSpace(rest[:parenIdx])
	if streamName == "" {
		return nil, fmt.Errorf("stream name is required")
	}

	// Parse column definitions
	closeIdx := strings.LastIndex(rest, ")")
	if closeIdx < 0 {
		return nil, fmt.Errorf("missing closing parenthesis")
	}
	colsDef := rest[parenIdx+1 : closeIdx]
	schema := make(map[string]string)
	for _, col := range strings.Split(colsDef, ",") {
		col = strings.TrimSpace(col)
		if col == "" {
			continue
		}
		parts := strings.Fields(col)
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid column definition: %s", col)
		}
		schema[parts[0]] = strings.ToLower(parts[1])
	}

	stmt := &StreamingStatement{
		Type:       StmtCreateStream,
		StreamName: streamName,
		Schema:     schema,
		Raw:        sql,
	}

	// Check for WATERMARK clause
	if idx := strings.Index(upper, "WATERMARK"); idx >= 0 {
		watermarkRest := sql[idx+len("WATERMARK"):]
		watermarkRest = strings.TrimSpace(watermarkRest)
		// Parse: FOR column AS delay
		if strings.HasPrefix(strings.ToUpper(watermarkRest), "FOR") {
			parts := strings.Fields(watermarkRest)
			if len(parts) >= 4 {
				delay, err := time.ParseDuration(parts[3])
				if err == nil {
					stmt.WatermarkSpec = &WatermarkSpec{
						Column:   parts[1],
						MaxDelay: delay,
					}
				}
			}
		}
	}

	return stmt, nil
}

func (p *StreamingParser) parseDropStream(sql string) (*StreamingStatement, error) {
	rest := strings.TrimSpace(sql[len("DROP STREAM"):])
	name := strings.TrimRight(strings.TrimSpace(rest), ";")
	if name == "" {
		return nil, fmt.Errorf("stream name is required")
	}
	return &StreamingStatement{
		Type:       StmtDropStream,
		StreamName: name,
		Raw:        sql,
	}, nil
}

func (p *StreamingParser) parseStreamSelect(sql string) (*StreamingStatement, error) {
	upper := strings.ToUpper(sql)
	stmt := &StreamingStatement{
		Type: StmtSelectStream,
		Raw:  sql,
	}

	// Extract window specification
	if idx := strings.Index(upper, "TUMBLING"); idx >= 0 {
		size := p.extractDuration(sql[idx:])
		stmt.Window = &WindowSpec{Type: "tumbling", Size: size}
	} else if idx := strings.Index(upper, "SLIDING"); idx >= 0 {
		size := p.extractDuration(sql[idx:])
		slide := size / 2 // default slide is half the window
		if slideIdx := strings.Index(upper[idx:], "SLIDE"); slideIdx >= 0 {
			slide = p.extractDuration(sql[idx+slideIdx:])
		}
		stmt.Window = &WindowSpec{Type: "sliding", Size: size, Slide: slide}
	} else if idx := strings.Index(upper, "SESSION"); idx >= 0 {
		gap := p.extractDuration(sql[idx:])
		stmt.Window = &WindowSpec{Type: "session", Gap: gap}
	}

	// Extract EMIT mode
	if strings.Contains(upper, "EMIT CHANGES") {
		stmt.EmitMode = "changes"
	} else if strings.Contains(upper, "EMIT FINAL") {
		stmt.EmitMode = "final"
	}

	// Extract GROUP BY
	if idx := strings.Index(upper, "GROUP BY"); idx >= 0 {
		groupByStr := sql[idx+len("GROUP BY"):]
		// Trim anything after HAVING, WINDOW, EMIT, or end
		for _, keyword := range []string{"HAVING", "WINDOW", "EMIT", "ORDER", "TUMBLING", "SLIDING", "SESSION"} {
			if ki := strings.Index(strings.ToUpper(groupByStr), keyword); ki >= 0 {
				groupByStr = groupByStr[:ki]
			}
		}
		for _, key := range strings.Split(groupByStr, ",") {
			key = strings.TrimSpace(key)
			if key != "" {
				stmt.GroupByKeys = append(stmt.GroupByKeys, key)
			}
		}
	}

	// Extract WATERMARK clause
	if idx := strings.Index(upper, "WATERMARK"); idx >= 0 {
		rest := sql[idx+len("WATERMARK"):]
		parts := strings.Fields(rest)
		if len(parts) >= 4 && strings.ToUpper(parts[0]) == "FOR" {
			delay, err := time.ParseDuration(parts[3])
			if err == nil {
				stmt.WatermarkSpec = &WatermarkSpec{
					Column:   parts[1],
					MaxDelay: delay,
				}
			}
		}
	}

	// Try to parse the SELECT portion
	selectSQL := sql
	for _, kw := range []string{"TUMBLING", "SLIDING", "SESSION", "EMIT"} {
		if idx := strings.Index(strings.ToUpper(selectSQL), kw); idx >= 0 {
			selectSQL = selectSQL[:idx]
		}
	}
	selectSQL = strings.TrimSpace(selectSQL)
	if strings.HasPrefix(strings.ToUpper(selectSQL), "SELECT") {
		parsed, err := ParseQuery(selectSQL)
		if err == nil {
			stmt.Select = parsed
		}
	}

	return stmt, nil
}

func (p *StreamingParser) extractDuration(s string) time.Duration {
	// Look for parenthesized duration like TUMBLING(5m) or TUMBLING(1h)
	parenIdx := strings.Index(s, "(")
	if parenIdx < 0 {
		return 5 * time.Minute // default
	}
	closeIdx := strings.Index(s[parenIdx:], ")")
	if closeIdx < 0 {
		return 5 * time.Minute
	}
	durStr := strings.TrimSpace(s[parenIdx+1 : parenIdx+closeIdx])
	d, err := time.ParseDuration(durStr)
	if err != nil {
		return 5 * time.Minute
	}
	return d
}
