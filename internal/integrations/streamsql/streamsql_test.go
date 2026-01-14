package streamsql

import (
	"context"
	"testing"
	"time"
)

func TestLexer(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCount int
		wantErr   bool
	}{
		{
			name:      "simple select",
			input:     "SELECT * FROM clicks",
			wantCount: 4, // SELECT, *, FROM, clicks + EOF
		},
		{
			name:      "select with where",
			input:     "SELECT user_id FROM clicks WHERE count > 5",
			wantCount: 8, // SELECT, user_id, FROM, clicks, WHERE, count, >, 5 + EOF
		},
		{
			name:      "aggregate function",
			input:     "SELECT COUNT(*) FROM events",
			wantCount: 7, // SELECT, COUNT, (, *, ), FROM, events + EOF
		},
		{
			name:      "string literal",
			input:     "SELECT name FROM users WHERE status = 'active'",
			wantCount: 8, // SELECT, name, FROM, users, WHERE, status, =, 'active' + EOF
		},
		{
			name:    "unterminated string",
			input:   "SELECT * FROM users WHERE name = 'incomplete",
			wantErr: true,
		},
		{
			name:      "group by",
			input:     "SELECT user_id, COUNT(*) FROM events GROUP BY user_id",
			wantCount: 11, // SELECT, user_id, ",", COUNT, (, *, ), FROM, events, GROUP BY, user_id + EOF
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := NewLexer(tt.input)
			tokens, err := lexer.Tokenize()
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// count includes EOF
			got := len(tokens)
			if got != tt.wantCount+1 {
				t.Errorf("token count = %d, want %d (tokens: %v)", got, tt.wantCount+1, tokenValues(tokens))
			}
		})
	}
}

func TestParser(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		check   func(t *testing.T, stmt *Statement)
		wantErr bool
	}{
		{
			name:  "simple select star",
			input: "SELECT * FROM events",
			check: func(t *testing.T, stmt *Statement) {
				if len(stmt.Select) != 1 || stmt.Select[0].Expression != "*" {
					t.Error("expected SELECT *")
				}
				if stmt.From == nil || stmt.From.Stream != "events" {
					t.Error("expected FROM events")
				}
			},
		},
		{
			name:  "select with columns",
			input: "SELECT user_id, name FROM users",
			check: func(t *testing.T, stmt *Statement) {
				if len(stmt.Select) != 2 {
					t.Fatalf("expected 2 select expressions, got %d", len(stmt.Select))
				}
				if stmt.Select[0].Expression != "user_id" {
					t.Errorf("expected user_id, got %s", stmt.Select[0].Expression)
				}
				if stmt.Select[1].Expression != "name" {
					t.Errorf("expected name, got %s", stmt.Select[1].Expression)
				}
			},
		},
		{
			name:  "select with where",
			input: "SELECT * FROM clicks WHERE count > 5",
			check: func(t *testing.T, stmt *Statement) {
				if stmt.Where == nil {
					t.Fatal("expected WHERE clause")
				}
				if len(stmt.Where.Conditions) != 1 {
					t.Fatalf("expected 1 condition, got %d", len(stmt.Where.Conditions))
				}
				cond := stmt.Where.Conditions[0]
				if cond.Field != "count" || cond.Operator != ">" {
					t.Errorf("unexpected condition: %+v", cond)
				}
			},
		},
		{
			name:  "group by with aggregate",
			input: "SELECT user_id, COUNT(*) FROM events GROUP BY user_id",
			check: func(t *testing.T, stmt *Statement) {
				if len(stmt.GroupBy) != 1 || stmt.GroupBy[0] != "user_id" {
					t.Errorf("expected GROUP BY user_id, got %v", stmt.GroupBy)
				}
				hasAgg := false
				for _, sel := range stmt.Select {
					if sel.Function == "COUNT" {
						hasAgg = true
					}
				}
				if !hasAgg {
					t.Error("expected COUNT aggregate")
				}
			},
		},
		{
			name:  "window tumble",
			input: "SELECT * FROM events WINDOW TUMBLE(ts, '1m')",
			check: func(t *testing.T, stmt *Statement) {
				if stmt.Window == nil {
					t.Fatal("expected WINDOW clause")
				}
				if stmt.Window.Type != WindowTumbling {
					t.Errorf("expected tumbling window, got %s", stmt.Window.Type)
				}
				if stmt.Window.Size != time.Minute {
					t.Errorf("expected 1m window size, got %v", stmt.Window.Size)
				}
				if stmt.Window.Field != "ts" {
					t.Errorf("expected field ts, got %s", stmt.Window.Field)
				}
			},
		},
		{
			name:  "window slide",
			input: "SELECT * FROM events WINDOW SLIDE(ts, '5m', '1m')",
			check: func(t *testing.T, stmt *Statement) {
				if stmt.Window == nil {
					t.Fatal("expected WINDOW clause")
				}
				if stmt.Window.Type != WindowSliding {
					t.Errorf("expected sliding window, got %s", stmt.Window.Type)
				}
				if stmt.Window.Size != 5*time.Minute {
					t.Errorf("expected 5m window size, got %v", stmt.Window.Size)
				}
				if stmt.Window.Slide != time.Minute {
					t.Errorf("expected 1m slide, got %v", stmt.Window.Slide)
				}
			},
		},
		{
			name:  "order by and limit",
			input: "SELECT name FROM users ORDER BY name LIMIT 10",
			check: func(t *testing.T, stmt *Statement) {
				if len(stmt.OrderBy) != 1 || stmt.OrderBy[0].Field != "name" {
					t.Error("expected ORDER BY name")
				}
				if stmt.Limit != 10 {
					t.Errorf("expected LIMIT 10, got %d", stmt.Limit)
				}
			},
		},
		{
			name:    "missing FROM",
			input:   "SELECT *",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := NewLexer(tt.input)
			tokens, err := lexer.Tokenize()
			if err != nil {
				t.Fatalf("lexer error: %v", err)
			}

			parser := NewParser(tokens)
			stmt, err := parser.Parse()
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, stmt)
			}
		})
	}
}

