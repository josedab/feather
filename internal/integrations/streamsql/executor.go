package streamsql

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// QueryExecutor processes records through a parsed query plan.
type QueryExecutor struct {
	statement *Statement
	state     *executorState
	mu        sync.Mutex
}

type executorState struct {
	windowBuckets map[string]*windowBucket
	groupStates   map[string]*groupState
	records       []*Record
}

type windowBucket struct {
}

type groupState struct {
	key    string
	count  int64
	sum    float64
	min    float64
	max    float64
	hasMin bool
	hasMax bool
	values map[string]interface{}
}

// newQueryExecutor creates a new executor for the given statement.
func newQueryExecutor(stmt *Statement) *QueryExecutor {
	return &QueryExecutor{
		statement: stmt,
		state: &executorState{
			windowBuckets: make(map[string]*windowBucket),
			groupStates:   make(map[string]*groupState),
		},
	}
}

// Execute runs the query against the provided records and returns results.
func (qe *QueryExecutor) Execute(records []*Record) (*QueryResult, error) {
	qe.mu.Lock()
	defer qe.mu.Unlock()

	qe.state.records = records

	// Apply WHERE filter
	filtered := qe.applyWhere(records)

	// Apply window if present
	if qe.statement.Window != nil {
		filtered = qe.applyWindow(filtered)
	}

	// Apply GROUP BY + aggregation or plain projection
	var rows []map[string]interface{}
	var columns []string
	var err error

	if len(qe.statement.GroupBy) > 0 || hasAggregates(qe.statement.Select) {
		rows, columns, err = qe.applyGroupBy(filtered)
		if err != nil {
			return nil, fmt.Errorf("executing GROUP BY: %w", err)
		}
	} else {
		rows, columns = qe.applyProjection(filtered)
	}

	// Apply HAVING
	if qe.statement.Having != nil {
		rows = qe.applyHaving(rows)
	}

	// Apply ORDER BY
	if len(qe.statement.OrderBy) > 0 {
		qe.applyOrderBy(rows)
	}

	// Apply LIMIT
	if qe.statement.Limit > 0 && len(rows) > qe.statement.Limit {
		rows = rows[:qe.statement.Limit]
	}

	return &QueryResult{
		Columns: columns,
		Rows:    rows,
		Count:   len(rows),
	}, nil
}

func (qe *QueryExecutor) applyWhere(records []*Record) []*Record {
	if qe.statement.Where == nil {
		return records
	}

	var result []*Record
	for _, rec := range records {
		if qe.matchConditions(rec.Fields, qe.statement.Where) {
			result = append(result, rec)
		}
	}
	return result
}

func (qe *QueryExecutor) matchConditions(fields map[string]interface{}, where *WhereClause) bool {
	if where == nil || len(where.Conditions) == 0 {
		return true
	}

	if where.Logic == "OR" {
		for _, cond := range where.Conditions {
			if qe.matchCondition(fields, cond) {
				return true
			}
		}
		return false
	}

	// AND logic (default)
	for _, cond := range where.Conditions {
		if !qe.matchCondition(fields, cond) {
			return false
		}
	}
	return true
}

func (qe *QueryExecutor) matchCondition(fields map[string]interface{}, cond *Condition) bool {
	val, ok := fields[cond.Field]
	if !ok {
		return false
	}
	return compareValues(val, cond.Operator, cond.Value)
}

func (qe *QueryExecutor) applyWindow(records []*Record) []*Record {
	w := qe.statement.Window
	if len(records) == 0 {
		return records
	}

	// Find time range
	minTime := records[0].Timestamp
	maxTime := records[0].Timestamp
	for _, r := range records[1:] {
		if r.Timestamp.Before(minTime) {
			minTime = r.Timestamp
		}
		if r.Timestamp.After(maxTime) {
			maxTime = r.Timestamp
		}
	}

	switch w.Type {
	case WindowTumbling:
		return qe.applyTumblingWindow(records, minTime, maxTime, w.Size)
	case WindowSliding:
		slide := w.Slide
		if slide == 0 {
			slide = w.Size / 2
		}
		return qe.applySlidingWindow(records, minTime, maxTime, w.Size, slide)
	default:
		return records
	}
}

