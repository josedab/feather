//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func baseURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("FEATHER_E2E_URL")
	if url == "" {
		t.Skip("FEATHER_E2E_URL not set, skipping e2e test")
	}
	return strings.TrimRight(url, "/")
}

func httpGet(t *testing.T, path string) (int, []byte) {
	t.Helper()
	resp, err := http.Get(baseURL(t) + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body
}

func httpPost(t *testing.T, path string, contentType string, payload string) (int, []byte) {
	t.Helper()
	resp, err := http.Post(baseURL(t)+path, contentType, strings.NewReader(payload))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body
}

func TestHealthEndpoint(t *testing.T) {
	code, body := httpGet(t, "/health")
	if code != http.StatusOK {
		t.Fatalf("GET /health returned %d: %s", code, body)
	}
}

func TestStoreAndRetrieveFeature(t *testing.T) {
	payload := `{"entity":"user:100","features":{"login_count":{"value":42,"type":"INT64"}}}`
	code, body := httpPost(t, "/v1/features", "application/json", payload)
	if code != http.StatusOK && code != http.StatusCreated {
		t.Fatalf("POST /v1/features returned %d: %s", code, body)
	}

	code, body = httpGet(t, "/v1/features?entity=user:100&feature=login_count")
	if code != http.StatusOK {
		t.Fatalf("GET /v1/features returned %d: %s", code, body)
	}
}

func TestBatchFeatures(t *testing.T) {
	payload := `{"entities":["user:1","user:2"],"features":["login_count"]}`
	code, body := httpPost(t, "/v1/features/batch", "application/json", payload)
	if code != http.StatusOK {
		t.Fatalf("POST /v1/features/batch returned %d: %s", code, body)
	}
}

func TestSchemaGroups(t *testing.T) {
	code, body := httpGet(t, "/v1/schema/groups")
	if code != http.StatusOK {
		t.Fatalf("GET /v1/schema/groups returned %d: %s", code, body)
	}
	var result interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
}

func TestDriftStatus(t *testing.T) {
	code, body := httpGet(t, "/v1/drift/status")
	if code != http.StatusOK {
		t.Fatalf("GET /v1/drift/status returned %d: %s", code, body)
	}
}

func TestFeatherQLQuery(t *testing.T) {
	payload := `{"query":"SELECT login_count FROM user_features WHERE entity = 'user:1'"}`
	code, body := httpPost(t, "/v1/featherql/execute", "application/json", payload)
	// Accept 200 or 400 (bad query is still a valid response)
	if code != http.StatusOK && code != http.StatusBadRequest {
		t.Fatalf("POST /v1/featherql/execute returned %d: %s", code, body)
	}
}

func TestOpenAPISpec(t *testing.T) {
	code, body := httpGet(t, "/openapi.json")
	if code != http.StatusOK {
		// Fall back to /swagger.json
		code, body = httpGet(t, "/swagger.json")
	}
	if code != http.StatusOK {
		t.Skipf("OpenAPI spec not available (status %d)", code)
	}
	if len(body) == 0 {
		t.Fatal("empty OpenAPI spec")
	}
	_ = fmt.Sprintf("OpenAPI spec size: %d bytes", len(body))
}
