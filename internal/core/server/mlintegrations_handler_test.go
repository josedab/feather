package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/integrations/airflow"
	"github.com/feather-store/feather/internal/integrations/kubeflow"
	"github.com/feather-store/feather/internal/integrations/mlflow"
)

func setupMLIntegrationsHandler(t *testing.T) (*MLIntegrationsHandler, *http.ServeMux) {
	t.Helper()
	ml := mlflow.NewTracker(mlflow.DefaultConfig())
	kf := kubeflow.NewManager(kubeflow.DefaultConfig())
	af := airflow.NewProvider(airflow.DefaultConfig())
	handler := NewMLIntegrationsHandler(ml, kf, af)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return handler, mux
}

func TestMLIntegrationsHandler_MLflowListRuns(t *testing.T) {
	_, mux := setupMLIntegrationsHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/integrations/mlflow/runs", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestMLIntegrationsHandler_MLflowStartRun(t *testing.T) {
	_, mux := setupMLIntegrationsHandler(t)
	body := `{"name":"test-run","experiment_id":"exp1"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/integrations/mlflow/runs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("expected %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestMLIntegrationsHandler_MLflowStartRun_InvalidJSON(t *testing.T) {
	_, mux := setupMLIntegrationsHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/integrations/mlflow/runs", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}
