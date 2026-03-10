package featherqlv2

import (
	"testing"
	"time"
)

func TestParseCreateStream(t *testing.T) {
	p := NewStreamingParser()

	t.Run("with schema", func(t *testing.T) {
		stmt, err := p.Parse("CREATE STREAM clicks (user_id STRING, amount FLOAT, ts TIMESTAMP)")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stmt.Type != StmtCreateStream {
			t.Fatalf("expected CREATE_STREAM, got %s", stmt.Type)
		}
		if stmt.StreamName != "clicks" {
			t.Fatalf("expected stream name 'clicks', got %s", stmt.StreamName)
		}
		if len(stmt.Schema) != 3 {
			t.Fatalf("expected 3 columns, got %d", len(stmt.Schema))
		}
		if stmt.Schema["user_id"] != "string" {
			t.Fatalf("expected user_id type 'string', got %s", stmt.Schema["user_id"])
		}
		if stmt.Schema["amount"] != "float" {
			t.Fatalf("expected amount type 'float', got %s", stmt.Schema["amount"])
		}
	})

	t.Run("without schema", func(t *testing.T) {
		stmt, err := p.Parse("CREATE STREAM events")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stmt.StreamName != "events" {
			t.Fatalf("expected stream name 'events', got %s", stmt.StreamName)
		}
		if len(stmt.Schema) != 0 {
			t.Fatalf("expected empty schema, got %d columns", len(stmt.Schema))
		}
	})

	t.Run("with watermark", func(t *testing.T) {
		stmt, err := p.Parse("CREATE STREAM events (ts TIMESTAMP, val FLOAT) WATERMARK FOR ts AS 5s")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stmt.WatermarkSpec == nil {
			t.Fatal("expected watermark spec")
		}
		if stmt.WatermarkSpec.Column != "ts" {
			t.Fatalf("expected watermark column 'ts', got %s", stmt.WatermarkSpec.Column)
		}
		if stmt.WatermarkSpec.MaxDelay != 5*time.Second {
			t.Fatalf("expected watermark delay 5s, got %v", stmt.WatermarkSpec.MaxDelay)
		}
	})

	t.Run("empty name", func(t *testing.T) {
		_, err := p.Parse("CREATE STREAM ")
		if err == nil {
			t.Fatal("expected error for empty stream name")
		}
	})

	t.Run("missing closing paren", func(t *testing.T) {
		_, err := p.Parse("CREATE STREAM foo (col1 STRING")
		if err == nil {
			t.Fatal("expected error for missing closing parenthesis")
		}
	})
}

func TestParseDropStream(t *testing.T) {
	p := NewStreamingParser()

	t.Run("basic", func(t *testing.T) {
		stmt, err := p.Parse("DROP STREAM clicks")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stmt.Type != StmtDropStream {
			t.Fatalf("expected DROP_STREAM, got %s", stmt.Type)
		}
		if stmt.StreamName != "clicks" {
			t.Fatalf("expected stream name 'clicks', got %s", stmt.StreamName)
		}
	})

	t.Run("with semicolon", func(t *testing.T) {
		stmt, err := p.Parse("DROP STREAM events;")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stmt.StreamName != "events" {
			t.Fatalf("expected stream name 'events', got %s", stmt.StreamName)
		}
	})

	t.Run("empty name", func(t *testing.T) {
		_, err := p.Parse("DROP STREAM ")
		if err == nil {
			t.Fatal("expected error for empty stream name")
		}
	})
}

func TestParseStreamSelectTumbling(t *testing.T) {
	p := NewStreamingParser()
	stmt, err := p.Parse("SELECT user_id, COUNT(*) FROM clicks GROUP BY user_id TUMBLING(5m) EMIT CHANGES")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stmt.Type != StmtSelectStream {
		t.Fatalf("expected SELECT_STREAM, got %s", stmt.Type)
	}
	if stmt.Window == nil {
		t.Fatal("expected window spec")
	}
	if stmt.Window.Type != "tumbling" {
		t.Fatalf("expected tumbling window, got %s", stmt.Window.Type)
	}
	if stmt.Window.Size != 5*time.Minute {
		t.Fatalf("expected 5m window size, got %v", stmt.Window.Size)
	}
	if stmt.EmitMode != "changes" {
		t.Fatalf("expected emit mode 'changes', got %s", stmt.EmitMode)
	}
	if len(stmt.GroupByKeys) != 1 || stmt.GroupByKeys[0] != "user_id" {
		t.Fatalf("expected GROUP BY [user_id], got %v", stmt.GroupByKeys)
	}
}

func TestParseStreamSelectSliding(t *testing.T) {
	p := NewStreamingParser()
	stmt, err := p.Parse("SELECT region, AVG(latency) FROM requests GROUP BY region SLIDING(10m) EMIT FINAL")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stmt.Window == nil {
		t.Fatal("expected window spec")
	}
	if stmt.Window.Type != "sliding" {
		t.Fatalf("expected sliding window, got %s", stmt.Window.Type)
	}
	if stmt.Window.Size != 10*time.Minute {
		t.Fatalf("expected 10m window size, got %v", stmt.Window.Size)
	}
	if stmt.EmitMode != "final" {
		t.Fatalf("expected emit mode 'final', got %s", stmt.EmitMode)
	}
}

func TestParseStreamSelectSession(t *testing.T) {
	p := NewStreamingParser()
	stmt, err := p.Parse("SELECT user_id, SUM(amount) FROM purchases GROUP BY user_id SESSION(30m) EMIT CHANGES")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stmt.Window == nil {
		t.Fatal("expected window spec")
	}
	if stmt.Window.Type != "session" {
		t.Fatalf("expected session window, got %s", stmt.Window.Type)
	}
	if stmt.Window.Gap != 30*time.Minute {
		t.Fatalf("expected 30m session gap, got %v", stmt.Window.Gap)
	}
}

func TestParseStreamSelectWithWatermark(t *testing.T) {
	p := NewStreamingParser()
	stmt, err := p.Parse("SELECT user_id, COUNT(*) FROM events GROUP BY user_id TUMBLING(1m) WATERMARK FOR event_time AS 10s EMIT CHANGES")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stmt.WatermarkSpec == nil {
		t.Fatal("expected watermark spec")
	}
	if stmt.WatermarkSpec.Column != "event_time" {
		t.Fatalf("expected watermark column 'event_time', got %s", stmt.WatermarkSpec.Column)
	}
	if stmt.WatermarkSpec.MaxDelay != 10*time.Second {
		t.Fatalf("expected watermark delay 10s, got %v", stmt.WatermarkSpec.MaxDelay)
	}
}

func TestParseRegularSelect(t *testing.T) {
	p := NewStreamingParser()
	stmt, err := p.Parse("SELECT name, age FROM users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stmt.Type != StmtSelectStream {
		t.Fatalf("expected SELECT_STREAM, got %s", stmt.Type)
	}
	if stmt.Select == nil {
		t.Fatal("expected parsed select statement")
	}
	if stmt.Select.From != "users" {
		t.Fatalf("expected FROM 'users', got %s", stmt.Select.From)
	}
}
