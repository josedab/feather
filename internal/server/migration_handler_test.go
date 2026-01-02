package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/feather-store/feather/internal/migration"
)

func setupMigrationHandler() *MigrationHandler {
	manager := migration.NewManager(migration.DefaultManagerConfig())
	return NewMigrationHandler(manager)
}

func TestNewMigrationHandler(t *testing.T) {
	handler := setupMigrationHandler()
	if handler == nil {
		t.Fatal("Expected handler to be non-nil")
	}
}

func TestMigrationHandler_Analyze(t *testing.T) {
	handler := setupMigrationHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	project := map[string]interface{}{
		"project": map[string]interface{}{
			"project": "test_project",
			"entities": []map[string]interface{}{
				{"name": "user_id", "value_type": "STRING"},
			},
			"feature_views": []map[string]interface{}{
				{
					"name":     "user_features",
					"entities": []string{"user_id"},
					"features": []map[string]interface{}{
						{"name": "age", "value_type": "INT64"},
					},
				},
			},
		},
	}

	body, _ := json.Marshal(project)
	req := httptest.NewRequest("POST", "/v1/migration/analyze", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMigrationHandler_Analyze_NilProject(t *testing.T) {
	handler := setupMigrationHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	body, _ := json.Marshal(map[string]interface{}{})
	req := httptest.NewRequest("POST", "/v1/migration/analyze", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestMigrationHandler_ConvertSchema(t *testing.T) {
	handler := setupMigrationHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	project := map[string]interface{}{
		"project": map[string]interface{}{
			"project": "test_project",
			"feature_views": []map[string]interface{}{
				{
					"name": "features",
					"features": []map[string]interface{}{
						{"name": "value", "value_type": "DOUBLE"},
					},
				},
			},
		},
	}

	body, _ := json.Marshal(project)
	req := httptest.NewRequest("POST", "/v1/migration/convert/schema", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMigrationHandler_ConvertConfig(t *testing.T) {
	handler := setupMigrationHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	project := map[string]interface{}{
		"project": map[string]interface{}{
			"project": "test_project",
		},
	}

	body, _ := json.Marshal(project)
	req := httptest.NewRequest("POST", "/v1/migration/convert/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMigrationHandler_FullMigration(t *testing.T) {
	handler := setupMigrationHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	project := map[string]interface{}{
		"project": map[string]interface{}{
			"project": "test_project",
			"feature_views": []map[string]interface{}{
				{
					"name": "features",
					"features": []map[string]interface{}{
						{"name": "value", "value_type": "DOUBLE"},
					},
				},
			},
		},
	}

	body, _ := json.Marshal(project)
	req := httptest.NewRequest("POST", "/v1/migration/full", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMigrationHandler_PlanCRUD(t *testing.T) {
	handler := setupMigrationHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Create plan
	createBody, _ := json.Marshal(map[string]interface{}{
		"id":          "test-plan",
		"name":        "Test Plan",
		"source_type": "parquet",
	})
	req := httptest.NewRequest("POST", "/v1/migration/plans", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Create: Expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	// Get plan
	req = httptest.NewRequest("GET", "/v1/migration/plans/test-plan", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Get: Expected status 200, got %d", w.Code)
	}

	// List plans
	req = httptest.NewRequest("GET", "/v1/migration/plans", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("List: Expected status 200, got %d", w.Code)
	}

	// Delete plan
	req = httptest.NewRequest("DELETE", "/v1/migration/plans/test-plan", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Delete: Expected status 200, got %d", w.Code)
	}

	// Verify deletion
	req = httptest.NewRequest("GET", "/v1/migration/plans/test-plan", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("After delete: Expected status 404, got %d", w.Code)
	}
}

func TestMigrationHandler_CreatePlan_MissingID(t *testing.T) {
	handler := setupMigrationHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	body, _ := json.Marshal(map[string]interface{}{
		"name": "Test Plan",
	})
	req := httptest.NewRequest("POST", "/v1/migration/plans", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestMigrationHandler_CreatePlan_Duplicate(t *testing.T) {
	handler := setupMigrationHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	body, _ := json.Marshal(map[string]interface{}{
		"id":   "dup-plan",
		"name": "Test Plan",
	})

	// First creation
	req := httptest.NewRequest("POST", "/v1/migration/plans", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Second creation (duplicate)
	req = httptest.NewRequest("POST", "/v1/migration/plans", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("Expected status 409, got %d", w.Code)
	}
}

func TestMigrationHandler_GetPlan_NotFound(t *testing.T) {
	handler := setupMigrationHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/migration/plans/nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestMigrationHandler_DeletePlan_NotFound(t *testing.T) {
	handler := setupMigrationHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("DELETE", "/v1/migration/plans/nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestMigrationHandler_ListJobs(t *testing.T) {
	handler := setupMigrationHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/migration/jobs", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestMigrationHandler_GetJob_NotFound(t *testing.T) {
	handler := setupMigrationHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/migration/jobs/nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestMigrationHandler_Stats(t *testing.T) {
	handler := setupMigrationHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/migration/stats", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	if !resp["success"].(bool) {
		t.Error("Expected success response")
	}
}

func TestMigrationHandler_InvalidJSON(t *testing.T) {
	handler := setupMigrationHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/v1/migration/analyze", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}