func TestEngineCreateStream(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	defer engine.Close()

	schema := map[string]string{
		"user_id": "string",
		"count":   "int",
	}

	if err := engine.CreateStream("clicks", schema); err != nil {
		t.Fatalf("CreateStream failed: %v", err)
	}

	// Duplicate should fail
	if err := engine.CreateStream("clicks", schema); err == nil {
		t.Error("expected error for duplicate stream")
	}

	streams := engine.ListStreams()
	if len(streams) != 1 {
		t.Fatalf("expected 1 stream, got %d", len(streams))
	}
	if streams[0].Name != "clicks" {
		t.Errorf("expected stream name 'clicks', got %q", streams[0].Name)
	}

	// Drop stream
	if err := engine.DropStream("clicks"); err != nil {
		t.Fatalf("DropStream failed: %v", err)
	}
	if len(engine.ListStreams()) != 0 {
		t.Error("expected 0 streams after drop")
	}

	// Drop non-existent should fail
	if err := engine.DropStream("nonexistent"); err == nil {
		t.Error("expected error for dropping non-existent stream")
	}
}

func TestEnginePushAndQuery(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	defer engine.Close()

	schema := map[string]string{"user_id": "string", "value": "int"}
	if err := engine.CreateStream("events", schema); err != nil {
		t.Fatalf("CreateStream failed: %v", err)
	}

	now := time.Now()
	records := []Record{
		{Fields: map[string]interface{}{"user_id": "u1", "value": 10}, Timestamp: now},
		{Fields: map[string]interface{}{"user_id": "u2", "value": 20}, Timestamp: now.Add(time.Second)},
		{Fields: map[string]interface{}{"user_id": "u1", "value": 30}, Timestamp: now.Add(2 * time.Second)},
	}
	for i := range records {
		if err := engine.Push("events", &records[i]); err != nil {
			t.Fatalf("Push failed: %v", err)
		}
	}

	// Push to non-existent stream should fail
	if err := engine.Push("nonexistent", &Record{Fields: map[string]interface{}{}}); err == nil {
		t.Error("expected error for pushing to non-existent stream")
	}

	ctx := context.Background()
	result, err := engine.ExecuteQuery(ctx, "SELECT * FROM events")
	if err != nil {
		t.Fatalf("ExecuteQuery failed: %v", err)
	}
	if result.Count != 3 {
		t.Errorf("expected 3 rows, got %d", result.Count)
	}

	// Query specific columns
	result, err = engine.ExecuteQuery(ctx, "SELECT user_id FROM events")
	if err != nil {
		t.Fatalf("ExecuteQuery failed: %v", err)
	}
	if result.Count != 3 {
		t.Errorf("expected 3 rows, got %d", result.Count)
	}

	// Verify stats
	stats := engine.Stats()
	if stats.StreamCount != 1 {
		t.Errorf("expected 1 stream, got %d", stats.StreamCount)
	}
	if stats.TotalRecords != 3 {
		t.Errorf("expected 3 total records, got %d", stats.TotalRecords)
	}
}

