package arrowflight

import (
	"context"
	"testing"
)

// mockReader implements FeatureReader for testing.
type mockReader struct {
	batch *RecordBatch
	err   error
}

func (m *mockReader) ReadBatch(_ context.Context, entities []string, features []string) (*RecordBatch, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.batch != nil {
		return m.batch, nil
	}
	cols := make(map[string][]interface{})
	cols["entity_key"] = make([]interface{}, len(entities))
	for i, e := range entities {
		cols["entity_key"][i] = e
	}
	for _, f := range features {
		cols[f] = make([]interface{}, len(entities))
	}
	return &RecordBatch{
		Rows:    len(entities),
		Columns: cols,
	}, nil
}

// mockWriter implements FeatureWriter for testing.
type mockWriter struct {
	result *PutResult
	err    error
}

func (m *mockWriter) WriteBatch(_ context.Context, batch *RecordBatch) (*PutResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.result != nil {
		return m.result, nil
	}
	return &PutResult{RowsWritten: int64(batch.Rows)}, nil
}

func TestNewServer(t *testing.T) {
	cfg := DefaultConfig()
	srv := NewServer(cfg)
	if srv == nil {
		t.Fatal("NewServer returned nil")
	}
	stats := srv.Stats()
	if stats.TotalDoGets != 0 || stats.TotalDoPuts != 0 {
		t.Errorf("fresh server should have zero stats, got gets=%d puts=%d", stats.TotalDoGets, stats.TotalDoPuts)
	}
}

func TestGetFlightInfo(t *testing.T) {
	tests := []struct {
		name    string
		desc    FlightDescriptor
		wantErr bool
	}{
		{
			name: "valid descriptor",
			desc: FlightDescriptor{
				Type:     "path",
				Entities: []string{"user:1"},
				Features: []string{"clicks", "views"},
			},
		},
		{
			name:    "no features",
			desc:    FlightDescriptor{Type: "path", Entities: []string{"user:1"}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := NewServer(DefaultConfig())
			info, err := srv.GetFlightInfo(context.Background(), tt.desc)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(info.Schema) != len(tt.desc.Features)+1 {
				t.Errorf("schema columns = %d, want %d", len(info.Schema), len(tt.desc.Features)+1)
			}
			if len(info.Endpoints) == 0 {
				t.Error("expected at least one endpoint")
			}
		})
	}
}

func TestDoGet(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*Server)
		wantErr bool
	}{
		{
			name: "with reader",
			setup: func(s *Server) {
				s.SetReader(&mockReader{})
			},
		},
		{
			name:  "without reader returns empty batch",
			setup: func(s *Server) {},
		},
		{
			name: "invalid ticket",
			setup: func(s *Server) {
				// do not issue a ticket
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := NewServer(DefaultConfig())
			tt.setup(srv)

			if tt.wantErr {
				_, err := srv.DoGet(context.Background(), FlightTicket{ID: "nonexistent"})
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			desc := FlightDescriptor{
				Type:     "path",
				Entities: []string{"user:1"},
				Features: []string{"clicks"},
			}
			info, err := srv.GetFlightInfo(context.Background(), desc)
			if err != nil {
				t.Fatalf("GetFlightInfo: %v", err)
			}

			batch, err := srv.DoGet(context.Background(), info.Endpoints[0].Ticket)
			if err != nil {
				t.Fatalf("DoGet: %v", err)
			}
			if batch == nil {
				t.Fatal("DoGet returned nil batch")
			}
		})
	}
}

func TestDoPut(t *testing.T) {
	tests := []struct {
		name    string
		batch   *RecordBatch
		wantErr bool
	}{
		{
			name: "valid batch",
			batch: &RecordBatch{
				Rows:    2,
				Columns: map[string][]interface{}{"clicks": {10, 20}},
			},
		},
		{
			name:    "nil batch",
			batch:   nil,
			wantErr: true,
		},
		{
			name:    "empty batch",
			batch:   &RecordBatch{Rows: 0},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := NewServer(DefaultConfig())
			desc := FlightDescriptor{Type: "path", Features: []string{"clicks"}}

			result, err := srv.DoPut(context.Background(), desc, tt.batch)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.RowsWritten != int64(tt.batch.Rows) {
				t.Errorf("RowsWritten = %d, want %d", result.RowsWritten, tt.batch.Rows)
			}
		})
	}
}

func TestDoExchange(t *testing.T) {
	tests := []struct {
		name    string
		req     ExchangeRequest
		wantErr bool
	}{
		{
			name: "query exchange",
			req: ExchangeRequest{
				Command:    "query",
				Descriptor: FlightDescriptor{Features: []string{"clicks"}, Entities: []string{"u1"}},
			},
		},
		{
			name: "transform exchange",
			req: ExchangeRequest{
				Command: "transform",
				Data:    &RecordBatch{Rows: 1, Columns: map[string][]interface{}{"a": {1}}},
			},
		},
		{
			name: "aggregate exchange",
			req: ExchangeRequest{
				Command: "aggregate",
				Data:    &RecordBatch{Rows: 2, Columns: map[string][]interface{}{"a": {1, 2}}},
			},
		},
		{
			name:    "unknown command",
			req:     ExchangeRequest{Command: "invalid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := NewServer(DefaultConfig())
			resp, err := srv.DoExchange(context.Background(), tt.req)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.Status != "ok" {
				t.Errorf("status = %q, want %q", resp.Status, "ok")
			}
		})
	}
}

func TestListFlights(t *testing.T) {
	srv := NewServer(DefaultConfig())

	if flights := srv.ListFlights(); len(flights) != 0 {
		t.Errorf("expected 0 flights, got %d", len(flights))
	}

	desc := FlightDescriptor{Type: "path", Entities: []string{"u1"}, Features: []string{"f1"}}
	if _, err := srv.GetFlightInfo(context.Background(), desc); err != nil {
		t.Fatalf("GetFlightInfo: %v", err)
	}

	flights := srv.ListFlights()
	if len(flights) != 1 {
		t.Errorf("expected 1 flight, got %d", len(flights))
	}
}

func TestStats(t *testing.T) {
	srv := NewServer(DefaultConfig())

	desc := FlightDescriptor{Type: "path", Entities: []string{"u1"}, Features: []string{"f1"}}
	if _, err := srv.GetFlightInfo(context.Background(), desc); err != nil {
		t.Fatal(err)
	}
	info, _ := srv.GetFlightInfo(context.Background(), desc)
	if _, err := srv.DoGet(context.Background(), info.Endpoints[0].Ticket); err != nil {
		t.Fatal(err)
	}

	batch := &RecordBatch{Rows: 3, Columns: map[string][]interface{}{"f1": {1, 2, 3}}}
	if _, err := srv.DoPut(context.Background(), desc, batch); err != nil {
		t.Fatal(err)
	}

	stats := srv.Stats()
	if stats.TotalDoGets < 1 {
		t.Errorf("TotalDoGets = %d, want >= 1", stats.TotalDoGets)
	}
	if stats.TotalDoPuts < 1 {
		t.Errorf("TotalDoPuts = %d, want >= 1", stats.TotalDoPuts)
	}
}
