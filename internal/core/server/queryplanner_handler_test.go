package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/queryplanner"
)

type testQueryPlannerServer struct {
	handler *QueryPlannerHandler
	planner *queryplanner.Planner
	mux     *http.ServeMux
	t       *testing.T
}

func newTestQueryPlannerServer(t *testing.T) *testQueryPlannerServer {
	t.Helper()
	planner := queryplanner.New(queryplanner.DefaultConfig())
	handler := NewQueryPlannerHandler(planner)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return &testQueryPlannerServer{handler: handler, planner: planner, mux: mux, t: t}
}

func (ts *testQueryPlannerServer) request(method, path string, body string) *httptest.ResponseRecorder {
	ts.t.Helper()
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)
	return rr
}

func TestQueryPlannerHandler_GetStats(t *testing.T) {
	ts := newTestQueryPlannerServer(t)
	rr := ts.request(http.MethodGet, "/v1/planner/stats", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
}

func TestQueryPlannerHandler_Optimize(t *testing.T) {
	ts := newTestQueryPlannerServer(t)
	body := `{"ID":"q1","Operations":[{"Type":"scan","Feature":"clicks"}],"Features":["clicks"],"Entities":["user:123"]}`
	rr := ts.request(http.MethodPost, "/v1/planner/optimize", body)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestQueryPlannerHandler_Optimize_InvalidJSON(t *testing.T) {
	ts := newTestQueryPlannerServer(t)
	rr := ts.request(http.MethodPost, "/v1/planner/optimize", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestQueryPlannerHandler_RecordCost(t *testing.T) {
	ts := newTestQueryPlannerServer(t)
	body := `{"op_type":"scan","duration_ms":15.5,"row_count":1000}`
	rr := ts.request(http.MethodPost, "/v1/planner/cost", body)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
	var result SuccessResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if !result.Success {
		t.Error("Expected success to be true")
	}
}

func TestQueryPlannerHandler_RecordCost_MissingOpType(t *testing.T) {
	ts := newTestQueryPlannerServer(t)
	body := `{"op_type":"","duration_ms":15.5,"row_count":1000}`
	rr := ts.request(http.MethodPost, "/v1/planner/cost", body)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestQueryPlannerHandler_ShouldReplan(t *testing.T) {
	ts := newTestQueryPlannerServer(t)
	rr := ts.request(http.MethodGet, "/v1/planner/replan/plan-123", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if _, ok := result["plan_id"]; !ok {
		t.Error("Expected plan_id key in response")
	}
}

func TestQueryPlannerHandler_RecordResult_InvalidJSON(t *testing.T) {
	ts := newTestQueryPlannerServer(t)
	rr := ts.request(http.MethodPost, "/v1/planner/result", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}
