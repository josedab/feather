package arrowflight

import (
	"fmt"
	"sync"
)

// ColumnBuilder accumulates values for a single column in columnar format.
type ColumnBuilder struct {
	Name     string
	DataType DataType
	Values   []interface{}
	Nulls    []bool
}

// ColumnStats tracks statistics for a column.
type ColumnStats struct {
	NullCount   int         `json:"null_count"`
	MinValue    interface{} `json:"min_value,omitempty"`
	MaxValue    interface{} `json:"max_value,omitempty"`
	Cardinality int         `json:"cardinality,omitempty"`
}

// Column represents a fully built column with values and statistics.
type Column struct {
	Name     string        `json:"name"`
	DataType DataType      `json:"data_type"`
	Values   []interface{} `json:"values"`
	NullMap  []bool        `json:"null_map,omitempty"`
	Stats    ColumnStats   `json:"stats"`
}

// ColumnarBatch holds data in columnar layout for efficient batch serving.
type ColumnarBatch struct {
	Columns  []*Column         `json:"columns"`
	RowCount int               `json:"row_count"`
	ByteSize int64             `json:"byte_size"`
	Schema   []ColumnSchema    `json:"schema"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// NewColumnBuilder creates a builder for the given column name and data type.
func NewColumnBuilder(name string, dataType DataType) *ColumnBuilder {
	return &ColumnBuilder{
		Name:     name,
		DataType: dataType,
		Values:   make([]interface{}, 0),
		Nulls:    make([]bool, 0),
	}
}

// Append adds a non-null value to the column.
func (cb *ColumnBuilder) Append(value interface{}) {
	cb.Values = append(cb.Values, value)
	cb.Nulls = append(cb.Nulls, false)
}

// AppendNull adds a null value to the column.
func (cb *ColumnBuilder) AppendNull() {
	cb.Values = append(cb.Values, nil)
	cb.Nulls = append(cb.Nulls, true)
}

// Build finalizes the column builder into an immutable Column with computed stats.
func (cb *ColumnBuilder) Build() *Column {
	col := &Column{
		Name:     cb.Name,
		DataType: cb.DataType,
		Values:   cb.Values,
		NullMap:  cb.Nulls,
		Stats:    computeStats(cb.Values, cb.Nulls),
	}

	// Only include NullMap if there are nulls
	hasNulls := false
	for _, n := range cb.Nulls {
		if n {
			hasNulls = true
			break
		}
	}
	if !hasNulls {
		col.NullMap = nil
	}

	return col
}

func computeStats(values []interface{}, nulls []bool) ColumnStats {
	stats := ColumnStats{}
	distinct := make(map[string]struct{})

	for i, v := range values {
		if i < len(nulls) && nulls[i] {
			stats.NullCount++
			continue
		}
		if v == nil {
			stats.NullCount++
			continue
		}

		distinct[fmt.Sprintf("%v", v)] = struct{}{}

		fv, ok := toFloat(v)
		if ok {
			if stats.MinValue == nil {
				stats.MinValue = fv
				stats.MaxValue = fv
			} else {
				if minF, mOk := toFloat(stats.MinValue); mOk && fv < minF {
					stats.MinValue = fv
				}
				if maxF, mOk := toFloat(stats.MaxValue); mOk && fv > maxF {
					stats.MaxValue = fv
				}
			}
		}
	}

	stats.Cardinality = len(distinct)
	return stats
}

// BatchConverter converts between row-oriented and columnar data formats.
type BatchConverter struct {
	mu sync.Mutex
}

// NewBatchConverter creates a new batch converter.
func NewBatchConverter() *BatchConverter {
	return &BatchConverter{}
}

// FromRows converts row-oriented feature data to a ColumnarBatch.
// entities is the list of entity keys, features maps entity -> feature_name -> value.
func (bc *BatchConverter) FromRows(entities []string, features map[string]map[string]interface{}, schema []ColumnSchema) (*ColumnarBatch, error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if len(schema) == 0 {
		return nil, fmt.Errorf("schema must not be empty")
	}

	builders := make(map[string]*ColumnBuilder, len(schema))
	for _, cs := range schema {
		builders[cs.Name] = NewColumnBuilder(cs.Name, cs.Type)
	}

	for _, entity := range entities {
		row := features[entity]
		for _, cs := range schema {
			if cs.Name == "entity_key" {
				builders[cs.Name].Append(entity)
				continue
			}
			if row == nil {
				builders[cs.Name].AppendNull()
				continue
			}
			val, exists := row[cs.Name]
			if !exists || val == nil {
				builders[cs.Name].AppendNull()
			} else {
				builders[cs.Name].Append(val)
			}
		}
	}

	columns := make([]*Column, 0, len(schema))
	var byteSize int64
	for _, cs := range schema {
		col := builders[cs.Name].Build()
		columns = append(columns, col)
		byteSize += estimateColumnSize(col)
	}

	return &ColumnarBatch{
		Columns:  columns,
		RowCount: len(entities),
		ByteSize: byteSize,
		Schema:   schema,
		Metadata: map[string]string{},
	}, nil
}

// ToRows converts a ColumnarBatch back to row-oriented maps.
func (bc *BatchConverter) ToRows(batch *ColumnarBatch) ([]map[string]interface{}, error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if batch == nil {
		return nil, fmt.Errorf("batch must not be nil")
	}

	colMap := make(map[string]*Column, len(batch.Columns))
	for _, col := range batch.Columns {
		colMap[col.Name] = col
	}

	rows := make([]map[string]interface{}, batch.RowCount)
	for i := 0; i < batch.RowCount; i++ {
		row := make(map[string]interface{}, len(batch.Columns))
		for _, col := range batch.Columns {
			if i < len(col.Values) {
				isNull := i < len(col.NullMap) && col.NullMap[i]
				if !isNull {
					row[col.Name] = col.Values[i]
				} else {
					row[col.Name] = nil
				}
			}
		}
		rows[i] = row
	}

	return rows, nil
}

// ProjectColumns returns a new batch with only the specified columns.
func (bc *BatchConverter) ProjectColumns(batch *ColumnarBatch, columns []string) (*ColumnarBatch, error) {
	if batch == nil {
		return nil, fmt.Errorf("batch must not be nil")
	}
	if len(columns) == 0 {
		return batch, nil
	}

	colSet := make(map[string]bool, len(columns))
	for _, c := range columns {
		colSet[c] = true
	}

	var projected []*Column
	var schema []ColumnSchema
	var byteSize int64

	for i, col := range batch.Columns {
		if colSet[col.Name] {
			projected = append(projected, col)
			if i < len(batch.Schema) {
				schema = append(schema, batch.Schema[i])
			}
			byteSize += estimateColumnSize(col)
		}
	}

	return &ColumnarBatch{
		Columns:  projected,
		RowCount: batch.RowCount,
		ByteSize: byteSize,
		Schema:   schema,
		Metadata: batch.Metadata,
	}, nil
}

// ToRecordBatch converts a ColumnarBatch to the existing RecordBatch format.
func (bc *BatchConverter) ToRecordBatch(batch *ColumnarBatch) *RecordBatch {
	if batch == nil {
		return nil
	}

	cols := make(map[string][]interface{}, len(batch.Columns))
	for _, col := range batch.Columns {
		cols[col.Name] = col.Values
	}

	return &RecordBatch{
		Schema:  batch.Schema,
		Rows:    batch.RowCount,
		Columns: cols,
	}
}

// FromRecordBatch converts a RecordBatch to a ColumnarBatch with computed stats.
func (bc *BatchConverter) FromRecordBatch(rb *RecordBatch) *ColumnarBatch {
	if rb == nil {
		return nil
	}

	columns := make([]*Column, 0, len(rb.Schema))
	var byteSize int64

	for _, cs := range rb.Schema {
		values := rb.Columns[cs.Name]
		if values == nil {
			values = make([]interface{}, rb.Rows)
		}

		nulls := make([]bool, len(values))
		for i, v := range values {
			if v == nil {
				nulls[i] = true
			}
		}

		col := &Column{
			Name:     cs.Name,
			DataType: cs.Type,
			Values:   values,
			Stats:    computeStats(values, nulls),
		}

		// Only set NullMap if there are nulls
		hasNulls := false
		for _, n := range nulls {
			if n {
				hasNulls = true
				break
			}
		}
		if hasNulls {
			col.NullMap = nulls
		}

		columns = append(columns, col)
		byteSize += estimateColumnSize(col)
	}

	return &ColumnarBatch{
		Columns:  columns,
		RowCount: rb.Rows,
		ByteSize: byteSize,
		Schema:   rb.Schema,
		Metadata: map[string]string{},
	}
}

func estimateColumnSize(col *Column) int64 {
	var size int64
	for _, v := range col.Values {
		switch val := v.(type) {
		case string:
			size += int64(len(val))
		case []byte:
			size += int64(len(val))
		case int64, float64:
			size += 8
		case int32, float32:
			size += 4
		case int:
			size += 8
		case bool:
			size++
		case nil:
			// no size for nulls
		default:
			size += 8 // default estimate
		}
	}
	return size
}