func TestAggregation(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	defer engine.Close()

	schema := map[string]string{"category": "string", "amount": "int"}
	if err := engine.CreateStream("sales", schema); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	data := []struct {
		cat string
		amt int
	}{
		{"A", 10},
		{"B", 20},
		{"A", 30},
		{"B", 40},
		{"A", 50},
	}
	for i, d := range data {
		rec := &Record{
			Fields:    map[string]interface{}{"category": d.cat, "amount": d.amt},
			Timestamp: now.Add(time.Duration(i) * time.Second),
		}
		if err := engine.Push("sales", rec); err != nil {
			t.Fatal(err)
		}
	}

	ctx := context.Background()

	t.Run("COUNT", func(t *testing.T) {
		result, err := engine.ExecuteQuery(ctx, "SELECT category, COUNT(*) FROM sales GROUP BY category")
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		if result.Count != 2 {
			t.Fatalf("expected 2 groups, got %d", result.Count)
		}
		for _, row := range result.Rows {
			cat := row["category"].(string)
			count := row["COUNT(*)"].(int64)
			switch cat {
			case "A":
				if count != 3 {
					t.Errorf("A count = %d, want 3", count)
				}
			case "B":
				if count != 2 {
					t.Errorf("B count = %d, want 2", count)
				}
			}
		}
	})

	t.Run("SUM", func(t *testing.T) {
		result, err := engine.ExecuteQuery(ctx, "SELECT category, SUM(amount) FROM sales GROUP BY category")
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range result.Rows {
			cat := row["category"].(string)
			sum := row["SUM(amount)"].(float64)
			switch cat {
			case "A":
				if sum != 90 {
					t.Errorf("A sum = %f, want 90", sum)
				}
			case "B":
				if sum != 60 {
					t.Errorf("B sum = %f, want 60", sum)
				}
			}
		}
	})

	t.Run("AVG", func(t *testing.T) {
		result, err := engine.ExecuteQuery(ctx, "SELECT category, AVG(amount) FROM sales GROUP BY category")
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range result.Rows {
			cat := row["category"].(string)
			avg := row["AVG(amount)"].(float64)
			switch cat {
			case "A":
				if avg != 30 {
					t.Errorf("A avg = %f, want 30", avg)
				}
			case "B":
				if avg != 30 {
					t.Errorf("B avg = %f, want 30", avg)
				}
			}
		}
	})

	t.Run("MIN_MAX", func(t *testing.T) {
		result, err := engine.ExecuteQuery(ctx, "SELECT category, MIN(amount) FROM sales GROUP BY category")
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range result.Rows {
			cat := row["category"].(string)
			minVal := row["MIN(amount)"].(float64)
			switch cat {
			case "A":
				if minVal != 10 {
					t.Errorf("A min = %f, want 10", minVal)
				}
			case "B":
				if minVal != 20 {
					t.Errorf("B min = %f, want 20", minVal)
				}
			}
		}
	})
}

func TestWindowTumble(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	defer engine.Close()

	schema := map[string]string{"ts": "timestamp", "value": "int"}
	if err := engine.CreateStream("metrics", schema); err != nil {
		t.Fatal(err)
	}

	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	// Push records within a 1-minute window
	for i := 0; i < 5; i++ {
		rec := &Record{
			Fields:    map[string]interface{}{"value": (i + 1) * 10},
			Timestamp: base.Add(time.Duration(i*10) * time.Second),
		}
		if err := engine.Push("metrics", rec); err != nil {
			t.Fatal(err)
		}
	}

	ctx := context.Background()
	result, err := engine.ExecuteQuery(ctx, "SELECT * FROM metrics WINDOW TUMBLE(ts, '1m')")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if result.Count != 5 {
		t.Errorf("expected 5 records in window, got %d", result.Count)
	}
}

func TestWhereFilter(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	defer engine.Close()

	schema := map[string]string{"name": "string", "score": "int"}
	if err := engine.CreateStream("players", schema); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	players := []struct {
		name  string
		score int
	}{
		{"alice", 100},
		{"bob", 50},
		{"charlie", 75},
		{"diana", 90},
	}
	for i, p := range players {
		rec := &Record{
			Fields:    map[string]interface{}{"name": p.name, "score": p.score},
			Timestamp: now.Add(time.Duration(i) * time.Second),
		}
		if err := engine.Push("players", rec); err != nil {
			t.Fatal(err)
		}
	}

	ctx := context.Background()

	t.Run("greater than", func(t *testing.T) {
		result, err := engine.ExecuteQuery(ctx, "SELECT * FROM players WHERE score > 70")
		if err != nil {
			t.Fatal(err)
		}
		if result.Count != 3 {
			t.Errorf("expected 3 rows with score > 70, got %d", result.Count)
		}
	})

	t.Run("equals string", func(t *testing.T) {
		result, err := engine.ExecuteQuery(ctx, "SELECT * FROM players WHERE name = 'alice'")
		if err != nil {
			t.Fatal(err)
		}
		if result.Count != 1 {
			t.Errorf("expected 1 row for alice, got %d", result.Count)
		}
	})

	t.Run("less than or equal", func(t *testing.T) {
		result, err := engine.ExecuteQuery(ctx, "SELECT * FROM players WHERE score <= 75")
		if err != nil {
			t.Fatal(err)
		}
		if result.Count != 2 {
			t.Errorf("expected 2 rows with score <= 75, got %d", result.Count)
		}
	})
}

