package featherqlv2

import (
	"testing"
)

func TestTokenize(t *testing.T) {
	tokens, err := Tokenize("SELECT avg(amount) AS avg_spend, count(*) FROM transactions WHERE user_id = '123'")
	if err != nil {
		t.Fatalf("tokenize error: %v", err)
	}
	// Should have: SELECT, avg, (, amount, ), AS, avg_spend, ,, count, (, *, ), FROM, transactions, WHERE, user_id, =, '123', EOF
	if len(tokens) < 15 {
		t.Errorf("expected at least 15 tokens, got %d", len(tokens))
	}
	if tokens[0].Type != TokenKeyword || tokens[0].Value != "SELECT" {
		t.Errorf("expected SELECT keyword, got %+v", tokens[0])
	}
}

func TestTokenizeOperators(t *testing.T) {
	tokens, err := Tokenize("a >= 10 AND b != 'x'")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tok := range tokens {
		if tok.Type == TokenOp && tok.Value == ">=" {
			found = true
		}
	}
	if !found {
		t.Error("expected >= operator token")
	}
}

func TestTokenizeError(t *testing.T) {
	_, err := Tokenize("SELECT 'unterminated")
	if err == nil {
		t.Error("expected error for unterminated string")
	}
}

func TestParseSimpleSelect(t *testing.T) {
	stmt, err := ParseQuery("SELECT name, age FROM users")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(stmt.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(stmt.Columns))
	}
	if stmt.From != "users" {
		t.Errorf("expected FROM users, got %q", stmt.From)
	}
}

func TestParseWithWhere(t *testing.T) {
	stmt, err := ParseQuery("SELECT score FROM features WHERE entity_id = '123'")
	if err != nil {
		t.Fatal(err)
	}
	if stmt.Where == nil {
		t.Fatal("expected WHERE clause")
	}
	if stmt.Where.Raw == "" {
		t.Error("expected non-empty WHERE raw text")
	}
}

func TestParseAggregates(t *testing.T) {
	stmt, err := ParseQuery("SELECT avg(amount) AS avg_spend, count(*) AS total FROM transactions")
	if err != nil {
		t.Fatal(err)
	}
	if len(stmt.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(stmt.Columns))
	}
	if !stmt.Columns[0].IsAgg {
		t.Error("expected first column to be aggregate")
	}
	if stmt.Columns[0].AggFunc != "AVG" {
		t.Errorf("expected AVG agg func, got %q", stmt.Columns[0].AggFunc)
	}
	if stmt.Columns[0].Alias != "avg_spend" {
		t.Errorf("expected alias avg_spend, got %q", stmt.Columns[0].Alias)
	}
}

func TestParseWindowFunction(t *testing.T) {
	stmt, err := ParseQuery("SELECT avg(amount) OVER (PARTITION BY user_id) AS avg_spend FROM transactions")
	if err != nil {
		t.Fatal(err)
	}
	if !stmt.HasWindow {
		t.Error("expected HasWindow to be true")
	}
	if !stmt.Columns[0].IsWindow {
		t.Error("expected column to be a window function")
	}
}

func TestParseGroupBy(t *testing.T) {
	stmt, err := ParseQuery("SELECT category, count(*) AS cnt FROM products GROUP BY category ORDER BY cnt DESC LIMIT 10")
	if err != nil {
		t.Fatal(err)
	}
	if len(stmt.GroupBy) != 1 || stmt.GroupBy[0] != "category" {
		t.Errorf("expected GROUP BY category, got %v", stmt.GroupBy)
	}
	if len(stmt.OrderBy) != 1 || !stmt.OrderBy[0].Desc {
		t.Errorf("expected ORDER BY cnt DESC, got %v", stmt.OrderBy)
	}
	if stmt.Limit != 10 {
		t.Errorf("expected LIMIT 10, got %d", stmt.Limit)
	}
}

func TestParseJoin(t *testing.T) {
	stmt, err := ParseQuery("SELECT name, score FROM users LEFT JOIN features ON users.id = features.entity_id")
	if err != nil {
		t.Fatal(err)
	}
	if len(stmt.Joins) != 1 {
		t.Fatalf("expected 1 join, got %d", len(stmt.Joins))
	}
	if stmt.Joins[0].Type != "LEFT" {
		t.Errorf("expected LEFT join, got %q", stmt.Joins[0].Type)
	}
	if stmt.Joins[0].Table != "features" {
		t.Errorf("expected join table features, got %q", stmt.Joins[0].Table)
	}
}

func TestParseInvalidQuery(t *testing.T) {
	_, err := ParseQuery("INSERT INTO table VALUES (1)")
	if err == nil {
		t.Error("expected error for non-SELECT query")
	}
}