func (qe *QueryExecutor) applyTumblingWindow(records []*Record, minTime, maxTime time.Time, size time.Duration) []*Record {
	// Assign records to the latest complete window bucket
	bucketStart := minTime.Truncate(size)
	bucketEnd := bucketStart.Add(size)

	// Find the window that contains the most records or use the latest
	if maxTime.After(bucketEnd) {
		// Multiple windows; return all records in range
		return records
	}

	var result []*Record
	for _, r := range records {
		if !r.Timestamp.Before(bucketStart) && r.Timestamp.Before(bucketEnd) {
			result = append(result, r)
		}
	}
	if len(result) == 0 {
		return records
	}
	return result
}

func (qe *QueryExecutor) applySlidingWindow(records []*Record, minTime, maxTime time.Time, size, slide time.Duration) []*Record {
	// Return records in the latest sliding window
	windowEnd := maxTime
	windowStart := windowEnd.Add(-size)

	var result []*Record
	for _, r := range records {
		if !r.Timestamp.Before(windowStart) && !r.Timestamp.After(windowEnd) {
			result = append(result, r)
		}
	}
	if len(result) == 0 {
		return records
	}
	return result
}

func (qe *QueryExecutor) applyGroupBy(records []*Record) ([]map[string]interface{}, []string, error) {
	groups := make(map[string]*groupState)

	for _, rec := range records {
		key := qe.groupKey(rec)
		gs, ok := groups[key]
		if !ok {
			gs = &groupState{
				key:    key,
				values: make(map[string]interface{}),
			}
			// Store group-by field values
			for _, field := range qe.statement.GroupBy {
				gs.values[field] = rec.Fields[field]
			}
			groups[key] = gs
		}
		gs.count++

		// Update aggregation state for each aggregate select expression
		for _, sel := range qe.statement.Select {
			if sel.Function == "" {
				continue
			}
			if len(sel.Args) == 0 {
				continue
			}
			arg := sel.Args[0]
			if arg == "*" {
				continue
			}
			numVal := toFloat64(rec.Fields[arg])
			aggKey := sel.Expression
			switch sel.Function {
			case "SUM", "AVG":
				gs.sum += numVal
			case "MIN":
				if !gs.hasMin || numVal < gs.min {
					gs.min = numVal
					gs.hasMin = true
				}
			case "MAX":
				if !gs.hasMax || numVal > gs.max {
					gs.max = numVal
					gs.hasMax = true
				}
			case "COUNT":
				// count is tracked above
			default:
				return nil, nil, fmt.Errorf("unknown aggregate function: %s", aggKey)
			}
		}
	}

	// Build result columns
	var columns []string
	for _, field := range qe.statement.GroupBy {
		columns = append(columns, field)
	}
	for _, sel := range qe.statement.Select {
		if sel.Function != "" {
			name := sel.Alias
			if name == "" {
				name = sel.Expression
			}
			columns = append(columns, name)
		}
	}

	// Build rows
	var rows []map[string]interface{}
	for _, gs := range groups {
		row := make(map[string]interface{})
		for k, v := range gs.values {
			row[k] = v
		}
		for _, sel := range qe.statement.Select {
			if sel.Function == "" {
				continue
			}
			name := sel.Alias
			if name == "" {
				name = sel.Expression
			}
			switch sel.Function {
			case "COUNT":
				row[name] = gs.count
			case "SUM":
				row[name] = gs.sum
			case "AVG":
				if gs.count > 0 {
					row[name] = gs.sum / float64(gs.count)
				} else {
					row[name] = float64(0)
				}
			case "MIN":
				row[name] = gs.min
			case "MAX":
				row[name] = gs.max
			}
		}
		rows = append(rows, row)
	}

	return rows, columns, nil
}

