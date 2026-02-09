package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestObservabilityConsole_RecordFreshness(t *testing.T) {
	oc := NewObservabilityConsole(DefaultConsoleConfig())
	oc.RecordFreshness("click_count", "user_features", time.Now().Add(-5*time.Second), 10000)

	heatmap := oc.GetFreshnessHeatmap()
	if len(heatmap) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(heatmap))
	}
	if heatmap[0].HeatmapColor != "green" {
		t.Errorf("expected green, got %s", heatmap[0].HeatmapColor)
	}
}

func TestObservabilityConsole_RecordLatency(t *testing.T) {
	oc := NewObservabilityConsole(DefaultConsoleConfig())
	oc.RecordLatency("/v1/features", 0.5, 2.0, 5.0, 100, 0.01)

	sparklines := oc.GetLatencySparklines()
	if len(sparklines) != 1 {
		t.Fatalf("expected 1 sparkline, got %d", len(sparklines))
	}
	if sparklines[0].P50Ms != 0.5 {
		t.Errorf("expected P50 0.5, got %f", sparklines[0].P50Ms)
	}
}

func TestObservabilityConsole_CostAttribution(t *testing.T) {
	oc := NewObservabilityConsole(DefaultConsoleConfig())
	oc.RecordCost("user_features", "feature", 0.10, 0.05, 0.02)

	costs := oc.GetCostAttribution()
	if len(costs) != 1 {
		t.Fatalf("expected 1 cost entry, got %d", len(costs))
	}
	if costs[0].CostPerHour != 0.17 {
		t.Errorf("expected cost 0.17, got %f", costs[0].CostPerHour)
	}
}

func TestObservabilityConsole_PipelineDAG(t *testing.T) {
	oc := NewObservabilityConsole(DefaultConsoleConfig())
	oc.SetPipelineNode(PipelineNode{
		ID:     "source-1",
		Name:   "kafka-source",
		Type:   "source",
		Status: "running",
	})

	dag := oc.GetPipelineDAG()
	if len(dag) != 1 {
		t.Fatalf("expected 1 node, got %d", len(dag))
	}
}

func TestObservabilityConsole_AlertChannels(t *testing.T) {
	oc := NewObservabilityConsole(DefaultConsoleConfig())
	oc.RegisterAlertChannel(AlertChannel{
		Type:     "slack",
		Name:     "test-slack",
		Endpoint: "https://hooks.slack.com/test",
		Enabled:  true,
	})

	mux := http.NewServeMux()
	oc.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/v1/observability/channels", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestObservabilityConsole_AddChannel(t *testing.T) {
	oc := NewObservabilityConsole(DefaultConsoleConfig())
	mux := http.NewServeMux()
	oc.RegisterRoutes(mux)

	body := `{"type":"webhook","name":"test-hook","endpoint":"https://example.com/hook","enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/observability/channels", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestObservabilityConsole_Overview(t *testing.T) {
	oc := NewObservabilityConsole(DefaultConsoleConfig())
	oc.RecordFreshness("f1", "g1", time.Now(), 10000)
	oc.RecordLatency("/v1/features", 1.0, 2.0, 5.0, 50, 0.0)

	mux := http.NewServeMux()
	oc.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/v1/observability/overview", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestHeatmapColor(t *testing.T) {
	tests := []struct {
		staleness int64
		sla       int64
		want      string
	}{
		{1000, 10000, "green"},
		{8000, 10000, "yellow"},
		{15000, 10000, "orange"},
		{25000, 10000, "red"},
	}
	for _, tt := range tests {
		got := computeHeatmapColor(tt.staleness, tt.sla)
		if got != tt.want {
			t.Errorf("staleness=%d sla=%d: got %s, want %s", tt.staleness, tt.sla, got, tt.want)
		}
	}
}
