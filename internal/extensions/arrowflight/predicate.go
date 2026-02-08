package arrowflight

import (
	"context"
	"fmt"
	"sync/atomic"
)

// Predicate represents a filter condition for predicate pushdown.
type Predicate struct {
	Column   string      `json:"column"`
	Operator string      `json:"operator"` // "eq", "ne", "gt", "gte", "lt", "lte", "in", "not_null"
	Value    interface{} `json:"value,omitempty"`
	Values   []interface{} `json:"values,omitempty"` // For "in" operator
}

// BatchRequest represents a columnar batch retrieval request with predicate pushdown.
type BatchRequest struct {
	Descriptor FlightDescriptor `json:"descriptor"`
	Predicates []Predicate      `json:"predicates,omitempty"`
	Columns    []string         `json:"columns,omitempty"`    // Column projection
	Limit      int              `json:"limit,omitempty"`
	Offset     int              `json:"offset,omitempty"`
	OrderBy    string           `json:"order_by,omitempty"`
	Descending bool             `json:"descending,omitempty"`
}

// BatchResponse wraps a RecordBatch with additional metadata.
type BatchResponse struct {
	Data       *RecordBatch `json:"data"`
	TotalRows  int64        `json:"total_rows"`
	HasMore    bool         `json:"has_more"`
	NextOffset int          `json:"next_offset,omitempty"`
}

// PredicateEvaluator evaluates predicates against column data.
type PredicateEvaluator struct{}

// NewPredicateEvaluator creates a new predicate evaluator.
func NewPredicateEvaluator() *PredicateEvaluator {
	return &PredicateEvaluator{}
}

// Apply filters a RecordBatch by the given predicates.
func (pe *PredicateEvaluator) Apply(batch *RecordBatch, predicates []Predicate) *RecordBatch {
	if len(predicates) == 0 || batch == nil || batch.Rows == 0 {
		return batch
	}

	// Build index of rows that pass all predicates
	passingRows := make([]int, 0, batch.Rows)
	for i := 0; i < batch.Rows; i++ {
		if pe.rowPassesAll(batch, i, predicates) {
			passingRows = append(passingRows, i)
		}
	}

	// Build filtered batch
	filtered := &RecordBatch{
		Schema:  batch.Schema,
		Rows:    len(passingRows),
		Columns: make(map[string][]interface{}),
	}

	for colName, colData := range batch.Columns {
		newCol := make([]interface{}, len(passingRows))
		for j, rowIdx := range passingRows {
			if rowIdx < len(colData) {
				newCol[j] = colData[rowIdx]
			}
		}
		filtered.Columns[colName] = newCol
	}

	return filtered
}

// ProjectColumns returns a batch with only the selected columns.
func (pe *PredicateEvaluator) ProjectColumns(batch *RecordBatch, columns []string) *RecordBatch {
	if len(columns) == 0 || batch == nil {
		return batch
	}

	colSet := make(map[string]bool, len(columns))
	for _, c := range columns {
		colSet[c] = true
	}

	projected := &RecordBatch{
		Rows:    batch.Rows,
		Columns: make(map[string][]interface{}),
	}

	for _, s := range batch.Schema {
		if colSet[s.Name] {
			projected.Schema = append(projected.Schema, s)
			if data, ok := batch.Columns[s.Name]; ok {
				projected.Columns[s.Name] = data
			}
		}
	}

	return projected
}

func (pe *PredicateEvaluator) rowPassesAll(batch *RecordBatch, rowIdx int, predicates []Predicate) bool {
	for _, pred := range predicates {
		colData, ok := batch.Columns[pred.Column]
		if !ok {
			return false
		}
		if rowIdx >= len(colData) {
			return false
		}
		if !pe.evaluatePredicate(colData[rowIdx], pred) {
			return false
		}
	}
	return true
}

func (pe *PredicateEvaluator) evaluatePredicate(value interface{}, pred Predicate) bool {
	switch pred.Operator {
	case "not_null":
		return value != nil
	case "eq":
		return fmt.Sprintf("%v", value) == fmt.Sprintf("%v", pred.Value)
	case "ne":
		return fmt.Sprintf("%v", value) != fmt.Sprintf("%v", pred.Value)
	case "in":
		valStr := fmt.Sprintf("%v", value)
		for _, v := range pred.Values {
			if fmt.Sprintf("%v", v) == valStr {
				return true
			}
		}
		return false
	case "gt", "gte", "lt", "lte":
		return pe.compareNumeric(value, pred.Value, pred.Operator)
	default:
		return true
	}
}

func (pe *PredicateEvaluator) compareNumeric(a, b interface{}, op string) bool {
	av, aOk := toFloat(a)
	bv, bOk := toFloat(b)
	if !aOk || !bOk {
		return false
	}
	switch op {
	case "gt":
		return av > bv
	case "gte":
		return av >= bv
	case "lt":
		return av < bv
	case "lte":
		return av <= bv
	}
	return false
}

func toFloat(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	default:
		return 0, false
	}
}

// GetBatch performs a batch retrieval with predicate pushdown.
func (s *Server) GetBatch(ctx context.Context, req BatchRequest) (*BatchResponse, error) {
	if len(req.Descriptor.Features) == 0 {
		return nil, fmt.Errorf("at least one feature is required")
	}

	atomic.AddInt64(&s.activeStreams, 1)
	defer atomic.AddInt64(&s.activeStreams, -1)

	// Get flight info and create ticket
	info, err := s.GetFlightInfo(ctx, req.Descriptor)
	if err != nil {
		return nil, fmt.Errorf("getting flight info: %w", err)
	}

	if len(info.Endpoints) == 0 {
		return nil, fmt.Errorf("no endpoints available")
	}

	// Retrieve batch via DoGet
	batch, err := s.DoGet(ctx, info.Endpoints[0].Ticket)
	if err != nil {
		return nil, fmt.Errorf("retrieving batch: %w", err)
	}

	// Apply predicate pushdown
	evaluator := NewPredicateEvaluator()
	if len(req.Predicates) > 0 {
		batch = evaluator.Apply(batch, req.Predicates)
	}

	// Apply column projection
	if len(req.Columns) > 0 {
		batch = evaluator.ProjectColumns(batch, req.Columns)
	}

	// Apply limit/offset
	totalRows := int64(batch.Rows)
	if req.Offset > 0 && req.Offset < batch.Rows {
		for colName, colData := range batch.Columns {
			batch.Columns[colName] = colData[req.Offset:]
		}
		batch.Rows -= req.Offset
	}
	hasMore := false
	if req.Limit > 0 && batch.Rows > req.Limit {
		for colName, colData := range batch.Columns {
			batch.Columns[colName] = colData[:req.Limit]
		}
		batch.Rows = req.Limit
		hasMore = true
	}

	return &BatchResponse{
		Data:       batch,
		TotalRows:  totalRows,
		HasMore:    hasMore,
		NextOffset: req.Offset + batch.Rows,
	}, nil
}
