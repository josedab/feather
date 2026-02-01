package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDashboardOverview(t *testing.T) {
	d := New(Config{})

	req := httptest.NewRequest("GET", "/api/dashboard/overview", nil)
	w := httptest.NewRecorder()

	d.handleOverview(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp OverviewResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.HealthStatus == "" {
		t.Error("expected health status")
	}
}

func TestDashboardFeatureList(t *testing.T) {
	d := New(Config{})

	req := httptest.NewRequest("GET", "/api/dashboard/features", nil)
	w := httptest.NewRecorder()

	d.handleFeatureList(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if _, ok := resp["features"]; !ok {
		t.Error("expected features in response")
	}
}

func TestDashboardHealth(t *testing.T) {
	d := New(Config{})

	req := httptest.NewRequest("GET", "/api/dashboard/health", nil)
	w := httptest.NewRecorder()

	d.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp["status"] != "healthy" {
		t.Error("expected healthy status")
	}
}

func TestAlertManager(t *testing.T) {
	am := NewAlertManager("")

	// Create alert
	alert := &Alert{
		Title:    "Test Alert",
		Message:  "This is a test",
		Severity: SeverityWarning,
		Source:   "test",
	}

	if err := am.Create(alert); err != nil {
		t.Fatalf("create alert: %v", err)
	}

	// Get alert
	retrieved, err := am.Get(alert.ID)
	if err != nil {
		t.Fatalf("get alert: %v", err)
	}
	if retrieved.Title != "Test Alert" {
		t.Error("expected correct title")
	}

	// Acknowledge alert
	if err := am.Acknowledge(alert.ID); err != nil {
		t.Fatalf("acknowledge alert: %v", err)
	}
	retrieved, _ = am.Get(alert.ID)
	if retrieved.AcknowledgedAt == nil {
		t.Error("expected acknowledged time")
	}

	// Get active alerts (should be 0)
	active := am.GetActive()
	if len(active) != 0 {
		t.Errorf("expected 0 active alerts, got %d", len(active))
	}

	// Resolve alert
	if err := am.Resolve(alert.ID); err != nil {
		t.Fatalf("resolve alert: %v", err)
	}
}

func TestAlertForDrift(t *testing.T) {
	alert := AlertForDrift("click_count", 0.15, 0.1)

	if alert.Severity != SeverityWarning {
		t.Errorf("expected warning severity")
	}
	if alert.FeatureName != "click_count" {
		t.Errorf("expected feature name")
	}
	if alert.Source != "drift_detector" {
		t.Errorf("expected drift_detector source")
	}
}

func TestAlertForStaleness(t *testing.T) {
	lastUpdated := time.Now().Add(-2 * time.Hour)
	expectedTTL := 30 * time.Minute

	alert := AlertForStaleness("stale_feature", lastUpdated, expectedTTL)

	if alert.Severity != SeverityCritical {
		t.Errorf("expected critical severity for very stale feature")
	}
	if alert.FeatureName != "stale_feature" {
		t.Errorf("expected feature name")
	}
}

func TestAnalyticsCollector(t *testing.T) {
	ac := NewAnalyticsCollector()

	// Record some accesses
	for i := 0; i < 100; i++ {
		ac.RecordFeatureAccess("feature1", 100)
		ac.RecordFeatureAccess("feature2", 200)
	}
	for i := 0; i < 50; i++ {
		ac.RecordFeatureAccess("feature3", 150)
	}

	ac.RecordEntityAccess("user")
	ac.RecordEntityAccess("product")

	ac.RecordCacheHit()
	ac.RecordCacheHit()
	ac.RecordCacheMiss()

	// Get analytics
	analytics := ac.GetAnalytics()

	if analytics.TotalRequests != 250 {
		t.Errorf("expected 250 requests, got %d", analytics.TotalRequests)
	}

	if analytics.UniqueFeatures != 3 {
		t.Errorf("expected 3 unique features, got %d", analytics.UniqueFeatures)
	}

	if analytics.UniqueEntityTypes != 2 {
		t.Errorf("expected 2 entity types, got %d", analytics.UniqueEntityTypes)
	}

	// Check cache hit rate (2 hits / 3 total = 0.667)
	if analytics.CacheHitRate < 0.66 || analytics.CacheHitRate > 0.67 {
		t.Errorf("expected ~0.67 cache hit rate, got %f", analytics.CacheHitRate)
	}

	// Check top features
	if len(analytics.TopFeatures) != 3 {
		t.Errorf("expected 3 top features, got %d", len(analytics.TopFeatures))
	}
	if analytics.TopFeatures[0].Name != "feature1" && analytics.TopFeatures[0].Name != "feature2" {
		t.Errorf("expected feature1 or feature2 at top")
	}
}

func TestDashboardRoutes(t *testing.T) {
	d := New(Config{})
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	tests := []struct {
		method string
		path   string
		status int
	}{
		{"GET", "/api/dashboard/overview", http.StatusOK},
		{"GET", "/api/dashboard/features", http.StatusOK},
		{"GET", "/api/dashboard/health", http.StatusOK},
		{"GET", "/api/dashboard/metrics", http.StatusOK},
		{"GET", "/api/dashboard/drift", http.StatusOK},
		{"GET", "/api/dashboard/alerts", http.StatusOK},
		{"GET", "/api/dashboard/analytics", http.StatusOK},
		{"GET", "/dashboard", http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			if w.Code != tc.status {
				t.Errorf("expected %d, got %d", tc.status, w.Code)
			}
		})
	}
}
