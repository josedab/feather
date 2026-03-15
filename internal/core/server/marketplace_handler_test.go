package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/feather-store/feather/internal/extensions/marketplace"
)

func newTestMarketplace(t *testing.T) (*MarketplaceHandler, *marketplace.Catalog) {
	t.Helper()
	catalog := marketplace.NewCatalog()
	handler := NewMarketplaceHandler(catalog)
	return handler, catalog
}

func publishTestFeature(t *testing.T, catalog *marketplace.Catalog, id, name, owner string) {
	t.Helper()
	feat := &marketplace.PublishedFeature{
		ID:    id,
		Name:  name,
		Owner: owner,
		Status: marketplace.FeatureStatusPublished,
	}
	if err := catalog.Publish(feat); err != nil {
		t.Fatalf("failed to publish test feature: %v", err)
	}
}

func TestMarketplaceHandler_ListFeatures_Empty(t *testing.T) {
	h, _ := newTestMarketplace(t)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/marketplace/features", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp["total"].(float64) != 0 {
		t.Errorf("total = %v, want 0", resp["total"])
	}
}

func TestMarketplaceHandler_ListFeatures_WithData(t *testing.T) {
	h, catalog := newTestMarketplace(t)
	publishTestFeature(t, catalog, "f1", "Feature 1", "team-a")
	publishTestFeature(t, catalog, "f2", "Feature 2", "team-b")

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/marketplace/features", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp["total"].(float64) != 2 {
		t.Errorf("total = %v, want 2", resp["total"])
	}
	features := resp["features"].([]interface{})
	if len(features) != 2 {
		t.Errorf("features count = %d, want 2", len(features))
	}
}

func TestMarketplaceHandler_ListFeatures_Pagination(t *testing.T) {
	h, catalog := newTestMarketplace(t)
	for i := 0; i < 5; i++ {
		publishTestFeature(t, catalog, "f"+string(rune('a'+i)), "Feature", "owner")
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/marketplace/features?limit=2&offset=1", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp["total"].(float64) != 5 {
		t.Errorf("total = %v, want 5", resp["total"])
	}
	if resp["limit"].(float64) != 2 {
		t.Errorf("limit = %v, want 2", resp["limit"])
	}
	if resp["offset"].(float64) != 1 {
		t.Errorf("offset = %v, want 1", resp["offset"])
	}
	features := resp["features"].([]interface{})
	if len(features) != 2 {
		t.Errorf("features page size = %d, want 2", len(features))
	}
}

func TestMarketplaceHandler_PublishFeature(t *testing.T) {
	h, _ := newTestMarketplace(t)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := `{"id":"feat-1","name":"Click Count","owner":"ml-team"}`
	req := httptest.NewRequest("POST", "/v1/marketplace/features", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if !resp["success"].(bool) {
		t.Error("expected success=true")
	}
}

func TestMarketplaceHandler_PublishFeature_InvalidBody(t *testing.T) {
	h, _ := newTestMarketplace(t)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/v1/marketplace/features", bytes.NewBufferString("not json"))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}

	// Verify error response includes code field
	var resp ErrorResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Code != "bad_request" {
		t.Errorf("error code = %q, want %q", resp.Code, "bad_request")
	}
	if resp.Error == "" {
		t.Error("expected non-empty error message")
	}
}

func TestMarketplaceHandler_PublishFeature_MissingFields(t *testing.T) {
	h, _ := newTestMarketplace(t)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := `{"id":"feat-1"}`
	req := httptest.NewRequest("POST", "/v1/marketplace/features", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rr.Code, rr.Body.String())
	}
}

func TestMarketplaceHandler_GetFeature(t *testing.T) {
	h, catalog := newTestMarketplace(t)
	publishTestFeature(t, catalog, "feat-1", "Click Count", "ml-team")

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/marketplace/features/feat-1", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	feat := resp["feature"].(map[string]interface{})
	if feat["id"] != "feat-1" {
		t.Errorf("feature id = %v, want feat-1", feat["id"])
	}
}

