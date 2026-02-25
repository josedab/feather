package streamsql

import (
	"testing"
	"time"
)

func makeRecords(fields []map[string]interface{}, timestamps ...time.Time) []*Record {
	records := make([]*Record, len(fields))
	for i, f := range fields {
		ts := time.Now()
		if i < len(timestamps) {
			ts = timestamps[i]
		}
		records[i] = &Record{Fields: f, Timestamp: ts}
	}
	return records
}

func TestApplySlidingWindow_OverlappingWindows(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	records := makeRecords(
		[]map[string]interface{}{
			{"val": 1},
			{"val": 2},
			{"val": 3},
			{"val": 4},
		},
		base,
		base.Add(1*time.Second),
		base.Add(2*time.Second),
		base.Add(3*time.Second),
	)

	stmt := &Statement{
		Select: []*SelectExpr{{Expression: "*"}},
		Window: &WindowClause{
			Type:  WindowSliding,
			Size:  3 * time.Second,
			Slide: 1 * time.Second,
		},
	}

	qe := newQueryExecutor(stmt)
	result, err := qe.Execute(records)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.Count == 0 {
		t.Error("expected non-empty result from sliding window")
	}
}

func TestApplySlidingWindow_BoundaryTimestamps(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	records := makeRecords(
		[]map[string]interface{}{
			{"val": 1},
			{"val": 2},
		},
		base,
		base.Add(5*time.Second),
	)

	stmt := &Statement{
		Select: []*SelectExpr{{Expression: "*"}},
		Window: &WindowClause{
			Type:  WindowSliding,
			Size:  2 * time.Second,
			Slide: 1 * time.Second,
		},
	}

	qe := newQueryExecutor(stmt)
	result, err := qe.Execute(records)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	// Only the record at boundary end should be in window
	if result.Count == 0 {
		t.Error("expected at least one record in window")
	}
}

func TestApplyHaving_FilterByAggregate(t *testing.T) {
	stmt := &Statement{
		Select: []*SelectExpr{
			{Expression: "COUNT(*)", Function: "COUNT", Args: []string{"*"}, Alias: "cnt"},
		},
		GroupBy: []string{"category"},
		Having: &WhereClause{
			Conditions: []*Condition{
				{Field: "cnt", Operator: ">", Value: 1},
			},
		},
	}

	records := makeRecords([]map[string]interface{}{
		{"category": "A", "val": 1},
		{"category": "A", "val": 2},
		{"category": "B", "val": 3},
	})

	qe := newQueryExecutor(stmt)
	result, err := qe.Execute(records)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Only category A has count > 1
	for _, row := range result.Rows {
		cnt, _ := toNumeric(row["cnt"])
		if cnt <= 1 {
			t.Errorf("expected all rows to have cnt > 1, got %.0f", cnt)
		}
	}
}

func TestApplyOrderBy_SingleColumn_ASC(t *testing.T) {
	stmt := &Statement{
		Select: []*SelectExpr{{Expression: "name"}, {Expression: "score"}},
		OrderBy: []*OrderByExpr{
			{Field: "score", Desc: false},
		},
	}

	records := makeRecords([]map[string]interface{}{
		{"name": "C", "score": 30},
		{"name": "A", "score": 10},
		{"name": "B", "score": 20},
	})

	qe := newQueryExecutor(stmt)
	result, err := qe.Execute(records)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Count != 3 {
		t.Fatalf("expected 3 rows, got %d", result.Count)
	}

	scores := make([]float64, 3)
	for i, row := range result.Rows {
		scores[i], _ = toNumeric(row["score"])
	}
	if scores[0] > scores[1] || scores[1] > scores[2] {
		t.Errorf("expected ascending order, got %v", scores)
	}
}

func TestApplyOrderBy_SingleColumn_DESC(t *testing.T) {
	stmt := &Statement{
		Select: []*SelectExpr{{Expression: "score"}},
		OrderBy: []*OrderByExpr{
			{Field: "score", Desc: true},
		},
	}

	records := makeRecords([]map[string]interface{}{
		{"score": 10},
		{"score": 30},
		{"score": 20},
	})

	qe := newQueryExecutor(stmt)
	result, err := qe.Execute(records)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	scores := make([]float64, 3)
	for i, row := range result.Rows {
		scores[i], _ = toNumeric(row["score"])
	}
	if scores[0] < scores[1] || scores[1] < scores[2] {
		t.Errorf("expected descending order, got %v", scores)
	}
}

