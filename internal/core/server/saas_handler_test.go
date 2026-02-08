package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/feather-store/feather/internal/platform/saas"
)

func setupSaaSHandler() (*SaaSHandler, *http.ServeMux) {
	planRegistry := saas.NewPlanRegistry()
	billingManager := saas.NewBillingManager(planRegistry)
	provisioningManager := saas.NewProvisioningManager(planRegistry, billingManager)
	handler := NewSaaSHandler(planRegistry, billingManager, provisioningManager)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return handler, mux
}

func TestSaaSHandler_ListPlans(t *testing.T) {
	_, mux := setupSaaSHandler()

	req := httptest.NewRequest("GET", "/v1/saas/plans", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var plans []saas.Plan
	if err := json.NewDecoder(w.Body).Decode(&plans); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(plans) < 4 {
		t.Errorf("Expected at least 4 plans, got %d", len(plans))
	}
}

func TestSaaSHandler_GetPlan(t *testing.T) {
	_, mux := setupSaaSHandler()

	req := httptest.NewRequest("GET", "/v1/saas/plans/free", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var plan saas.Plan
	if err := json.NewDecoder(w.Body).Decode(&plan); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if plan.ID != "free" {
		t.Errorf("Expected plan ID 'free', got '%s'", plan.ID)
	}
}

func TestSaaSHandler_GetPlan_NotFound(t *testing.T) {
	_, mux := setupSaaSHandler()

	req := httptest.NewRequest("GET", "/v1/saas/plans/nonexistent", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestSaaSHandler_CreatePlan(t *testing.T) {
	_, mux := setupSaaSHandler()

	plan := saas.Plan{
		ID:     "custom",
		Name:   "Custom Plan",
		Tier:   saas.TierPro,
		Active: true,
	}
	body, _ := json.Marshal(plan)

	req := httptest.NewRequest("POST", "/v1/saas/plans", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSaaSHandler_ComparePlans(t *testing.T) {
	_, mux := setupSaaSHandler()

	req := httptest.NewRequest("GET", "/v1/saas/plans/compare?plan1=free&plan2=pro", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var comparison saas.PlanComparison
	if err := json.NewDecoder(w.Body).Decode(&comparison); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if comparison.Plan1.ID != "free" || comparison.Plan2.ID != "pro" {
		t.Error("Comparison should contain free and pro plans")
	}
}

func TestSaaSHandler_CreateSubscription(t *testing.T) {
	_, mux := setupSaaSHandler()

	reqBody := map[string]interface{}{
		"organization_id": "org_1",
		"plan_id":         "starter",
		"billing_period":  "monthly",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/v1/saas/subscriptions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var sub saas.Subscription
	if err := json.NewDecoder(w.Body).Decode(&sub); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if sub.OrganizationID != "org_1" {
		t.Errorf("Expected org_1, got %s", sub.OrganizationID)
	}
}

func TestSaaSHandler_ListSubscriptions(t *testing.T) {
	_, mux := setupSaaSHandler()

	// Create a subscription first
	reqBody := map[string]interface{}{
		"organization_id": "org_1",
		"plan_id":         "starter",
		"billing_period":  "monthly",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/v1/saas/subscriptions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Now list subscriptions
	req = httptest.NewRequest("GET", "/v1/saas/subscriptions?org_id=org_1", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var subs []*saas.Subscription
	if err := json.NewDecoder(w.Body).Decode(&subs); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(subs) < 1 {
		t.Error("Expected at least 1 subscription")
	}
}

func TestSaaSHandler_CreateInstance(t *testing.T) {
	_, mux := setupSaaSHandler()

	reqBody := saas.ProvisioningRequest{
		OrganizationID: "org_1",
		Name:           "test-instance",
		Region:         "us-east-1",
		Size:           saas.SizeMedium,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/v1/saas/instances", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var instance saas.Instance
	if err := json.NewDecoder(w.Body).Decode(&instance); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if instance.Name != "test-instance" {
		t.Errorf("Expected name 'test-instance', got '%s'", instance.Name)
	}
}

func TestSaaSHandler_CreateInstance_InvalidRegion(t *testing.T) {
	_, mux := setupSaaSHandler()

	reqBody := saas.ProvisioningRequest{
		OrganizationID: "org_1",
		Name:           "test-instance",
		Region:         "invalid-region",
		Size:           saas.SizeMedium,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/v1/saas/instances", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestSaaSHandler_GetRegions(t *testing.T) {
	_, mux := setupSaaSHandler()

	req := httptest.NewRequest("GET", "/v1/saas/regions", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var regions []saas.Region
	if err := json.NewDecoder(w.Body).Decode(&regions); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(regions) < 4 {
		t.Errorf("Expected at least 4 regions, got %d", len(regions))
	}
}

func TestSaaSHandler_GetSizes(t *testing.T) {
	_, mux := setupSaaSHandler()

	req := httptest.NewRequest("GET", "/v1/saas/sizes", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var sizes []map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&sizes); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(sizes) < 4 {
		t.Errorf("Expected at least 4 sizes, got %d", len(sizes))
	}
}

func TestSaaSHandler_RecordUsage(t *testing.T) {
	_, mux := setupSaaSHandler()

	reqBody := saas.UsageRecord{
		OrganizationID: "org_1",
		Metric:         saas.MetricRequests,
		Quantity:       1000,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/v1/saas/usage", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSaaSHandler_GetUsageSummary(t *testing.T) {
	_, mux := setupSaaSHandler()

	req := httptest.NewRequest("GET", "/v1/saas/usage?org_id=org_1", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestSaaSHandler_ListInvoices(t *testing.T) {
	_, mux := setupSaaSHandler()

	req := httptest.NewRequest("GET", "/v1/saas/invoices?org_id=org_1", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestSaaSHandler_GenerateInvoice(t *testing.T) {
	_, mux := setupSaaSHandler()

	// First create a subscription
	subReq := map[string]interface{}{
		"organization_id": "org_1",
		"plan_id":         "starter",
		"billing_period":  "monthly",
	}
	subBody, _ := json.Marshal(subReq)
	req := httptest.NewRequest("POST", "/v1/saas/subscriptions", bytes.NewReader(subBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var sub saas.Subscription
	json.NewDecoder(w.Body).Decode(&sub)

	// Now generate invoice
	invoiceReq := map[string]string{
		"subscription_id": sub.ID,
	}
	invoiceBody, _ := json.Marshal(invoiceReq)
	req = httptest.NewRequest("POST", "/v1/saas/invoices", bytes.NewReader(invoiceBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSaaSHandler_AddPaymentMethod(t *testing.T) {
	_, mux := setupSaaSHandler()

	method := saas.PaymentMethod{
		Type:  "card",
		Last4: "4242",
	}
	body, _ := json.Marshal(method)

	req := httptest.NewRequest("POST", "/v1/saas/payment-methods?org_id=org_1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSaaSHandler_ListPaymentMethods(t *testing.T) {
	_, mux := setupSaaSHandler()

	req := httptest.NewRequest("GET", "/v1/saas/payment-methods?org_id=org_1", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestSaaSHandler_ListSubscriptions_NoOrgID(t *testing.T) {
	_, mux := setupSaaSHandler()

	req := httptest.NewRequest("GET", "/v1/saas/subscriptions", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestSaaSHandler_ListInstances_NoOrgID(t *testing.T) {
	_, mux := setupSaaSHandler()

	req := httptest.NewRequest("GET", "/v1/saas/instances", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}
