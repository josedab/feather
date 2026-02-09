package arrowflight

import (
	"fmt"
	"strings"
)

// DomainTypeMapping maps Feather domain types to Arrow data types.
var DomainTypeMapping = map[string]DataType{
	"int64":     DataTypeInt64,
	"float64":   DataTypeFloat64,
	"string":    DataTypeString,
	"bool":      DataTypeBool,
	"bytes":     DataTypeBytes,
	"vector":    DataTypeBytes,   // vectors serialized as bytes
	"timestamp": DataTypeInt64,   // nanosecond epoch
}

// ArrowSchema converts a list of feature specs (name -> domain type) to Arrow column schemas.
func ArrowSchema(features map[string]string) []ColumnSchema {
	schema := make([]ColumnSchema, 0, len(features)+1)
	// Entity key is always the first column
	schema = append(schema, ColumnSchema{Name: "entity_key", Type: DataTypeString, Nullable: false})

	for name, domainType := range features {
		arrowType, ok := DomainTypeMapping[domainType]
		if !ok {
			arrowType = DataTypeString // fallback
		}
		schema = append(schema, ColumnSchema{
			Name:     name,
			Type:     arrowType,
			Nullable: true,
		})
	}
	return schema
}

// BatchBuilder helps construct RecordBatch instances column-by-column.
type BatchBuilder struct {
	schema  []ColumnSchema
	columns map[string][]interface{}
	rows    int
}

// NewBatchBuilder creates a builder with the given schema.
func NewBatchBuilder(schema []ColumnSchema) *BatchBuilder {
	cols := make(map[string][]interface{}, len(schema))
	for _, col := range schema {
		cols[col.Name] = make([]interface{}, 0)
	}
	return &BatchBuilder{
		schema:  schema,
		columns: cols,
	}
}

// AddRow adds a row of values. The values map must match column names.
func (b *BatchBuilder) AddRow(values map[string]interface{}) error {
	for _, col := range b.schema {
		val, ok := values[col.Name]
		if !ok {
			if col.Nullable {
				val = nil
			} else {
				return fmt.Errorf("missing required column %q", col.Name)
			}
		}
		b.columns[col.Name] = append(b.columns[col.Name], val)
	}
	b.rows++
	return nil
}

// Build creates the RecordBatch.
func (b *BatchBuilder) Build() *RecordBatch {
	return &RecordBatch{
		Schema:  b.schema,
		Rows:    b.rows,
		Columns: b.columns,
	}
}

// Rows returns the current number of rows.
func (b *BatchBuilder) Rows() int {
	return b.rows
}

// PredicateFilter applies predicate pushdown to a RecordBatch, returning only
// rows that match all predicates.
func PredicateFilter(batch *RecordBatch, predicates []Predicate) *RecordBatch {
	if len(predicates) == 0 || batch == nil || batch.Rows == 0 {
		return batch
	}

	// Build column index for fast lookup
	colIdx := make(map[string]int)
	for i, col := range batch.Schema {
		colIdx[col.Name] = i
	}

	// Filter rows
	keep := make([]bool, batch.Rows)
	for i := range keep {
		keep[i] = true
	}

	for _, pred := range predicates {
		col, ok := batch.Columns[pred.Column]
		if !ok {
			continue
		}
		for i := 0; i < batch.Rows && i < len(col); i++ {
			if !keep[i] {
				continue
			}
			if !evalPredicate(col[i], pred) {
				keep[i] = false
			}
		}
	}

	// Build filtered batch
	filteredCols := make(map[string][]interface{}, len(batch.Columns))
	for name := range batch.Columns {
		filteredCols[name] = make([]interface{}, 0)
	}

	filteredRows := 0
	for i := 0; i < batch.Rows; i++ {
		if !keep[i] {
			continue
		}
		for name, col := range batch.Columns {
			if i < len(col) {
				filteredCols[name] = append(filteredCols[name], col[i])
			}
		}
		filteredRows++
	}

	return &RecordBatch{
		Schema:  batch.Schema,
		Rows:    filteredRows,
		Columns: filteredCols,
	}
}

func evalPredicate(val interface{}, pred Predicate) bool {
	if val == nil {
		return pred.Operator == "is_null"
	}

	switch pred.Operator {
	case "eq", "=", "==":
		return fmt.Sprintf("%v", val) == fmt.Sprintf("%v", pred.Value)
	case "neq", "!=":
		return fmt.Sprintf("%v", val) != fmt.Sprintf("%v", pred.Value)
	case "gt", ">":
		return compareNumeric(val, pred.Value) > 0
	case "gte", ">=":
		return compareNumeric(val, pred.Value) >= 0
	case "lt", "<":
		return compareNumeric(val, pred.Value) < 0
	case "lte", "<=":
		return compareNumeric(val, pred.Value) <= 0
	case "contains":
		return strings.Contains(fmt.Sprintf("%v", val), fmt.Sprintf("%v", pred.Value))
	case "is_null":
		return val == nil
	case "is_not_null":
		return val != nil
	default:
		return true
	}
}

func compareNumeric(a, b interface{}) int {
	af := toArrowFloat(a)
	bf := toArrowFloat(b)
	if af < bf {
		return -1
	}
	if af > bf {
		return 1
	}
	return 0
}

func toArrowFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case int32:
		return float64(val)
	default:
		return 0
	}
}

// ColumnProjection returns a new batch with only the selected columns.
func ColumnProjection(batch *RecordBatch, columns []string) *RecordBatch {
	if batch == nil {
		return nil
	}

	colSet := make(map[string]bool, len(columns))
	for _, c := range columns {
		colSet[c] = true
	}

	var schema []ColumnSchema
	projectedCols := make(map[string][]interface{})
	for _, col := range batch.Schema {
		if colSet[col.Name] {
			schema = append(schema, col)
			projectedCols[col.Name] = batch.Columns[col.Name]
		}
	}

	return &RecordBatch{
		Schema:  schema,
		Rows:    batch.Rows,
		Columns: projectedCols,
	}
}