func TestMarketplaceHandler_GetFeature_NotFound(t *testing.T) {
	h, _ := newTestMarketplace(t)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/marketplace/features/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestMarketplaceHandler_DeprecateFeature(t *testing.T) {
	h, catalog := newTestMarketplace(t)
	publishTestFeature(t, catalog, "feat-1", "Click Count", "ml-team")

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/v1/marketplace/features/feat-1/deprecate", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}

func TestMarketplaceHandler_DeprecateFeature_NotFound(t *testing.T) {
	h, _ := newTestMarketplace(t)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/v1/marketplace/features/nope/deprecate", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestMarketplaceHandler_Subscribe(t *testing.T) {
	h, catalog := newTestMarketplace(t)
	publishTestFeature(t, catalog, "feat-1", "Click Count", "ml-team")

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := `{"subscriber_id":"user-1","team":"data-eng"}`
	req := httptest.NewRequest("POST", "/v1/marketplace/features/feat-1/subscribe", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rr.Code, rr.Body.String())
	}
}

func TestMarketplaceHandler_Subscribe_MissingSubscriberID(t *testing.T) {
	h, catalog := newTestMarketplace(t)
	publishTestFeature(t, catalog, "feat-1", "Click Count", "ml-team")

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := `{"team":"data-eng"}`
	req := httptest.NewRequest("POST", "/v1/marketplace/features/feat-1/subscribe", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestMarketplaceHandler_Unsubscribe(t *testing.T) {
	h, catalog := newTestMarketplace(t)
	publishTestFeature(t, catalog, "feat-1", "Click Count", "ml-team")
	catalog.Subscribe("feat-1", "user-1", "data-eng")

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := `{"subscriber_id":"user-1"}`
	req := httptest.NewRequest("DELETE", "/v1/marketplace/features/feat-1/subscribe", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rr.Code, rr.Body.String())
	}
}

func TestMarketplaceHandler_GetSubscribers(t *testing.T) {
	h, catalog := newTestMarketplace(t)
	publishTestFeature(t, catalog, "feat-1", "Click Count", "ml-team")
	catalog.Subscribe("feat-1", "user-1", "data-eng")
	catalog.Subscribe("feat-1", "user-2", "ml")

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/marketplace/features/feat-1/subscribers", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["total"].(float64) != 2 {
		t.Errorf("total = %v, want 2", resp["total"])
	}
}

func TestMarketplaceHandler_Search(t *testing.T) {
	h, catalog := newTestMarketplace(t)
	publishTestFeature(t, catalog, "f1", "Click Count", "ml-team")
	publishTestFeature(t, catalog, "f2", "Revenue", "data-team")

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/marketplace/search?owner=ml-team", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	results := resp["results"].([]interface{})
	if len(results) != 1 {
		t.Errorf("results = %d, want 1 (filtered by owner)", len(results))
	}
}

func TestMarketplaceHandler_Stats(t *testing.T) {
	h, catalog := newTestMarketplace(t)
	publishTestFeature(t, catalog, "f1", "Click Count", "ml-team")

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/marketplace/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["success"] != true {
		t.Error("expected success=true")
	}
}

func TestParsePagination(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantLimit  int
		wantOffset int
	}{
		{"defaults", "", 100, 0},
		{"custom limit", "?limit=50", 50, 0},
		{"custom offset", "?offset=10", 100, 10},
		{"both", "?limit=25&offset=5", 25, 5},
		{"exceeds max", "?limit=5000", 1000, 0},
		{"negative limit ignored", "?limit=-1", 100, 0},
		{"negative offset ignored", "?offset=-1", 100, 0},
		{"invalid limit ignored", "?limit=abc", 100, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test"+tt.query, nil)
			limit, offset := parsePagination(req, 100, 1000)
			if limit != tt.wantLimit {
				t.Errorf("limit = %d, want %d", limit, tt.wantLimit)
			}
			if offset != tt.wantOffset {
				t.Errorf("offset = %d, want %d", offset, tt.wantOffset)
			}
		})
	}
}

func TestSetPaginationHeaders(t *testing.T) {
	tests := []struct {
		name           string
		total          int
		limit          int
		offset         int
		wantTotalCount string
		wantLinkNext   bool
		wantLinkPrev   bool
	}{
		{"first page with more", 50, 10, 0, "50", true, false},
		{"middle page", 50, 10, 20, "50", true, true},
		{"last page", 50, 10, 40, "50", false, true},
		{"single page", 5, 10, 0, "5", false, false},
		{"empty", 0, 10, 0, "0", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/test", nil)

			setPaginationHeaders(rr, tt.total, tt.limit, tt.offset, req)

			if got := rr.Header().Get("X-Total-Count"); got != tt.wantTotalCount {
				t.Errorf("X-Total-Count = %q, want %q", got, tt.wantTotalCount)
			}

			link := rr.Header().Get("Link")
			hasNext := len(link) > 0 && contains(link, `rel="next"`)
			hasPrev := len(link) > 0 && contains(link, `rel="prev"`)

			if hasNext != tt.wantLinkNext {
				t.Errorf("Link next = %v, want %v (Link: %q)", hasNext, tt.wantLinkNext, link)
			}
			if hasPrev != tt.wantLinkPrev {
				t.Errorf("Link prev = %v, want %v (Link: %q)", hasPrev, tt.wantLinkPrev, link)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestHttpStatusToErrorCode(t *testing.T) {
	tests := []struct {
		status int
		want   string
	}{
		{http.StatusBadRequest, "bad_request"},
		{http.StatusUnauthorized, "unauthorized"},
		{http.StatusForbidden, "forbidden"},
		{http.StatusNotFound, "not_found"},
		{http.StatusConflict, "conflict"},
		{http.StatusTooManyRequests, "rate_limited"},
		{http.StatusServiceUnavailable, "service_unavailable"},
		{http.StatusGatewayTimeout, "timeout"},
		{http.StatusInternalServerError, "internal_error"},
		{http.StatusTeapot, "error"}, // unknown 4xx
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("status_%d", tt.status), func(t *testing.T) {
			got := httpStatusToErrorCode(tt.status)
			if got != tt.want {
				t.Errorf("httpStatusToErrorCode(%d) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestErrorResponse_NotFound_IncludesCode(t *testing.T) {
	h, _ := newTestMarketplace(t)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/marketplace/features/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}

	var resp ErrorResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Code != "not_found" {
		t.Errorf("error code = %q, want %q", resp.Code, "not_found")
	}
}