func (qe *QueryExecutor) applyProjection(records []*Record) ([]map[string]interface{}, []string) {
	if len(records) == 0 {
		return nil, nil
	}

	isStar := len(qe.statement.Select) == 1 && qe.statement.Select[0].Expression == "*"

	var columns []string
	if isStar {
		// Gather all unique column names
		seen := make(map[string]bool)
		for _, rec := range records {
			for k := range rec.Fields {
				if !seen[k] {
					seen[k] = true
					columns = append(columns, k)
				}
			}
		}
		sort.Strings(columns)
	} else {
		for _, sel := range qe.statement.Select {
			name := sel.Alias
			if name == "" {
				name = sel.Expression
			}
			columns = append(columns, name)
		}
	}

	var rows []map[string]interface{}
	for _, rec := range records {
		row := make(map[string]interface{})
		if isStar {
			for k, v := range rec.Fields {
				row[k] = v
			}
		} else {
			for _, sel := range qe.statement.Select {
				name := sel.Alias
				if name == "" {
					name = sel.Expression
				}
				row[name] = rec.Fields[sel.Expression]
			}
		}
		rows = append(rows, row)
	}
	return rows, columns
}

func (qe *QueryExecutor) applyHaving(rows []map[string]interface{}) []map[string]interface{} {
	if qe.statement.Having == nil {
		return rows
	}
	var result []map[string]interface{}
	for _, row := range rows {
		if qe.matchConditions(row, qe.statement.Having) {
			result = append(result, row)
		}
	}
	return result
}

func (qe *QueryExecutor) applyOrderBy(rows []map[string]interface{}) {
	sort.SliceStable(rows, func(i, j int) bool {
		for _, ob := range qe.statement.OrderBy {
			vi := rows[i][ob.Field]
			vj := rows[j][ob.Field]
			cmp := compareForSort(vi, vj)
			if cmp == 0 {
				continue
			}
			if ob.Desc {
				return cmp > 0
			}
			return cmp < 0
		}
		return false
	})
}

func (qe *QueryExecutor) groupKey(rec *Record) string {
	if len(qe.statement.GroupBy) == 0 {
		return "_all"
	}
	var parts []string
	for _, field := range qe.statement.GroupBy {
		parts = append(parts, fmt.Sprintf("%v", rec.Fields[field]))
	}
	return strings.Join(parts, "|")
}

func hasAggregates(exprs []*SelectExpr) bool {
	for _, e := range exprs {
		if e.Function != "" {
			return true
		}
	}
	return false
}

func toFloat64(v interface{}) float64 {
	switch n := v.(type) {
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case float64:
		return n
	case float32:
		return float64(n)
	default:
		return 0
	}
}

func compareValues(left interface{}, op string, right interface{}) bool {
	// Try numeric comparison
	lf, lok := toNumeric(left)
	rf, rok := toNumeric(right)
	if lok && rok {
		switch op {
		case "=":
			return lf == rf
		case "!=", "<>":
			return lf != rf
		case "<":
			return lf < rf
		case "<=":
			return lf <= rf
		case ">":
			return lf > rf
		case ">=":
			return lf >= rf
		}
	}

	// Fall back to string comparison
	ls := fmt.Sprintf("%v", left)
	rs := fmt.Sprintf("%v", right)
	switch op {
	case "=":
		return ls == rs
	case "!=", "<>":
		return ls != rs
	case "<":
		return ls < rs
	case "<=":
		return ls <= rs
	case ">":
		return ls > rs
	case ">=":
		return ls >= rs
	}
	return false
}

func toNumeric(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case float32:
		return float64(n), true
	default:
		return 0, false
	}
}

func compareForSort(a, b interface{}) int {
	af, aok := toNumeric(a)
	bf, bok := toNumeric(b)
	if aok && bok {
		if af < bf {
			return -1
		}
		if af > bf {
			return 1
		}
		return 0
	}
	as := fmt.Sprintf("%v", a)
	bs := fmt.Sprintf("%v", b)
	if as < bs {
		return -1
	}
	if as > bs {
		return 1
	}
	return 0
}
