package streamsql

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// EngineConfig configures the streaming SQL engine.
type EngineConfig struct {
	MaxQueries     int           `json:"max_queries"`
	MaxStreams      int           `json:"max_streams"`
	BufferSize     int           `json:"buffer_size"`
	DefaultTimeout time.Duration `json:"default_timeout"`
}

// DefaultEngineConfig returns an EngineConfig with sensible defaults.
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		MaxQueries:     100,
		MaxStreams:      50,
		BufferSize:     10000,
		DefaultTimeout: 30 * time.Second,
	}
}

// QueryStatus represents the status of a registered query.
type QueryStatus string

const (
	QueryStatusActive   QueryStatus = "active"
	QueryStatusPaused   QueryStatus = "paused"
	QueryStatusStopped  QueryStatus = "stopped"
)

// RegisteredQuery holds metadata about a registered continuous query.
type RegisteredQuery struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	SQL         string      `json:"sql"`
	Statement   *Statement  `json:"statement"`
	Status      QueryStatus `json:"status"`
	CreatedAt   time.Time   `json:"created_at"`
	ResultCount int64       `json:"result_count"`
}

// Stream represents a named event stream with a schema.
type Stream struct {
	Name    string
	Schema  map[string]string
	records []*Record
	mu      sync.RWMutex
}

// Record represents a single event record in a stream.
type Record struct {
	Fields    map[string]interface{} `json:"fields"`
	Timestamp time.Time              `json:"timestamp"`
}

// QueryResult holds the result of a query execution.
type QueryResult struct {
	Columns []string                 `json:"columns"`
	Rows    []map[string]interface{} `json:"rows"`
	Count   int                      `json:"count"`
}

// StreamInfo provides metadata about a stream.
type StreamInfo struct {
	Name        string            `json:"name"`
	Schema      map[string]string `json:"schema"`
	RecordCount int               `json:"record_count"`
}

// EngineStats provides runtime statistics about the engine.
type EngineStats struct {
	StreamCount    int   `json:"stream_count"`
	QueryCount     int   `json:"query_count"`
	ViewCount      int   `json:"view_count"`
	TotalRecords   int64 `json:"total_records"`
	TotalQueries   int64 `json:"total_queries"`
}

// Engine is the streaming SQL execution engine.
type Engine struct {
	config       EngineConfig
	queries      map[string]*RegisteredQuery
	streams      map[string]*Stream
	views        map[string]*MaterializedView
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	totalRecords int64
	totalQueries int64
}

// NewEngine creates a new streaming SQL engine with the given configuration.
func NewEngine(config EngineConfig) *Engine {
	ctx, cancel := context.WithCancel(context.Background())
	return &Engine{
		config:  config,
		queries: make(map[string]*RegisteredQuery),
		streams: make(map[string]*Stream),
		views:   make(map[string]*MaterializedView),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// CreateStream creates a new named stream with the given schema.
func (e *Engine) CreateStream(name string, schema map[string]string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.streams[name]; exists {
		return fmt.Errorf("creating stream: stream %q already exists", name)
	}
	if len(e.streams) >= e.config.MaxStreams {
		return fmt.Errorf("creating stream: max streams limit (%d) reached", e.config.MaxStreams)
	}

	e.streams[name] = &Stream{
		Name:   name,
		Schema: schema,
	}
	return nil
}

// DropStream removes a named stream.
func (e *Engine) DropStream(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.streams[name]; !exists {
		return fmt.Errorf("dropping stream: stream %q not found", name)
	}
	delete(e.streams, name)
	return nil
}

// ListStreams returns metadata about all registered streams.
func (e *Engine) ListStreams() []*StreamInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()

	infos := make([]*StreamInfo, 0, len(e.streams))
	for _, s := range e.streams {
		s.mu.RLock()
		count := len(s.records)
		s.mu.RUnlock()
		infos = append(infos, &StreamInfo{
			Name:        s.Name,
			Schema:      s.Schema,
			RecordCount: count,
		})
	}
	return infos
}

// Push pushes a record into the named stream.
func (e *Engine) Push(streamName string, record *Record) error {
	e.mu.RLock()
	stream, exists := e.streams[streamName]
	e.mu.RUnlock()

	if !exists {
		return fmt.Errorf("pushing record: stream %q not found", streamName)
	}

	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now()
	}

	stream.mu.Lock()
	stream.records = append(stream.records, record)
	stream.mu.Unlock()

	atomic.AddInt64(&e.totalRecords, 1)

	e.refreshOnInsertViews(e.ctx)

	return nil
}

