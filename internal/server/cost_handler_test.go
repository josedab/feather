package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/feather-store/feather/internal/cost"
)

func setupCostHandler() *CostHandler {
	tracker := cost.NewTracker("USD")
	budgetManager := cost.NewBudgetManager(tracker)
	chargebackManager := cost.NewChargebackManager(tracker)
	return NewCostHandler(tracker, budgetManager, chargebackManager)
}

func TestCostHandler_RecordUsage(t *testing.T) {
	handler := setupCostHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	body := `{"tenantId":"tenant-1","category":"api","unit":"requests","quantity":1000}`
	req := httptest.NewRequest("POST", "/v1/cost/usage", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var entry cost.CostEntry
	if err := json.NewDecoder(w.Body).Decode(&entry); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if entry.ID == "" {
		t.Error("Expected entry to have ID")
	}
	if entry.Cost <= 0 {
		t.Error("Expected positive cost")
	}
}

func TestCostHandler_RecordUsage_Validation(t *testing.T) {
	handler := setupCostHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	tests := []struct {
		name string
		body string
	}{
		{"missing category", `{"unit":"requests","quantity":1000}`},
		{"missing unit", `{"category":"api","quantity":1000}`},
		{"zero quantity", `{"category":"api","unit":"requests","quantity":0}`},
		{"negative quantity", `{"category":"api","unit":"requests","quantity":-10}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/v1/cost/usage", bytes.NewBufferString(tt.body))
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected status 400, got %d", w.Code)
			}
		})
	}
}

func TestCostHandler_GetUsage(t *testing.T) {
	handler := setupCostHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Record some usage first
	body := `{"tenantId":"tenant-1","category":"api","unit":"requests","quantity":1000}`
	req := httptest.NewRequest("POST", "/v1/cost/usage", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Get usage
	req = httptest.NewRequest("GET", "/v1/cost/usage?tenant_id=tenant-1", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	if response["count"].(float64) < 1 {
		t.Error("Expected at least 1 record")
	}
}

func TestCostHandler_GetCostSummary(t *testing.T) {
	handler := setupCostHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Record usage
	body := `{"tenantId":"tenant-1","category":"api","unit":"requests","quantity":1000}`
	req := httptest.NewRequest("POST", "/v1/cost/usage", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Get summary
	req = httptest.NewRequest("GET", "/v1/cost/summary?tenant_id=tenant-1", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestCostHandler_ListRates(t *testing.T) {
	handler := setupCostHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/cost/rates", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	// Should have default rates
	if response["count"].(float64) == 0 {
		t.Error("Expected default rates")
	}
}

func TestCostHandler_SetRate(t *testing.T) {
	handler := setupCostHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	body := `{"category":"custom","unit":"items","pricePerUnit":0.01,"description":"Custom rate"}`
	req := httptest.NewRequest("PUT", "/v1/cost/rates", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCostHandler_Budget_CRUD(t *testing.T) {
	handler := setupCostHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Create budget
	body := `{"tenantId":"tenant-1","name":"Test Budget","amount":1000,"period":"monthly"}`
	req := httptest.NewRequest("POST", "/v1/cost/budgets", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var budget cost.Budget
	json.NewDecoder(w.Body).Decode(&budget)
	budgetID := budget.ID

	// Get budget
	req = httptest.NewRequest("GET", "/v1/cost/budgets/"+budgetID, nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Update budget
	body = `{"amount":2000}`
	req = httptest.NewRequest("PUT", "/v1/cost/budgets/"+budgetID, bytes.NewBufferString(body))
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// List budgets
	req = httptest.NewRequest("GET", "/v1/cost/budgets?tenant_id=tenant-1", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Delete budget
	req = httptest.NewRequest("DELETE", "/v1/cost/budgets/"+budgetID, nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", w.Code)
	}

	// Verify deleted
	req = httptest.NewRequest("GET", "/v1/cost/budgets/"+budgetID, nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestCostHandler_GetBudgetStatus(t *testing.T) {
	handler := setupCostHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Create budget
	body := `{"tenantId":"tenant-1","name":"Test Budget","amount":1000,"period":"monthly"}`
	req := httptest.NewRequest("POST", "/v1/cost/budgets", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var budget cost.Budget
	json.NewDecoder(w.Body).Decode(&budget)

	// Get status
	req = httptest.NewRequest("GET", "/v1/cost/budgets/"+budget.ID+"/status", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var status cost.BudgetStatus
	json.NewDecoder(w.Body).Decode(&status)

	if status.BudgetAmount != 1000 {
		t.Errorf("Expected budget amount 1000, got %f", status.BudgetAmount)
	}
}

func TestCostHandler_Alerts(t *testing.T) {
	handler := setupCostHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Create budget with low amount to trigger alerts
	body := `{"tenantId":"tenant-1","name":"Test Budget","amount":0.0001,"period":"monthly"}`
	req := httptest.NewRequest("POST", "/v1/cost/budgets", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Record usage to exceed budget
	body = `{"tenantId":"tenant-1","category":"api","unit":"requests","quantity":10000}`
	req = httptest.NewRequest("POST", "/v1/cost/usage", bytes.NewBufferString(body))
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Get alerts
	req = httptest.NewRequest("GET", "/v1/cost/alerts?tenant_id=tenant-1", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestCostHandler_AllocationRule_CRUD(t *testing.T) {
	handler := setupCostHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Create rule
	body := `{"tenantId":"tenant-1","name":"ML Allocation","costCenter":"ml-team","percentage":100,"priority":1}`
	req := httptest.NewRequest("POST", "/v1/cost/rules", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var rule cost.CostAllocationRule
	json.NewDecoder(w.Body).Decode(&rule)
	ruleID := rule.ID

	// Get rule
	req = httptest.NewRequest("GET", "/v1/cost/rules/"+ruleID, nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Update rule
	body = `{"percentage":50}`
	req = httptest.NewRequest("PUT", "/v1/cost/rules/"+ruleID, bytes.NewBufferString(body))
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// List rules
	req = httptest.NewRequest("GET", "/v1/cost/rules?tenant_id=tenant-1", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Delete rule
	req = httptest.NewRequest("DELETE", "/v1/cost/rules/"+ruleID, nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", w.Code)
	}
}

func TestCostHandler_Invoice_CRUD(t *testing.T) {
	handler := setupCostHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Record usage first
	body := `{"tenantId":"tenant-1","category":"api","unit":"requests","quantity":10000}`
	req := httptest.NewRequest("POST", "/v1/cost/usage", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Generate invoice
	now := time.Now()
	invoiceBody := map[string]interface{}{
		"tenantId": "tenant-1",
		"start":    now.Add(-time.Hour).Format(time.RFC3339),
		"end":      now.Add(time.Hour).Format(time.RFC3339),
	}
	jsonBody, _ := json.Marshal(invoiceBody)
	req = httptest.NewRequest("POST", "/v1/cost/invoices", bytes.NewBuffer(jsonBody))
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var invoice cost.Invoice
	json.NewDecoder(w.Body).Decode(&invoice)
	invoiceID := invoice.ID

	// Get invoice
	req = httptest.NewRequest("GET", "/v1/cost/invoices/"+invoiceID, nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Update invoice status
	body = `{"status":"pending"}`
	req = httptest.NewRequest("PUT", "/v1/cost/invoices/"+invoiceID+"/status", bytes.NewBufferString(body))
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Apply credit
	body = `{"description":"Promotional credit","amount":0.5}`
	req = httptest.NewRequest("POST", "/v1/cost/invoices/"+invoiceID+"/credit", bytes.NewBufferString(body))
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// List invoices
	req = httptest.NewRequest("GET", "/v1/cost/invoices?tenant_id=tenant-1", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestCostHandler_GenerateReport(t *testing.T) {
	handler := setupCostHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Record usage
	body := `{"tenantId":"tenant-1","featureGroup":"recs","category":"api","unit":"requests","quantity":10000}`
	req := httptest.NewRequest("POST", "/v1/cost/usage", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Generate report
	now := time.Now()
	reportBody := map[string]interface{}{
		"tenantId":      "tenant-1",
		"start":         now.Add(-time.Hour).Format(time.RFC3339),
		"end":           now.Add(time.Hour).Format(time.RFC3339),
		"granularity":   "daily",
		"includeTrends": true,
	}
	jsonBody, _ := json.Marshal(reportBody)
	req = httptest.NewRequest("POST", "/v1/cost/reports", bytes.NewBuffer(jsonBody))
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var report cost.CostReport
	json.NewDecoder(w.Body).Decode(&report)

	if report.ID == "" {
		t.Error("Expected report to have ID")
	}
	if report.TotalCost <= 0 {
		t.Error("Expected positive total cost")
	}
}

func TestCostHandler_GetChargebacks(t *testing.T) {
	handler := setupCostHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Create allocation rule
	body := `{"tenantId":"tenant-1","name":"All to Engineering","costCenter":"engineering","percentage":100,"priority":1}`
	req := httptest.NewRequest("POST", "/v1/cost/rules", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Record usage
	body = `{"tenantId":"tenant-1","featureGroup":"api","category":"api","unit":"requests","quantity":10000}`
	req = httptest.NewRequest("POST", "/v1/cost/usage", bytes.NewBufferString(body))
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Get chargebacks
	now := time.Now()
	req = httptest.NewRequest("GET", "/v1/cost/chargebacks?tenant_id=tenant-1&start="+now.Add(-time.Hour).Format(time.RFC3339)+"&end="+now.Add(time.Hour).Format(time.RFC3339), nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	if len(response["chargebacks"].(map[string]interface{})) == 0 {
		t.Error("Expected at least one chargeback")
	}
}

func TestCostHandler_InvalidJSON(t *testing.T) {
	handler := setupCostHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	endpoints := []struct {
		method string
		path   string
	}{
		{"POST", "/v1/cost/usage"},
		{"PUT", "/v1/cost/rates"},
		{"POST", "/v1/cost/budgets"},
		{"POST", "/v1/cost/rules"},
		{"POST", "/v1/cost/invoices"},
		{"POST", "/v1/cost/reports"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, bytes.NewBufferString("{invalid json"))
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected status 400, got %d", w.Code)
			}
		})
	}
}

func TestCostHandler_NotFound(t *testing.T) {
	handler := setupCostHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/v1/cost/budgets/nonexistent"},
		{"PUT", "/v1/cost/budgets/nonexistent"},
		{"DELETE", "/v1/cost/budgets/nonexistent"},
		{"GET", "/v1/cost/budgets/nonexistent/status"},
		{"GET", "/v1/cost/rules/nonexistent"},
		{"PUT", "/v1/cost/rules/nonexistent"},
		{"DELETE", "/v1/cost/rules/nonexistent"},
		{"GET", "/v1/cost/invoices/nonexistent"},
		{"PUT", "/v1/cost/invoices/nonexistent/status"},
		{"POST", "/v1/cost/invoices/nonexistent/credit"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			var body *bytes.Buffer
			if ep.method == "PUT" || ep.method == "POST" {
				body = bytes.NewBufferString(`{"amount":1}`)
			} else {
				body = bytes.NewBuffer(nil)
			}
			req := httptest.NewRequest(ep.method, ep.path, body)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusNotFound {
				t.Errorf("Expected status 404, got %d for %s %s", w.Code, ep.method, ep.path)
			}
		})
	}
}
