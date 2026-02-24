package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/arrowflight"
	"github.com/feather-store/feather/internal/extensions/computegraph"
	"github.com/feather-store/feather/internal/extensions/embeddingmgmt"
	"github.com/feather-store/feather/internal/extensions/feastcompat"
	"github.com/feather-store/feather/internal/extensions/gitopsdefs"
	"github.com/feather-store/feather/internal/extensions/prefetch"
	"github.com/feather-store/feather/internal/extensions/sdkcodegen"
	"github.com/feather-store/feather/internal/extensions/streamcompute"
	"github.com/feather-store/feather/internal/platform/transform"
)

// ---------------------------------------------------------------------------
// PythonRuntimeHandler
// ---------------------------------------------------------------------------

func setupPythonRuntimeHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	handler := NewPythonRuntimeHandler(transform.NewPythonExecutor())
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestPythonRuntime_ListWorkers(t *testing.T) {
	mux := setupPythonRuntimeHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/python/workers", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPythonRuntime_Stats(t *testing.T) {
	mux := setupPythonRuntimeHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/python/workers/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// ComputeGraphV2Handler
// ---------------------------------------------------------------------------

func setupComputeGraphV2Handler(t *testing.T) *http.ServeMux {
	t.Helper()
	engine := computegraph.NewEngine(computegraph.DefaultEngineConfig())
	memoizer := computegraph.NewMemoizer(computegraph.DefaultMemoizerConfig())
	handler := NewComputeGraphV2Handler(engine, memoizer)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestComputeGraphV2_MemoizerStats(t *testing.T) {
	mux := setupComputeGraphV2Handler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/graph/memoizer/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestComputeGraphV2_DeriveInvalidJSON(t *testing.T) {
	mux := setupComputeGraphV2Handler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/graph/derive", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestComputeGraphV2_MemoizerInvalidate(t *testing.T) {
	mux := setupComputeGraphV2Handler(t)
	body := `{"keys":["key1","key2"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/graph/memoizer/invalidate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// ConsistencyAdvancedHandler
// ---------------------------------------------------------------------------

func setupConsistencyAdvancedHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	handler := NewConsistencyAdvancedHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestConsistencyAdvanced_KS(t *testing.T) {
	mux := setupConsistencyAdvancedHandler(t)
	body := `{"online":[1.0,2.0,3.0],"offline":[1.1,2.1,3.1],"threshold":0.5}`
	req := httptest.NewRequest(http.MethodPost, "/v1/consistency/test/ks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestConsistencyAdvanced_PSI(t *testing.T) {
	mux := setupConsistencyAdvancedHandler(t)
	body := `{"actual":[1.0,2.0,3.0],"expected":[1.1,2.1,3.1],"num_bins":5,"threshold":0.5}`
	req := httptest.NewRequest(http.MethodPost, "/v1/consistency/test/psi", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestConsistencyAdvanced_ChiSquared(t *testing.T) {
	mux := setupConsistencyAdvancedHandler(t)
	body := `{"observed":{"a":10,"b":20},"expected":{"a":12,"b":18},"threshold":5.0}`
	req := httptest.NewRequest(http.MethodPost, "/v1/consistency/test/chi-squared", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestConsistencyAdvanced_JSD(t *testing.T) {
	mux := setupConsistencyAdvancedHandler(t)
	body := `{"p":[1.0,2.0,3.0],"q":[1.1,2.1,3.1],"num_bins":5,"threshold":0.5}`
	req := httptest.NewRequest(http.MethodPost, "/v1/consistency/test/jsd", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestConsistencyAdvanced_Snapshot(t *testing.T) {
	mux := setupConsistencyAdvancedHandler(t)
	body := `{"values":[1.0,2.0,3.0,4.0,5.0]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/consistency/snapshot", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestConsistencyAdvanced_InvalidJSON(t *testing.T) {
	mux := setupConsistencyAdvancedHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/consistency/test/ks", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// GitOpsManifestHandler
// ---------------------------------------------------------------------------

func setupGitOpsManifestHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	handler := NewGitOpsManifestHandler(gitopsdefs.NewManifestLoader())
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestGitOpsManifest_CICDGenerate(t *testing.T) {
	mux := setupGitOpsManifestHandler(t)
	body := `{"Provider":"github","Branch":"main","FeatureDir":"features/","FeatherImage":"feather:latest","AutoApply":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/gitops/cicd/generate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGitOpsManifest_CICDGenerateInvalidProvider(t *testing.T) {
	mux := setupGitOpsManifestHandler(t)
	body := `{"Provider":"unsupported"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/gitops/cicd/generate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGitOpsManifest_InvalidJSON(t *testing.T) {
	mux := setupGitOpsManifestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/gitops/cicd/generate", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// ArrowBatchHandler
// ---------------------------------------------------------------------------

func setupArrowBatchHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	server := arrowflight.NewServer(arrowflight.DefaultConfig())
	bs := arrowflight.NewBatchServer(server, arrowflight.DefaultBatchConfig())
	conv := arrowflight.NewBatchConverter()
	handler := NewArrowBatchHandler(bs, conv)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestArrowBatch_Stats(t *testing.T) {
	mux := setupArrowBatchHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/flight/batch/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestArrowBatch_InvalidJSON(t *testing.T) {
	mux := setupArrowBatchHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/flight/batch", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// StreamAdvancedHandler
// ---------------------------------------------------------------------------

func setupStreamAdvancedHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	proc := streamcompute.NewExactlyOnceProcessor(streamcompute.DefaultExactlyOnceConfig())
	handler := NewStreamAdvancedHandler(proc)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestStreamAdvanced_BeginTransaction(t *testing.T) {
	mux := setupStreamAdvancedHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/stream/transaction/begin", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestStreamAdvanced_ListPatterns(t *testing.T) {
	mux := setupStreamAdvancedHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/stream/patterns", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestStreamAdvanced_LateDataStats(t *testing.T) {
	mux := setupStreamAdvancedHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/stream/latedata/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestStreamAdvanced_CreatePatternInvalidJSON(t *testing.T) {
	mux := setupStreamAdvancedHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/stream/patterns", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// FeastEnhancedHandler
// ---------------------------------------------------------------------------

func setupFeastEnhancedHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	svcMgr := feastcompat.NewFeatureServiceManager(feastcompat.DefaultFeatureServiceConfig())
	adapter := feastcompat.NewAdapter(feastcompat.DefaultAdapterConfig())
	gw := feastcompat.NewGateway(adapter)
	suite := feastcompat.NewCompatTestSuite(gw)
	migration := feastcompat.NewMigrationCLI()
	handler := NewFeastEnhancedHandler(svcMgr, suite, migration)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestFeastEnhanced_ListServices(t *testing.T) {
	mux := setupFeastEnhancedHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/feast/services", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestFeastEnhanced_CreateService(t *testing.T) {
	mux := setupFeastEnhancedHandler(t)
	body := `{"name":"test_svc","views":["view1"],"description":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/feast/services", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestFeastEnhanced_GetServiceNotFound(t *testing.T) {
	mux := setupFeastEnhancedHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/feast/services/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestFeastEnhanced_ListTests(t *testing.T) {
	mux := setupFeastEnhancedHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/feast/compat/tests", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestFeastEnhanced_InvalidJSON(t *testing.T) {
	mux := setupFeastEnhancedHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/feast/services", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// EmbeddingLifecycleHandler
// ---------------------------------------------------------------------------

func setupEmbeddingLifecycleHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	mgr := embeddingmgmt.NewManager(embeddingmgmt.DefaultManagerConfig())
	bp := embeddingmgmt.NewBatchProcessor(mgr, embeddingmgmt.DefaultBatchConfig())
	ab := embeddingmgmt.NewABTester(mgr)
	drift := embeddingmgmt.NewVectorDriftDetector(embeddingmgmt.DefaultVectorDriftConfig())
	handler := NewEmbeddingLifecycleHandler(bp, ab, drift)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestEmbeddingLifecycle_ListBatch(t *testing.T) {
	mux := setupEmbeddingLifecycleHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/embeddings/batch", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestEmbeddingLifecycle_ListABTests(t *testing.T) {
	mux := setupEmbeddingLifecycleHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/embeddings/abtest", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestEmbeddingLifecycle_GetABTestNotFound(t *testing.T) {
	mux := setupEmbeddingLifecycleHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/embeddings/abtest/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestEmbeddingLifecycle_ListDrift(t *testing.T) {
	mux := setupEmbeddingLifecycleHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/embeddings/drift", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestEmbeddingLifecycle_InvalidJSON(t *testing.T) {
	mux := setupEmbeddingLifecycleHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings/batch", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// SDKLanguagesHandler
// ---------------------------------------------------------------------------

func setupSDKLanguagesHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	handler := NewSDKLanguagesHandler(sdkcodegen.NewLanguageRegistry())
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestSDKLanguages_List(t *testing.T) {
	mux := setupSDKLanguagesHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/sdk/languages", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSDKLanguages_GenerateInvalidJSON(t *testing.T) {
	mux := setupSDKLanguagesHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/sdk/generate/go", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// PredictiveWarmingHandler
// ---------------------------------------------------------------------------

func setupPredictiveWarmingHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	forecaster := prefetch.NewForecaster(prefetch.DefaultForecasterConfig())
	warmer := prefetch.NewWarmer(prefetch.DefaultWarmerConfig(), forecaster)
	handler := NewPredictiveWarmingHandler(forecaster, warmer)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestPredictiveWarming_RecordAccess(t *testing.T) {
	mux := setupPredictiveWarmingHandler(t)
	body := `{"entity":"user:123","feature":"click_count"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/warming/access", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPredictiveWarming_Clusters(t *testing.T) {
	mux := setupPredictiveWarmingHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/warming/clusters", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPredictiveWarming_Plan(t *testing.T) {
	mux := setupPredictiveWarmingHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/warming/plan", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPredictiveWarming_Stats(t *testing.T) {
	mux := setupPredictiveWarmingHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/warming/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPredictiveWarming_InvalidJSON(t *testing.T) {
	mux := setupPredictiveWarmingHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/warming/access", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