// ExecuteQuery parses and executes a one-shot SQL query against the engine's streams.
func (e *Engine) ExecuteQuery(ctx context.Context, sql string) (*QueryResult, error) {
	stmt, err := parseSQL(sql)
	if err != nil {
		return nil, fmt.Errorf("executing query: %w", err)
	}

	records, err := e.collectRecords(stmt)
	if err != nil {
		return nil, fmt.Errorf("executing query: %w", err)
	}

	executor := newQueryExecutor(stmt)
	result, err := executor.Execute(records)
	if err != nil {
		return nil, fmt.Errorf("executing query: %w", err)
	}

	atomic.AddInt64(&e.totalQueries, 1)
	return result, nil
}

// RegisterQuery registers a continuous query by name.
func (e *Engine) RegisterQuery(ctx context.Context, name, sql string) (*RegisteredQuery, error) {
	stmt, err := parseSQL(sql)
	if err != nil {
		return nil, fmt.Errorf("registering query: %w", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.queries[name]; exists {
		return nil, fmt.Errorf("registering query: query %q already exists", name)
	}
	if len(e.queries) >= e.config.MaxQueries {
		return nil, fmt.Errorf("registering query: max queries limit (%d) reached", e.config.MaxQueries)
	}

	rq := &RegisteredQuery{
		ID:        fmt.Sprintf("q-%d", time.Now().UnixNano()),
		Name:      name,
		SQL:       sql,
		Statement: stmt,
		Status:    QueryStatusActive,
		CreatedAt: time.Now(),
	}
	e.queries[name] = rq
	return rq, nil
}

// UnregisterQuery removes a registered query by name.
func (e *Engine) UnregisterQuery(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.queries[name]; !exists {
		return fmt.Errorf("unregistering query: query %q not found", name)
	}
	delete(e.queries, name)
	return nil
}

// PauseQuery pauses a registered query.
func (e *Engine) PauseQuery(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	q, exists := e.queries[name]
	if !exists {
		return fmt.Errorf("pausing query: query %q not found", name)
	}
	if q.Status != QueryStatusActive {
		return fmt.Errorf("pausing query: query %q is not active (status: %s)", name, q.Status)
	}
	q.Status = QueryStatusPaused
	return nil
}

// ResumeQuery resumes a paused query.
func (e *Engine) ResumeQuery(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	q, exists := e.queries[name]
	if !exists {
		return fmt.Errorf("resuming query: query %q not found", name)
	}
	if q.Status != QueryStatusPaused {
		return fmt.Errorf("resuming query: query %q is not paused (status: %s)", name, q.Status)
	}
	q.Status = QueryStatusActive
	return nil
}

// GetQuery returns a registered query by name.
func (e *Engine) GetQuery(name string) (*RegisteredQuery, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	q, exists := e.queries[name]
	if !exists {
		return nil, fmt.Errorf("getting query: query %q not found", name)
	}
	return q, nil
}

// PushBatch pushes multiple records to a stream in a single call.
func (e *Engine) PushBatch(streamName string, records []*Record) (int, error) {
	e.mu.RLock()
	stream, exists := e.streams[streamName]
	e.mu.RUnlock()

	if !exists {
		return 0, fmt.Errorf("pushing batch: stream %q not found", streamName)
	}

	stream.mu.Lock()
	pushed := 0
	for _, record := range records {
		if record.Timestamp.IsZero() {
			record.Timestamp = time.Now()
		}
		stream.records = append(stream.records, record)
		pushed++
	}
	stream.mu.Unlock()

	atomic.AddInt64(&e.totalRecords, int64(pushed))

	e.refreshOnInsertViews(e.ctx)

	return pushed, nil
}

// GetStream returns information about a specific stream.
func (e *Engine) GetStream(name string) (*StreamInfo, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stream, exists := e.streams[name]
	if !exists {
		return nil, fmt.Errorf("getting stream: stream %q not found", name)
	}

	stream.mu.RLock()
	defer stream.mu.RUnlock()

	return &StreamInfo{
		Name:        stream.Name,
		Schema:      stream.Schema,
		RecordCount: len(stream.records),
	}, nil
}

// ListQueries returns all registered queries.
func (e *Engine) ListQueries() []*RegisteredQuery {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*RegisteredQuery, 0, len(e.queries))
	for _, q := range e.queries {
		result = append(result, q)
	}
	return result
}

// Stats returns runtime statistics about the engine.
func (e *Engine) Stats() *EngineStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return &EngineStats{
		StreamCount:  len(e.streams),
		QueryCount:   len(e.queries),
		ViewCount:    len(e.views),
		TotalRecords: atomic.LoadInt64(&e.totalRecords),
		TotalQueries: atomic.LoadInt64(&e.totalQueries),
	}
}

