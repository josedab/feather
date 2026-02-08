package streamsql

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RefreshMode controls how a materialized view is refreshed.
type RefreshMode string

const (
	RefreshOnDemand RefreshMode = "on_demand"
	RefreshPeriodic RefreshMode = "periodic"
	RefreshOnInsert RefreshMode = "on_insert"
)

// ViewStatus represents the status of a materialized view.
type ViewStatus string

const (
	ViewStatusActive ViewStatus = "active"
	ViewStatusPaused ViewStatus = "paused"
	ViewStatusError  ViewStatus = "error"
)

// MaterializedView stores a compiled query with cached results.
type MaterializedView struct {
	Name         string
	Query        string
	Statement    *Statement
	Results      *QueryResult
	RefreshMode  RefreshMode
	Interval     time.Duration
	Status       ViewStatus
	CreatedAt    time.Time
	LastRefresh  time.Time
	RefreshCount int64
	mu           sync.RWMutex
}

// MaterializedViewInfo is a JSON-friendly representation of a materialized view.
type MaterializedViewInfo struct {
	Name         string      `json:"name"`
	Query        string      `json:"query"`
	RefreshMode  RefreshMode `json:"refresh_mode"`
	Interval     string      `json:"interval,omitempty"`
	Status       ViewStatus  `json:"status"`
	CreatedAt    time.Time   `json:"created_at"`
	LastRefresh  time.Time   `json:"last_refresh"`
	RefreshCount int64       `json:"refresh_count"`
	RowCount     int         `json:"row_count"`
}

// Refresh re-executes the view's query against the provided records and caches the results.
func (mv *MaterializedView) Refresh(records []*Record) error {
	mv.mu.Lock()
	defer mv.mu.Unlock()

	if mv.Status == ViewStatusPaused {
		return fmt.Errorf("refreshing view %q: view is paused", mv.Name)
	}

	executor := newQueryExecutor(mv.Statement)
	result, err := executor.Execute(records)
	if err != nil {
		mv.Status = ViewStatusError
		return fmt.Errorf("refreshing view %q: %w", mv.Name, err)
	}

	mv.Results = result
	mv.LastRefresh = time.Now()
	mv.RefreshCount++
	mv.Status = ViewStatusActive
	return nil
}

// GetResults returns the cached query results.
func (mv *MaterializedView) GetResults() *QueryResult {
	mv.mu.RLock()
	defer mv.mu.RUnlock()
	return mv.Results
}

// Pause pauses the materialized view.
func (mv *MaterializedView) Pause() error {
	mv.mu.Lock()
	defer mv.mu.Unlock()

	if mv.Status != ViewStatusActive {
		return fmt.Errorf("pausing view %q: view is not active (status: %s)", mv.Name, mv.Status)
	}
	mv.Status = ViewStatusPaused
	return nil
}

// Resume resumes a paused materialized view.
func (mv *MaterializedView) Resume() error {
	mv.mu.Lock()
	defer mv.mu.Unlock()

	if mv.Status != ViewStatusPaused {
		return fmt.Errorf("resuming view %q: view is not paused (status: %s)", mv.Name, mv.Status)
	}
	mv.Status = ViewStatusActive
	return nil
}

// CreateMaterializedView creates and registers a new materialized view.
func (e *Engine) CreateMaterializedView(name, sql string, mode RefreshMode, interval time.Duration) error {
	stmt, err := parseSQL(sql)
	if err != nil {
		return fmt.Errorf("creating materialized view: %w", err)
	}

	e.mu.Lock()
	if _, exists := e.views[name]; exists {
		e.mu.Unlock()
		return fmt.Errorf("creating materialized view: view %q already exists", name)
	}

	mv := &MaterializedView{
		Name:        name,
		Query:       sql,
		Statement:   stmt,
		RefreshMode: mode,
		Interval:    interval,
		Status:      ViewStatusActive,
		CreatedAt:   time.Now(),
	}
	e.views[name] = mv
	e.mu.Unlock()

	// Perform initial refresh outside the lock
	records, err := e.collectRecords(stmt)
	if err == nil && len(records) > 0 {
		_ = mv.Refresh(records)
	}

	return nil
}

// DropMaterializedView removes a materialized view by name.
func (e *Engine) DropMaterializedView(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.views[name]; !exists {
		return fmt.Errorf("dropping materialized view: view %q not found", name)
	}
	delete(e.views, name)
	return nil
}

// RefreshView re-executes the query for a materialized view and updates its cached results.
func (e *Engine) RefreshView(name string) error {
	e.mu.RLock()
	mv, exists := e.views[name]
	e.mu.RUnlock()

	if !exists {
		return fmt.Errorf("refreshing view: view %q not found", name)
	}

	records, err := e.collectRecords(mv.Statement)
	if err != nil {
		return fmt.Errorf("refreshing view %q: %w", name, err)
	}

	return mv.Refresh(records)
}

// GetView returns a materialized view by name.
func (e *Engine) GetView(name string) (*MaterializedView, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	mv, exists := e.views[name]
	if !exists {
		return nil, fmt.Errorf("getting view: view %q not found", name)
	}
	return mv, nil
}

// ListViews returns metadata about all registered materialized views.
func (e *Engine) ListViews() []*MaterializedViewInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()

	infos := make([]*MaterializedViewInfo, 0, len(e.views))
	for _, mv := range e.views {
		mv.mu.RLock()
		rowCount := 0
		if mv.Results != nil {
			rowCount = mv.Results.Count
		}
		intervalStr := ""
		if mv.Interval > 0 {
			intervalStr = mv.Interval.String()
		}
		infos = append(infos, &MaterializedViewInfo{
			Name:         mv.Name,
			Query:        mv.Query,
			RefreshMode:  mv.RefreshMode,
			Interval:     intervalStr,
			Status:       mv.Status,
			CreatedAt:    mv.CreatedAt,
			LastRefresh:  mv.LastRefresh,
			RefreshCount: mv.RefreshCount,
			RowCount:     rowCount,
		})
		mv.mu.RUnlock()
	}
	return infos
}

// refreshOnInsertViews refreshes all views with RefreshOnInsert mode.
func (e *Engine) refreshOnInsertViews(ctx context.Context) {
	e.mu.RLock()
	var onInsertViews []*MaterializedView
	for _, mv := range e.views {
		mv.mu.RLock()
		if mv.RefreshMode == RefreshOnInsert && mv.Status == ViewStatusActive {
			onInsertViews = append(onInsertViews, mv)
		}
		mv.mu.RUnlock()
	}
	e.mu.RUnlock()

	for _, mv := range onInsertViews {
		records, err := e.collectRecords(mv.Statement)
		if err != nil {
			continue
		}
		_ = mv.Refresh(records)
	}
}
