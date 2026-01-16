package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/feather-store/feather/internal/extensions/joins"
)

type testJoinsServer struct {
	handler *JoinsHandler
	mux     *http.ServeMux
	t       *testing.T
}

func newTestJoinsServer(t *testing.T) *testJoinsServer {
	t.Helper()
	engine := joins.NewEngine(joins.DefaultEngineConfig())
	handler := NewJoinsHandler(engine)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return &testJoinsServer{handler: handler, mux: mux, t: t}
}

func (ts *testJoinsServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func TestJoinsHandler_CreateAndListPlans(t *testing.T) {
	ts := newTestJoinsServer(t)

	body := `{"left_entity":"users","right_entity":"transactions","join_key":"user_id","join_type":0,"window":300000000000,"watermark":60000000000}`
	rr := ts.request("POST", "/v1/joins/plans", body)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var plan joins.JoinPlan
	if err := json.NewDecoder(rr.Body).Decode(&plan); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if plan.ID == "" {
		t.Error("plan ID should not be empty")
	}

	// List plans
	rr = ts.request("GET", "/v1/joins/plans", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var plans []joins.JoinPlan
	if err := json.NewDecoder(rr.Body).Decode(&plans); err != nil {
		t.Fatalf("decoding plans: %v", err)
	}
	if len(plans) != 1 {
		t.Errorf("expected 1 plan, got %d", len(plans))
	}
}

func TestJoinsHandler_GetPlan(t *testing.T) {
	ts := newTestJoinsServer(t)

	body := `{"left_entity":"users","right_entity":"txns","join_key":"uid"}`
	rr := ts.request("POST", "/v1/joins/plans", body)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
	var plan joins.JoinPlan
	json.NewDecoder(rr.Body).Decode(&plan)

	rr = ts.request("GET", "/v1/joins/plans/"+plan.ID, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// Not found
	rr = ts.request("GET", "/v1/joins/plans/nonexistent", "")
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestJoinsHandler_DeletePlan(t *testing.T) {
	ts := newTestJoinsServer(t)

	body := `{"left_entity":"a","right_entity":"b","join_key":"k"}`
	rr := ts.request("POST", "/v1/joins/plans", body)
	var plan joins.JoinPlan
	json.NewDecoder(rr.Body).Decode(&plan)

	rr = ts.request("DELETE", "/v1/joins/plans/"+plan.ID, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	rr = ts.request("DELETE", "/v1/joins/plans/nonexistent", "")
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestJoinsHandler_ExecuteJoin(t *testing.T) {
	ts := newTestJoinsServer(t)

	// Create plan
	body := `{"left_entity":"users","right_entity":"txns","join_key":"uid","join_type":0,"window":600000000000,"watermark":3600000000000}`
	rr := ts.request("POST", "/v1/joins/plans", body)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var plan joins.JoinPlan
	json.NewDecoder(rr.Body).Decode(&plan)

	now := time.Now().UnixMilli()
	execBody := fmt.Sprintf(`{
		"left_data": {
			"e1": {"age": {"value": 25, "timestamp": %d}},
			"e2": {"age": {"value": 30, "timestamp": %d}}
		},
		"right_data": {
			"e1": {"amount": {"value": 100, "timestamp": %d}}
		}
	}`, now, now, now)

	rr = ts.request("POST", "/v1/joins/execute/"+plan.ID, execBody)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var output joins.JoinOutput
	if err := json.NewDecoder(rr.Body).Decode(&output); err != nil {
		t.Fatalf("decoding output: %v", err)
	}
	if len(output.Results) != 1 {
		t.Errorf("expected 1 result for inner join, got %d", len(output.Results))
	}
}

func TestJoinsHandler_InvalidBody(t *testing.T) {
	ts := newTestJoinsServer(t)

	rr := ts.request("POST", "/v1/joins/plans", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid body, got %d", rr.Code)
	}
}