func TestApplyOrderBy_MultipleColumns(t *testing.T) {
	stmt := &Statement{
		Select: []*SelectExpr{{Expression: "dept"}, {Expression: "score"}},
		OrderBy: []*OrderByExpr{
			{Field: "dept", Desc: false},
			{Field: "score", Desc: true},
		},
	}

	records := makeRecords([]map[string]interface{}{
		{"dept": "B", "score": 10},
		{"dept": "A", "score": 20},
		{"dept": "A", "score": 30},
		{"dept": "B", "score": 40},
	})

	qe := newQueryExecutor(stmt)
	result, err := qe.Execute(records)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Count != 4 {
		t.Fatalf("expected 4 rows, got %d", result.Count)
	}
}

func TestApplyOrderBy_NilValues(t *testing.T) {
	stmt := &Statement{
		Select: []*SelectExpr{{Expression: "val"}},
		OrderBy: []*OrderByExpr{
			{Field: "val", Desc: false},
		},
	}

	records := makeRecords([]map[string]interface{}{
		{"val": 2},
		{},
		{"val": 1},
	})

	qe := newQueryExecutor(stmt)
	result, err := qe.Execute(records)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.Count != 3 {
		t.Errorf("expected 3 rows, got %d", result.Count)
	}
}

func TestCompareForSort_Numeric(t *testing.T) {
	tests := []struct {
		a, b interface{}
		want int
	}{
		{1, 2, -1},
		{2, 1, 1},
		{3, 3, 0},
		{1.5, 2.5, -1},
		{int64(10), float64(5), 1},
	}

	for _, tt := range tests {
		got := compareForSort(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("compareForSort(%v, %v) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestCompareForSort_String(t *testing.T) {
	tests := []struct {
		a, b interface{}
		want int
	}{
		{"apple", "banana", -1},
		{"banana", "apple", 1},
		{"same", "same", 0},
	}

	for _, tt := range tests {
		got := compareForSort(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("compareForSort(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestCompareForSort_MixedType(t *testing.T) {
	// Non-numeric falls back to string comparison
	got := compareForSort("abc", struct{}{})
	if got == 0 {
		t.Error("expected non-zero for mixed types")
	}
}

func TestMatchConditions_ORLogic(t *testing.T) {
	stmt := &Statement{
		Select: []*SelectExpr{{Expression: "*"}},
		Where: &WhereClause{
			Logic: "OR",
			Conditions: []*Condition{
				{Field: "status", Operator: "=", Value: "active"},
				{Field: "priority", Operator: ">", Value: 5},
			},
		},
	}

	records := makeRecords([]map[string]interface{}{
		{"status": "active", "priority": 1},
		{"status": "inactive", "priority": 10},
		{"status": "inactive", "priority": 1},
	})

	qe := newQueryExecutor(stmt)
	result, err := qe.Execute(records)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Count != 2 {
		t.Errorf("expected 2 matching rows (OR), got %d", result.Count)
	}
}

func TestMatchConditions_ANDLogic(t *testing.T) {
	stmt := &Statement{
		Select: []*SelectExpr{{Expression: "*"}},
		Where: &WhereClause{
			Logic: "AND",
			Conditions: []*Condition{
				{Field: "status", Operator: "=", Value: "active"},
				{Field: "priority", Operator: ">", Value: 5},
			},
		},
	}

	records := makeRecords([]map[string]interface{}{
		{"status": "active", "priority": 10},
		{"status": "active", "priority": 1},
		{"status": "inactive", "priority": 10},
	})

	qe := newQueryExecutor(stmt)
	result, err := qe.Execute(records)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Count != 1 {
		t.Errorf("expected 1 matching row (AND), got %d", result.Count)
	}
}

func TestMatchConditions_MissingField(t *testing.T) {
	stmt := &Statement{
		Select: []*SelectExpr{{Expression: "*"}},
		Where: &WhereClause{
			Conditions: []*Condition{
				{Field: "nonexistent", Operator: "=", Value: "x"},
			},
		},
	}

	records := makeRecords([]map[string]interface{}{
		{"name": "test"},
	})

	qe := newQueryExecutor(stmt)
	result, err := qe.Execute(records)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Count != 0 {
		t.Errorf("expected 0 rows for missing field, got %d", result.Count)
	}
}

func TestCompareValues_AllOperators(t *testing.T) {
	tests := []struct {
		left     interface{}
		op       string
		right    interface{}
		expected bool
	}{
		{10, "=", 10, true},
		{10, "=", 20, false},
		{10, "!=", 20, true},
		{10, "<>", 20, true},
		{10, "<", 20, true},
		{20, "<", 10, false},
		{10, "<=", 10, true},
		{10, "<=", 20, true},
		{20, ">", 10, true},
		{10, ">", 20, false},
		{20, ">=", 20, true},
		{20, ">=", 10, true},
		// String comparison
		{"abc", "=", "abc", true},
		{"abc", "<", "def", true},
		{"def", ">", "abc", true},
		{"abc", "!=", "def", true},
	}

	for _, tt := range tests {
		got := compareValues(tt.left, tt.op, tt.right)
		if got != tt.expected {
			t.Errorf("compareValues(%v, %s, %v) = %v, want %v", tt.left, tt.op, tt.right, got, tt.expected)
		}
	}
}

func TestToNumeric_EdgeCases(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected float64
		ok       bool
	}{
		{int(42), 42.0, true},
		{int64(100), 100.0, true},
		{float64(3.14), 3.14, true},
		{float32(2.5), 2.5, true},
		{"not a number", 0, false},
		{nil, 0, false},
		{true, 0, false},
	}

	for _, tt := range tests {
		got, ok := toNumeric(tt.input)
		if ok != tt.ok {
			t.Errorf("toNumeric(%v) ok = %v, want %v", tt.input, ok, tt.ok)
		}
		if ok && got != tt.expected {
			t.Errorf("toNumeric(%v) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestToFloat64_EdgeCases(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected float64
	}{
		{int(42), 42.0},
		{int64(100), 100.0},
		{float64(3.14), 3.14},
		{float32(2.5), 2.5},
		{"string", 0},
		{nil, 0},
	}

	for _, tt := range tests {
		got := toFloat64(tt.input)
		if got != tt.expected {
			t.Errorf("toFloat64(%v) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestExecutor_EmptyRecords(t *testing.T) {
	stmt := &Statement{
		Select: []*SelectExpr{{Expression: "*"}},
	}

	qe := newQueryExecutor(stmt)
	result, err := qe.Execute([]*Record{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.Count != 0 {
		t.Errorf("expected 0 rows, got %d", result.Count)
	}
}

func TestExecutor_GroupByWithAggregates(t *testing.T) {
	stmt := &Statement{
		Select: []*SelectExpr{
			{Expression: "SUM(amount)", Function: "SUM", Args: []string{"amount"}, Alias: "total"},
			{Expression: "COUNT(*)", Function: "COUNT", Args: []string{"*"}, Alias: "cnt"},
			{Expression: "MIN(amount)", Function: "MIN", Args: []string{"amount"}, Alias: "min_amt"},
			{Expression: "MAX(amount)", Function: "MAX", Args: []string{"amount"}, Alias: "max_amt"},
			{Expression: "AVG(amount)", Function: "AVG", Args: []string{"amount"}, Alias: "avg_amt"},
		},
		GroupBy: []string{"region"},
	}

	records := makeRecords([]map[string]interface{}{
		{"region": "US", "amount": 100},
		{"region": "US", "amount": 200},
		{"region": "EU", "amount": 50},
	})

	qe := newQueryExecutor(stmt)
	result, err := qe.Execute(records)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Count != 2 { // 2 groups
		t.Errorf("expected 2 groups, got %d", result.Count)
	}
}

func TestExecutor_Limit(t *testing.T) {
	stmt := &Statement{
		Select: []*SelectExpr{{Expression: "*"}},
		Limit:  2,
	}

	records := makeRecords([]map[string]interface{}{
		{"val": 1},
		{"val": 2},
		{"val": 3},
		{"val": 4},
	})

	qe := newQueryExecutor(stmt)
	result, err := qe.Execute(records)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Count != 2 {
		t.Errorf("expected 2 rows with limit, got %d", result.Count)
	}
}

func TestExecutor_WhereNil(t *testing.T) {
	stmt := &Statement{
		Select: []*SelectExpr{{Expression: "*"}},
		Where:  nil,
	}

	records := makeRecords([]map[string]interface{}{
		{"val": 1},
	})

	qe := newQueryExecutor(stmt)
	result, err := qe.Execute(records)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.Count != 1 {
		t.Errorf("expected 1 row, got %d", result.Count)
	}
}
