package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/feastcompat"
)

func newTestFeastGatewayHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	adapter := feastcompat.NewAdapter(feastcompat.DefaultAdapterConfig())
	gateway := feastcompat.NewGateway(adapter)
	handler := NewFeastGatewayHandler(gateway)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestFeastGatewayHandler_Push(t *testing.T) {
	mux := newTestFeastGatewayHandler(t)

	body := `{"push_source_name":"events","df":[{"user_id":"u1","value":10}]}`
	req := httptest.NewRequest("POST", "/v1/feast/push", strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("POST /v1/feast/push = %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestFeastGatewayHandler_Apply(t *testing.T) {
	mux := newTestFeastGatewayHandler(t)

	body := `{"entities":[{"name":"user","value_type":"STRING"}],"feature_views":[{"name":"user_features","entities":["user"],"schema":[{"name":"age","dtype":"INT64"}],"online":true}],"feature_services":[{"name":"user_svc","feature_views":["user_features"]}]}`
	req := httptest.NewRequest("POST", "/v1/feast/apply", strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("POST /v1/feast/apply = %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestFeastGatewayHandler_ListServices(t *testing.T) {
	mux := newTestFeastGatewayHandler(t)

	req := httptest.NewRequest("GET", "/v1/feast/services", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /v1/feast/services = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestFeastGatewayHandler_GetServiceNotFound(t *testing.T) {
	mux := newTestFeastGatewayHandler(t)

	req := httptest.NewRequest("GET", "/v1/feast/services/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("GET nonexistent service = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestFeastGatewayHandler_SaveAndGetDataset(t *testing.T) {
	mux := newTestFeastGatewayHandler(t)

	body := `{"name":"training_v1","feature_service":"user_svc","entity_df":[{"user_id":"u1"}]}`
	req := httptest.NewRequest("POST", "/v1/feast/datasets", strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("POST /v1/feast/datasets = %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	req = httptest.NewRequest("GET", "/v1/feast/datasets/training_v1", nil)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET dataset = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestFeastGatewayHandler_InvalidJSON(t *testing.T) {
	mux := newTestFeastGatewayHandler(t)

	req := httptest.NewRequest("POST", "/v1/feast/push", strings.NewReader("invalid"))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST with bad JSON = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}