// Close shuts down the engine and releases resources.
func (e *Engine) Close() error {
	e.cancel()

	e.mu.Lock()
	defer e.mu.Unlock()

	for name := range e.queries {
		delete(e.queries, name)
	}

	for name := range e.views {
		delete(e.views, name)
	}

	return nil
}

// collectRecords gathers records from the stream(s) referenced in the statement.
func (e *Engine) collectRecords(stmt *Statement) ([]*Record, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if stmt.From == nil {
		return nil, fmt.Errorf("collecting records: no FROM clause")
	}

	stream, exists := e.streams[stmt.From.Stream]
	if !exists {
		return nil, fmt.Errorf("collecting records: stream %q not found", stmt.From.Stream)
	}

	stream.mu.RLock()
	records := make([]*Record, len(stream.records))
	copy(records, stream.records)
	stream.mu.RUnlock()

	// Handle JOIN if present
	if stmt.From.Join != nil {
		joinStream, exists := e.streams[stmt.From.Join.Stream]
		if !exists {
			return nil, fmt.Errorf("collecting records: join stream %q not found", stmt.From.Join.Stream)
		}

		joinStream.mu.RLock()
		joinRecords := make([]*Record, len(joinStream.records))
		copy(joinRecords, joinStream.records)
		joinStream.mu.RUnlock()

		records = crossJoin(records, joinRecords, stmt.From.Join.Condition)
	}

	return records, nil
}

// crossJoin performs a simple cross join with optional condition filtering.
func crossJoin(left, right []*Record, condition string) []*Record {
	var result []*Record

	// Parse the join condition (simple "a.field = b.field" format)
	parts := parseJoinCondition(condition)

	for _, l := range left {
		for _, r := range right {
			merged := &Record{
				Fields:    make(map[string]interface{}),
				Timestamp: l.Timestamp,
			}
			for k, v := range l.Fields {
				merged.Fields[k] = v
			}
			for k, v := range r.Fields {
				if _, exists := merged.Fields[k]; !exists {
					merged.Fields[k] = v
				}
			}

			if len(parts) == 2 {
				lVal := merged.Fields[parts[0]]
				rVal := merged.Fields[parts[1]]
				if fmt.Sprintf("%v", lVal) == fmt.Sprintf("%v", rVal) {
					result = append(result, merged)
				}
			} else {
				result = append(result, merged)
			}
		}
	}
	return result
}

// parseJoinCondition parses a simple "field1 = field2" condition.
func parseJoinCondition(condition string) []string {
	// Handle "a.field = b.field" or "field1 = field2"
	parts := splitOnEquals(condition)
	if len(parts) != 2 {
		return nil
	}
	left := extractFieldName(parts[0])
	right := extractFieldName(parts[1])
	return []string{left, right}
}

func splitOnEquals(s string) []string {
	idx := -1
	for i, ch := range s {
		if ch == '=' {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil
	}
	return []string{s[:idx], s[idx+1:]}
}

func extractFieldName(s string) string {
	s = trimSpaces(s)
	// Handle qualified names like "a.field"
	if idx := lastIndexByte(s, '.'); idx >= 0 {
		s = s[idx+1:]
	}
	return trimSpaces(s)
}

func trimSpaces(s string) string {
	start := 0
	end := len(s)
	for start < end && s[start] == ' ' {
		start++
	}
	for end > start && s[end-1] == ' ' {
		end--
	}
	return s[start:end]
}

func lastIndexByte(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// parseSQL is a helper that lexes and parses a SQL string.
func parseSQL(sql string) (*Statement, error) {
	lexer := NewLexer(sql)
	tokens, err := lexer.Tokenize()
	if err != nil {
		return nil, fmt.Errorf("parsing SQL: %w", err)
	}

	parser := NewParser(tokens)
	stmt, err := parser.Parse()
	if err != nil {
		return nil, fmt.Errorf("parsing SQL: %w", err)
	}
	return stmt, nil
}
