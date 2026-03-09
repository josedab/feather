package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/feather-store/feather/internal/extensions/graphqlfederation"
)

func TestGraphQLFederationHandler_ServiceRegistration(t *testing.T) {
	gw := graphqlfederation.NewGateway(graphqlfederation.DefaultGatewayConfig())
	handler := NewGraphQLFederationHandler(gw)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	body, _ := json.Marshal(graphqlfederation.ServiceConfig{
		Name: "features",
		URL:  "http://features:8080/graphql",
	})
	req := httptest.NewRequest("POST", "/v1/graphql/federation/services", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest("GET", "/v1/graphql/federation/services", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
