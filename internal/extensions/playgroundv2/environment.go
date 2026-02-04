// Package playgroundv2 provides an enhanced, interactive feature development
// environment with query building, schema browsing, live simulation, and
// deploy-flow capabilities for feature store exploration.
package playgroundv2

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Config holds all tunable parameters for the playground environment.
type Config struct {
	MaxConcurrentUsers int  `json:"max_concurrent_users" yaml:"max_concurrent_users"`
	QueryTimeoutSeconds int  `json:"query_timeout_seconds" yaml:"query_timeout_seconds"`
	MaxResultSize      int  `json:"max_result_size" yaml:"max_result_size"`
	EnableSimulation   bool `json:"enable_simulation" yaml:"enable_simulation"`
	EnableDeployFlow   bool `json:"enable_deploy_flow" yaml:"enable_deploy_flow"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		MaxConcurrentUsers:  50,
		QueryTimeoutSeconds: 30,
		MaxResultSize:       10000,
		EnableSimulation:    true,
		EnableDeployFlow:    true,
	}
}

// TimeRange represents a bounded time interval for queries.
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// FeatureSpec describes a feature to register.
type FeatureSpec struct {
	Name        string `json:"name"`
	DataType    string `json:"data_type"`
	Description string `json:"description"`
}

// --- Query Builder types ---

// Query represents a playground query request.
type Query struct {
	Text           string    `json:"text"`
	EntityFilters  []string  `json:"entity_filters,omitempty"`
	FeatureFilters []string  `json:"feature_filters,omitempty"`
	TimeRange      *TimeRange `json:"time_range,omitempty"`
	Format         string    `json:"format,omitempty"`
}

// ColumnInfo describes a single column in a query result set.
type ColumnInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
}

// QueryResult holds the output of a playground query.
type QueryResult struct {
	Columns    []string        `json:"columns"`
	Rows       [][]interface{} `json:"rows"`
	RowCount   int             `json:"row_count"`
	DurationMs int64           `json:"duration_ms"`
	Schema     []ColumnInfo    `json:"schema"`
}

// --- Schema Browser types ---

// SchemaInfo is a summary of a feature group's schema.
type SchemaInfo struct {
	GroupName    string            `json:"group_name"`
	EntityType   string            `json:"entity_type"`
	FeatureCount int               `json:"feature_count"`
	Description  string            `json:"description"`
	Tags         map[string]string `json:"tags,omitempty"`
}

// FeatureDetail describes a single feature inside a schema.
type FeatureDetail struct {
	Name           string        `json:"name"`
	DataType       string        `json:"data_type"`
	HasAggregation bool          `json:"has_aggregation"`
	HasValidation  bool          `json:"has_validation"`
	SampleValues   []interface{} `json:"sample_values,omitempty"`
}

// SchemaDetails is the full description of a feature group.
type SchemaDetails struct {
	SchemaInfo
	Features []FeatureDetail `json:"features"`
}

// --- Simulation types ---

// SimulationConfig describes how to run a live feature simulation.
type SimulationConfig struct {
	Features          []string `json:"features"`
	DurationSeconds   int      `json:"duration_seconds"`
	UpdateFrequencyMs int      `json:"update_frequency_ms"`
}

// SimulationEvent records a single simulated feature change.
type SimulationEvent struct {
	Timestamp  time.Time   `json:"timestamp"`
	Feature    string      `json:"feature"`
	Entity     string      `json:"entity"`
	OldValue   interface{} `json:"old_value"`
	NewValue   interface{} `json:"new_value"`
	DriftScore float64     `json:"drift_score"`
}

// SimulationSession tracks an active or completed simulation.
type SimulationSession struct {
	ID     string            `json:"id"`
	Status string            `json:"status"` // "running", "stopped", "completed"
	Events []SimulationEvent `json:"events"`

	mu     sync.Mutex
	cancel chan struct{}
}

// --- Deploy Flow types ---

// RegistrationSpec is the request body for a deploy-flow registration.
type RegistrationSpec struct {
	GroupName  string        `json:"group_name"`
	EntityType string        `json:"entity_type"`
	Features   []FeatureSpec `json:"features"`
}

// RegistrationPreview is the dry-run result of a registration.
type RegistrationPreview struct {
	Valid    bool     `json:"valid"`
	Warnings []string `json:"warnings,omitempty"`
	Errors   []string `json:"errors,omitempty"`
	Impact   string   `json:"impact"`
}

// RegistrationResult is the outcome of a confirmed registration.
type RegistrationResult struct {
	GroupName       string `json:"group_name"`
	FeaturesCreated int    `json:"features_created"`
	Status          string `json:"status"`
}

// --- Stats ---

// Stats holds aggregate counters for the playground.
type Stats struct {
	ActiveUsers              int64 `json:"active_users"`
	QueriesExecuted          int64 `json:"queries_executed"`
	SimulationsRun           int64 `json:"simulations_run"`
	RegistrationsCompleted   int64 `json:"registrations_completed"`
}

// --- SchemaProvider abstracts schema access for testability ---

// SchemaProvider is an optional dependency for real schema data.
// When nil the environment returns demo data.
type SchemaProvider interface {
	ListGroups() []SchemaInfo
	GetGroupDetails(groupName string) (*SchemaDetails, error)
}

// --- Environment ---

// Environment is the top-level playground service. It is safe for concurrent
// use by multiple goroutines.
type Environment struct {
	cfg      Config
	schemas  SchemaProvider

	mu          sync.RWMutex
	simulations map[string]*SimulationSession
	groups      map[string]*RegistrationResult

	activeUsers            atomic.Int64
	queriesExecuted        atomic.Int64
	simulationsRun         atomic.Int64
	registrationsCompleted atomic.Int64
}

// NewEnvironment creates a playground environment. provider may be nil, in
// which case built-in demo schemas are returned.
func NewEnvironment(cfg Config, provider SchemaProvider) *Environment {
	return &Environment{
		cfg:         cfg,
		schemas:     provider,
		simulations: make(map[string]*SimulationSession),
		groups:      make(map[string]*RegistrationResult),
	}
}

// --- Query Builder ---

// ExecuteQuery parses and executes a playground query, returning a result set.
func (e *Environment) ExecuteQuery(q Query) (*QueryResult, error) {
	start := time.Now()

	if strings.TrimSpace(q.Text) == "" {
		return nil, fmt.Errorf("playgroundv2: query text must not be empty")
	}

	columns := []string{"entity", "feature", "value", "timestamp"}
	schema := []ColumnInfo{
		{Name: "entity", Type: "string", Nullable: false},
		{Name: "feature", Type: "string", Nullable: false},
		{Name: "value", Type: "any", Nullable: true},
		{Name: "timestamp", Type: "int64", Nullable: false},
	}

	rows := e.buildRows(q)
	if len(rows) > e.cfg.MaxResultSize {
		rows = rows[:e.cfg.MaxResultSize]
	}

	e.queriesExecuted.Add(1)

	return &QueryResult{
		Columns:    columns,
		Rows:       rows,
		RowCount:   len(rows),
		DurationMs: time.Since(start).Milliseconds(),
		Schema:     schema,
	}, nil
}

// buildRows produces placeholder result rows filtered by the query parameters.
func (e *Environment) buildRows(q Query) [][]interface{} {
	entities := q.EntityFilters
	if len(entities) == 0 {
		entities = []string{"user:1", "user:2", "user:3"}
	}
	features := q.FeatureFilters
	if len(features) == 0 {
		features = []string{"login_count", "last_active"}
	}

	now := time.Now().Unix()
	var rows [][]interface{}
	for _, ent := range entities {
		for _, feat := range features {
			rows = append(rows, []interface{}{ent, feat, nil, now})
		}
	}
	return rows
}

// --- Schema Browser ---

// BrowseSchemas returns summary information for all known feature groups.
func (e *Environment) BrowseSchemas() []SchemaInfo {
	if e.schemas != nil {
		return e.schemas.ListGroups()
	}

	// Built-in demo schemas.
	return []SchemaInfo{
		{
			GroupName:    "user_features",
			EntityType:   "user",
			FeatureCount: 5,
			Description:  "Core user behavioural features",
			Tags:         map[string]string{"team": "ml", "env": "production"},
		},
		{
			GroupName:    "product_features",
			EntityType:   "product",
			FeatureCount: 3,
			Description:  "Product catalogue features",
			Tags:         map[string]string{"team": "data", "env": "staging"},
		},
	}
}

// GetSchemaDetails returns full details for a single feature group.
func (e *Environment) GetSchemaDetails(groupName string) (*SchemaDetails, error) {
	if e.schemas != nil {
		return e.schemas.GetGroupDetails(groupName)
	}

	demos := map[string]*SchemaDetails{
		"user_features": {
			SchemaInfo: SchemaInfo{
				GroupName: "user_features", EntityType: "user",
				FeatureCount: 5, Description: "Core user behavioural features",
				Tags: map[string]string{"team": "ml", "env": "production"},
			},
			Features: []FeatureDetail{
				{Name: "login_count", DataType: "int64", HasAggregation: true, HasValidation: true, SampleValues: []interface{}{10, 25, 42}},
				{Name: "last_active", DataType: "timestamp", HasAggregation: false, HasValidation: false, SampleValues: []interface{}{"2024-01-15T10:30:00Z"}},
				{Name: "total_spend", DataType: "float64", HasAggregation: true, HasValidation: true, SampleValues: []interface{}{99.99, 250.0}},
				{Name: "is_premium", DataType: "bool", HasAggregation: false, HasValidation: false, SampleValues: []interface{}{true, false}},
				{Name: "name", DataType: "string", HasAggregation: false, HasValidation: true, SampleValues: []interface{}{"Alice", "Bob"}},
			},
		},
		"product_features": {
			SchemaInfo: SchemaInfo{
				GroupName: "product_features", EntityType: "product",
				FeatureCount: 3, Description: "Product catalogue features",
				Tags: map[string]string{"team": "data", "env": "staging"},
			},
			Features: []FeatureDetail{
				{Name: "price", DataType: "float64", HasAggregation: false, HasValidation: true, SampleValues: []interface{}{19.99, 49.99}},
				{Name: "category", DataType: "string", HasAggregation: false, HasValidation: false, SampleValues: []interface{}{"electronics", "books"}},
				{Name: "view_count", DataType: "int64", HasAggregation: true, HasValidation: false, SampleValues: []interface{}{120, 3400}},
			},
		},
	}

	d, ok := demos[groupName]
	if !ok {
		return nil, fmt.Errorf("playgroundv2: schema group %q not found", groupName)
	}
	return d, nil
}

// --- Response Viewer ---

// FormatResponse serialises a QueryResult in the requested format.
// Supported formats: "json", "csv", "table", "chart".
func (e *Environment) FormatResponse(result *QueryResult, format string) ([]byte, error) {
	if result == nil {
		return nil, fmt.Errorf("playgroundv2: result must not be nil")
	}

	switch strings.ToLower(format) {
	case "json":
		return json.MarshalIndent(result, "", "  ")
	case "csv":
		return e.formatCSV(result)
	case "table":
		return e.formatTable(result), nil
	case "chart":
		return e.formatChart(result), nil
	default:
		return nil, fmt.Errorf("playgroundv2: unsupported format %q (supported: json, csv, table, chart)", format)
	}
}

func (e *Environment) formatCSV(result *QueryResult) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	if err := w.Write(result.Columns); err != nil {
		return nil, fmt.Errorf("playgroundv2: writing csv header: %w", err)
	}
	for _, row := range result.Rows {
		record := make([]string, len(row))
		for i, v := range row {
			record[i] = fmt.Sprintf("%v", v)
		}
		if err := w.Write(record); err != nil {
			return nil, fmt.Errorf("playgroundv2: writing csv row: %w", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("playgroundv2: flushing csv: %w", err)
	}
	return buf.Bytes(), nil
}

func (e *Environment) formatTable(result *QueryResult) []byte {
	var buf bytes.Buffer
	widths := make([]int, len(result.Columns))
	for i, col := range result.Columns {
		widths[i] = len(col)
	}
	for _, row := range result.Rows {
		for i, v := range row {
			l := len(fmt.Sprintf("%v", v))
			if l > widths[i] {
				widths[i] = l
			}
		}
	}

	writePadded := func(vals []string) {
		for i, v := range vals {
			if i > 0 {
				buf.WriteString(" | ")
			}
			buf.WriteString(v)
			if pad := widths[i] - len(v); pad > 0 {
				buf.WriteString(strings.Repeat(" ", pad))
			}
		}
		buf.WriteByte('\n')
	}

	writePadded(result.Columns)
	for i := range result.Columns {
		if i > 0 {
			buf.WriteString("-+-")
		}
		buf.WriteString(strings.Repeat("-", widths[i]))
	}
	buf.WriteByte('\n')

	for _, row := range result.Rows {
		vals := make([]string, len(row))
		for i, v := range row {
			vals[i] = fmt.Sprintf("%v", v)
		}
		writePadded(vals)
	}
	return buf.Bytes()
}

func (e *Environment) formatChart(result *QueryResult) []byte {
	chart := struct {
		Type    string          `json:"type"`
		Columns []string        `json:"columns"`
		Data    [][]interface{} `json:"data"`
		Rows    int             `json:"rows"`
	}{
		Type:    "bar",
		Columns: result.Columns,
		Data:    result.Rows,
		Rows:    result.RowCount,
	}
	out, _ := json.MarshalIndent(chart, "", "  ")
	return out
}

// --- Live Simulation ---

// StartSimulation begins a background simulation that generates synthetic
// feature update events.
func (e *Environment) StartSimulation(cfg SimulationConfig) (*SimulationSession, error) {
	if !e.cfg.EnableSimulation {
		return nil, fmt.Errorf("playgroundv2: simulation is disabled")
	}
	if len(cfg.Features) == 0 {
		return nil, fmt.Errorf("playgroundv2: at least one feature is required")
	}
	if cfg.DurationSeconds <= 0 {
		cfg.DurationSeconds = 10
	}
	if cfg.UpdateFrequencyMs <= 0 {
		cfg.UpdateFrequencyMs = 500
	}

	sess := &SimulationSession{
		ID:     fmt.Sprintf("sim-%d", time.Now().UnixNano()),
		Status: "running",
		cancel: make(chan struct{}),
	}

	e.mu.Lock()
	e.simulations[sess.ID] = sess
	e.mu.Unlock()

	e.simulationsRun.Add(1)

	go e.runSimulation(sess, cfg)

	return sess, nil
}

func (e *Environment) runSimulation(sess *SimulationSession, cfg SimulationConfig) {
	ticker := time.NewTicker(time.Duration(cfg.UpdateFrequencyMs) * time.Millisecond)
	defer ticker.Stop()

	deadline := time.After(time.Duration(cfg.DurationSeconds) * time.Second)
	entities := []string{"user:100", "user:200", "user:300"}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for {
		select {
		case <-sess.cancel:
			sess.mu.Lock()
			sess.Status = "stopped"
			sess.mu.Unlock()
			return
		case <-deadline:
			sess.mu.Lock()
			sess.Status = "completed"
			sess.mu.Unlock()
			return
		case <-ticker.C:
			feat := cfg.Features[rng.Intn(len(cfg.Features))]
			ent := entities[rng.Intn(len(entities))]
			oldVal := rng.Float64() * 100
			newVal := rng.Float64() * 100
			drift := rng.Float64()

			evt := SimulationEvent{
				Timestamp:  time.Now(),
				Feature:    feat,
				Entity:     ent,
				OldValue:   oldVal,
				NewValue:   newVal,
				DriftScore: drift,
			}

			sess.mu.Lock()
			sess.Events = append(sess.Events, evt)
			sess.mu.Unlock()
		}
	}
}

// StopSimulation terminates a running simulation session.
func (e *Environment) StopSimulation(sessionID string) error {
	e.mu.RLock()
	sess, ok := e.simulations[sessionID]
	e.mu.RUnlock()

	if !ok {
		return fmt.Errorf("playgroundv2: simulation %q not found", sessionID)
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	if sess.Status != "running" {
		return fmt.Errorf("playgroundv2: simulation %q is not running (status: %s)", sessionID, sess.Status)
	}

	close(sess.cancel)
	return nil
}

// --- Deploy Flow ---

// PreviewRegistration validates a registration spec without persisting it.
func (e *Environment) PreviewRegistration(spec RegistrationSpec) (*RegistrationPreview, error) {
	if !e.cfg.EnableDeployFlow {
		return nil, fmt.Errorf("playgroundv2: deploy flow is disabled")
	}

	preview := &RegistrationPreview{Valid: true}

	if spec.GroupName == "" {
		preview.Valid = false
		preview.Errors = append(preview.Errors, "group_name is required")
	}
	if spec.EntityType == "" {
		preview.Valid = false
		preview.Errors = append(preview.Errors, "entity_type is required")
	}
	if len(spec.Features) == 0 {
		preview.Valid = false
		preview.Errors = append(preview.Errors, "at least one feature is required")
	}

	seen := make(map[string]struct{})
	for _, f := range spec.Features {
		if f.Name == "" {
			preview.Valid = false
			preview.Errors = append(preview.Errors, "feature name must not be empty")
		}
		if _, dup := seen[f.Name]; dup {
			preview.Valid = false
			preview.Errors = append(preview.Errors, fmt.Sprintf("duplicate feature name %q", f.Name))
		}
		seen[f.Name] = struct{}{}

		if f.DataType == "" {
			preview.Warnings = append(preview.Warnings, fmt.Sprintf("feature %q has no data type; will default to string", f.Name))
		}
	}

	e.mu.RLock()
	_, exists := e.groups[spec.GroupName]
	e.mu.RUnlock()

	if exists {
		preview.Warnings = append(preview.Warnings, fmt.Sprintf("group %q already exists and will be updated", spec.GroupName))
		preview.Impact = fmt.Sprintf("update group %q with %d features", spec.GroupName, len(spec.Features))
	} else {
		preview.Impact = fmt.Sprintf("create group %q with %d features", spec.GroupName, len(spec.Features))
	}

	return preview, nil
}

// ConfirmRegistration validates and persists a feature group registration.
func (e *Environment) ConfirmRegistration(spec RegistrationSpec) (*RegistrationResult, error) {
	preview, err := e.PreviewRegistration(spec)
	if err != nil {
		return nil, err
	}
	if !preview.Valid {
		return nil, fmt.Errorf("playgroundv2: registration invalid: %s", strings.Join(preview.Errors, "; "))
	}

	result := &RegistrationResult{
		GroupName:       spec.GroupName,
		FeaturesCreated: len(spec.Features),
		Status:          "created",
	}

	e.mu.Lock()
	if _, exists := e.groups[spec.GroupName]; exists {
		result.Status = "updated"
	}
	e.groups[spec.GroupName] = result
	e.mu.Unlock()

	e.registrationsCompleted.Add(1)

	return result, nil
}

// --- Stats ---

// Stats returns aggregate playground counters.
func (e *Environment) Stats() Stats {
	return Stats{
		ActiveUsers:            e.activeUsers.Load(),
		QueriesExecuted:        e.queriesExecuted.Load(),
		SimulationsRun:         e.simulationsRun.Load(),
		RegistrationsCompleted: e.registrationsCompleted.Load(),
	}
}