func TestRegisterQuery(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	defer engine.Close()

	schema := map[string]string{"user_id": "string", "action": "string"}
	if err := engine.CreateStream("events", schema); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	rq, err := engine.RegisterQuery(ctx, "active_users", "SELECT user_id FROM events WHERE action = 'login'")
	if err != nil {
		t.Fatalf("RegisterQuery failed: %v", err)
	}

	if rq.Name != "active_users" {
		t.Errorf("expected name 'active_users', got %q", rq.Name)
	}
	if rq.Status != QueryStatusActive {
		t.Errorf("expected status active, got %q", rq.Status)
	}

	// Duplicate should fail
	_, err = engine.RegisterQuery(ctx, "active_users", "SELECT * FROM events")
	if err == nil {
		t.Error("expected error for duplicate query registration")
	}

	queries := engine.ListQueries()
	if len(queries) != 1 {
		t.Fatalf("expected 1 query, got %d", len(queries))
	}

	// Unregister
	if err := engine.UnregisterQuery("active_users"); err != nil {
		t.Fatalf("UnregisterQuery failed: %v", err)
	}
	if len(engine.ListQueries()) != 0 {
		t.Error("expected 0 queries after unregister")
	}

	// Unregister non-existent should fail
	if err := engine.UnregisterQuery("nonexistent"); err == nil {
		t.Error("expected error for unregistering non-existent query")
	}
}

// tokenValues returns the string values of all tokens for debugging.
func tokenValues(tokens []Token) []string {
	var vals []string
	for _, t := range tokens {
		vals = append(vals, t.Value)
	}
	return vals
}

func TestEngine_PauseResumeQuery(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	defer engine.Close()

	_ = engine.CreateStream("events", map[string]string{"user": "string"})

	_, err := engine.RegisterQuery(context.Background(), "q1", "SELECT user FROM events")
	if err != nil {
		t.Fatalf("RegisterQuery failed: %v", err)
	}

	// Pause
	if err := engine.PauseQuery("q1"); err != nil {
		t.Fatalf("PauseQuery failed: %v", err)
	}
	q, _ := engine.GetQuery("q1")
	if q.Status != QueryStatusPaused {
		t.Errorf("expected paused, got %s", q.Status)
	}

	// Pause again should fail
	if err := engine.PauseQuery("q1"); err == nil {
		t.Error("expected error pausing already-paused query")
	}

	// Resume
	if err := engine.ResumeQuery("q1"); err != nil {
		t.Fatalf("ResumeQuery failed: %v", err)
	}
	q, _ = engine.GetQuery("q1")
	if q.Status != QueryStatusActive {
		t.Errorf("expected active, got %s", q.Status)
	}

	// Non-existent
	if err := engine.PauseQuery("nonexistent"); err == nil {
		t.Error("expected error for non-existent query")
	}
}

func TestEngine_PushBatch(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	defer engine.Close()

	_ = engine.CreateStream("logs", map[string]string{"level": "string", "msg": "string"})

	records := []*Record{
		{Fields: map[string]interface{}{"level": "info", "msg": "hello"}},
		{Fields: map[string]interface{}{"level": "error", "msg": "oops"}},
		{Fields: map[string]interface{}{"level": "info", "msg": "world"}},
	}

	pushed, err := engine.PushBatch("logs", records)
	if err != nil {
		t.Fatalf("PushBatch failed: %v", err)
	}
	if pushed != 3 {
		t.Errorf("expected 3 pushed, got %d", pushed)
	}

	info, _ := engine.GetStream("logs")
	if info.RecordCount != 3 {
		t.Errorf("expected 3 records, got %d", info.RecordCount)
	}

	// Non-existent stream
	_, err = engine.PushBatch("nonexistent", records)
	if err == nil {
		t.Error("expected error for non-existent stream")
	}
}

func TestEngine_GetStream(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	defer engine.Close()

	_ = engine.CreateStream("test", map[string]string{"a": "string"})

	info, err := engine.GetStream("test")
	if err != nil {
		t.Fatalf("GetStream failed: %v", err)
	}
	if info.Name != "test" {
		t.Errorf("expected name 'test', got %s", info.Name)
	}

	_, err = engine.GetStream("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent stream")
	}
}
