package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/llmstore"
)

func newTestLLMStoreHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	store := llmstore.NewStore(llmstore.DefaultStoreConfig())
	handler := NewLLMStoreHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestLLMStoreHandler_CreatePrompt(t *testing.T) {
	mux := newTestLLMStoreHandler(t)

	body := `{"id":"summarize","name":"Summarize","template":"Summarize: {{text}}","model":"gpt-4"}`
	req := httptest.NewRequest("POST", "/v1/llm/prompts", strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("POST prompt = %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}
}

func TestLLMStoreHandler_ListPrompts(t *testing.T) {
	mux := newTestLLMStoreHandler(t)

	req := httptest.NewRequest("GET", "/v1/llm/prompts", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET prompts = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestLLMStoreHandler_StoreEmbedding(t *testing.T) {
	mux := newTestLLMStoreHandler(t)

	body := `{"id":"e1","vector":[0.1,0.2,0.3],"text":"hello world"}`
	req := httptest.NewRequest("POST", "/v1/llm/embeddings", strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("POST embedding = %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}
}

func TestLLMStoreHandler_SearchSimilar(t *testing.T) {
	mux := newTestLLMStoreHandler(t)

	// Store some embeddings first
	for _, body := range []string{
		`{"id":"e1","vector":[1,0,0],"text":"apple"}`,
		`{"id":"e2","vector":[0,1,0],"text":"car"}`,
	} {
		req := httptest.NewRequest("POST", "/v1/llm/embeddings", strings.NewReader(body))
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
	}

	body := `{"vector":[1,0,0],"top_k":2,"min_score":0.5}`
	req := httptest.NewRequest("POST", "/v1/llm/embeddings/search", strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("POST search = %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestLLMStoreHandler_CreatePipeline(t *testing.T) {
	mux := newTestLLMStoreHandler(t)

	body := `{"id":"qa","name":"QA Pipeline","prompt_template_id":"qa_prompt","embedding_model":"ada-002","top_k":5}`
	req := httptest.NewRequest("POST", "/v1/llm/rag/pipelines", strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("POST pipeline = %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}
}

func TestLLMStoreHandler_PromptNotFound(t *testing.T) {
	mux := newTestLLMStoreHandler(t)

	req := httptest.NewRequest("GET", "/v1/llm/prompts/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("GET nonexistent = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestLLMStoreHandler_Stats(t *testing.T) {
	mux := newTestLLMStoreHandler(t)

	req := httptest.NewRequest("GET", "/v1/llm/store/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET stats = %d, want %d", rr.Code, http.StatusOK)
	}
}
