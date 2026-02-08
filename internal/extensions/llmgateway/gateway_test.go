package llmgateway

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestGateway() *Gateway {
	cfg := DefaultGatewayConfig()
	cfg.MaxCacheEntries = 100
	return NewGateway(cfg)
}

// ---------- Cache Lookup ----------

func TestLookup_Miss(t *testing.T) {
	gw := newTestGateway()
	resp := gw.Lookup(LookupRequest{Prompt: "hello", Model: "gpt-4", Provider: ProviderOpenAI})
	assert.False(t, resp.Hit)
	assert.Equal(t, "miss", resp.Source)
}

func TestLookup_Hit(t *testing.T) {
	gw := newTestGateway()
	_, err := gw.Store(StoreRequest{
		Prompt:   "hello",
		Response: "world",
		Model:    "gpt-4",
		Provider: ProviderOpenAI,
		TokensIn: 10, TokensOut: 5,
	})
	require.NoError(t, err)

	resp := gw.Lookup(LookupRequest{Prompt: "hello", Model: "gpt-4", Provider: ProviderOpenAI})
	assert.True(t, resp.Hit)
	assert.Equal(t, "cache", resp.Source)
	assert.Equal(t, "world", resp.Entry.Response)
}

func TestLookup_DifferentModel_Miss(t *testing.T) {
	gw := newTestGateway()
	_, _ = gw.Store(StoreRequest{
		Prompt: "hello", Response: "world", Model: "gpt-4",
		Provider: ProviderOpenAI, TokensIn: 10, TokensOut: 5,
	})
	resp := gw.Lookup(LookupRequest{Prompt: "hello", Model: "gpt-3.5", Provider: ProviderOpenAI})
	assert.False(t, resp.Hit)
}

// ---------- Store ----------

func TestStore_Validation(t *testing.T) {
	gw := newTestGateway()

	tests := []struct {
		name    string
		req     StoreRequest
		wantErr bool
	}{
		{"valid", StoreRequest{Prompt: "p", Response: "r", Model: "m", Provider: ProviderOpenAI}, false},
		{"empty prompt", StoreRequest{Prompt: "", Response: "r"}, true},
		{"empty response", StoreRequest{Prompt: "p", Response: ""}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := gw.Store(tt.req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStore_CostCalculation(t *testing.T) {
	gw := newTestGateway()
	entry, err := gw.Store(StoreRequest{
		Prompt: "p", Response: "r", Model: "m",
		Provider: ProviderOpenAI, TokensIn: 500, TokensOut: 500,
	})
	require.NoError(t, err)
	// 1000 tokens at $0.03/1K = $0.03
	assert.InDelta(t, 0.03, entry.CostUSD, 0.001)
}

func TestStore_Eviction(t *testing.T) {
	cfg := DefaultGatewayConfig()
	cfg.MaxCacheEntries = 2
	gw := NewGateway(cfg)

	_, _ = gw.Store(StoreRequest{Prompt: "a", Response: "1", Model: "m", Provider: ProviderOpenAI})
	_, _ = gw.Store(StoreRequest{Prompt: "b", Response: "2", Model: "m", Provider: ProviderOpenAI})
	_, _ = gw.Store(StoreRequest{Prompt: "c", Response: "3", Model: "m", Provider: ProviderOpenAI})

	stats := gw.GetStats()
	assert.Equal(t, 2, stats["cache_size"])
}

// ---------- Templates ----------

func TestRegisterTemplate_Validation(t *testing.T) {
	gw := newTestGateway()

	tests := []struct {
		name    string
		tmpl    PromptTemplate
		wantErr bool
	}{
		{"valid", PromptTemplate{Name: "greet", Template: "Hello {{.name}}"}, false},
		{"no name", PromptTemplate{Template: "Hi"}, true},
		{"no body", PromptTemplate{Name: "empty"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := gw.RegisterTemplate(tt.tmpl)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRegisterTemplate_ExtractsVariables(t *testing.T) {
	gw := newTestGateway()
	err := gw.RegisterTemplate(PromptTemplate{
		Name:     "multi",
		Template: "Hello {{.first}} {{.last}}, you are {{.first}}",
	})
	require.NoError(t, err)

	tmpls := gw.ListTemplates()
	require.Len(t, tmpls, 1)
	assert.ElementsMatch(t, []string{"first", "last"}, tmpls[0].Variables)
}

func TestRenderTemplate(t *testing.T) {
	gw := newTestGateway()
	require.NoError(t, gw.RegisterTemplate(PromptTemplate{
		Name:      "greet",
		Template:  "Hello {{.name}}, welcome to {{.place}}!",
		Model:     "gpt-4",
		Provider:  ProviderOpenAI,
		MaxTokens: 100,
	}))

	resp, err := gw.RenderTemplate(RenderRequest{
		TemplateName: "greet",
		Variables:    map[string]string{"name": "Alice", "place": "Feather"},
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello Alice, welcome to Feather!", resp.RenderedPrompt)
	assert.Equal(t, "gpt-4", resp.Model)
	assert.Greater(t, resp.EstimatedCost, float64(0))
}

func TestRenderTemplate_NotFound(t *testing.T) {
	gw := newTestGateway()
	_, err := gw.RenderTemplate(RenderRequest{TemplateName: "nope"})
	assert.Error(t, err)
}

// ---------- A/B Tests ----------

func TestCreateABTest_Validation(t *testing.T) {
	gw := newTestGateway()

	tests := []struct {
		name    string
		test    ABTest
		wantErr bool
	}{
		{
			"valid",
			ABTest{
				ID:         "t1",
				VariantA:   ABVariant{TemplateName: "a"},
				VariantB:   ABVariant{TemplateName: "b"},
				TrafficPct: 50,
			},
			false,
		},
		{"no ID", ABTest{VariantA: ABVariant{TemplateName: "a"}, VariantB: ABVariant{TemplateName: "b"}}, true},
		{"missing variant", ABTest{ID: "t2", VariantA: ABVariant{TemplateName: "a"}}, true},
		{
			"invalid traffic",
			ABTest{
				ID: "t3", VariantA: ABVariant{TemplateName: "a"},
				VariantB: ABVariant{TemplateName: "b"}, TrafficPct: 150,
			},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := gw.CreateABTest(tt.test)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestResolveABTest_Deterministic(t *testing.T) {
	gw := newTestGateway()
	_, err := gw.CreateABTest(ABTest{
		ID:         "exp1",
		VariantA:   ABVariant{TemplateName: "a", Model: "gpt-4"},
		VariantB:   ABVariant{TemplateName: "b", Model: "gpt-3.5"},
		TrafficPct: 50,
	})
	require.NoError(t, err)

	// Same entity should always get the same variant.
	variant1, label1, err1 := gw.ResolveABTest("exp1", "user-42")
	require.NoError(t, err1)
	variant2, label2, err2 := gw.ResolveABTest("exp1", "user-42")
	require.NoError(t, err2)

	assert.Equal(t, label1, label2)
	assert.Equal(t, variant1.TemplateName, variant2.TemplateName)
}

func TestResolveABTest_NotFound(t *testing.T) {
	gw := newTestGateway()
	_, _, err := gw.ResolveABTest("nope", "entity")
	assert.Error(t, err)
}

func TestGetABTestResults(t *testing.T) {
	gw := newTestGateway()
	_, _ = gw.CreateABTest(ABTest{
		ID:         "exp2",
		VariantA:   ABVariant{TemplateName: "a"},
		VariantB:   ABVariant{TemplateName: "b"},
		TrafficPct: 50,
	})

	// Resolve a few entities to generate results.
	for i := 0; i < 20; i++ {
		_, _, _ = gw.ResolveABTest("exp2", fmt.Sprintf("entity-%d", i))
	}

	test, err := gw.GetABTestResults("exp2")
	require.NoError(t, err)
	assert.Equal(t, int64(20), test.Results.VariantACalls+test.Results.VariantBCalls)
}

func TestListABTests(t *testing.T) {
	gw := newTestGateway()
	_, _ = gw.CreateABTest(ABTest{
		ID: "a", VariantA: ABVariant{TemplateName: "x"}, VariantB: ABVariant{TemplateName: "y"}, TrafficPct: 50,
	})
	_, _ = gw.CreateABTest(ABTest{
		ID: "b", VariantA: ABVariant{TemplateName: "x"}, VariantB: ABVariant{TemplateName: "y"}, TrafficPct: 50,
	})
	assert.Len(t, gw.ListABTests(), 2)
}

// ---------- Rate Limiter ----------

func TestAllowRequest(t *testing.T) {
	cfg := DefaultGatewayConfig()
	cfg.RateLimitPerMinute = 3
	gw := NewGateway(cfg)

	// First 3 calls should succeed.
	assert.True(t, gw.AllowRequest("client-1"))
	assert.True(t, gw.AllowRequest("client-1"))
	assert.True(t, gw.AllowRequest("client-1"))

	// 4th call should be rate-limited.
	assert.False(t, gw.AllowRequest("client-1"))

	// Different client is independent.
	assert.True(t, gw.AllowRequest("client-2"))
}

// ---------- Cost Tracking ----------

func TestCostTracking(t *testing.T) {
	gw := newTestGateway()

	_, _ = gw.Store(StoreRequest{
		Prompt: "a", Response: "b", Model: "m",
		Provider: ProviderOpenAI, TokensIn: 1000, TokensOut: 0,
	})
	_, _ = gw.Store(StoreRequest{
		Prompt: "c", Response: "d", Model: "m",
		Provider: ProviderAnthropic, TokensIn: 1000, TokensOut: 0,
	})

	costs := gw.GetCosts()
	assert.Contains(t, costs, "openai")
	assert.Contains(t, costs, "anthropic")
	assert.InDelta(t, 0.03, costs["openai"].TotalCost, 0.001)
	assert.InDelta(t, 0.025, costs["anthropic"].TotalCost, 0.001)
}

func TestCostTracking_OllamaFree(t *testing.T) {
	gw := newTestGateway()
	entry, err := gw.Store(StoreRequest{
		Prompt: "a", Response: "b", Model: "llama2",
		Provider: ProviderOllama, TokensIn: 1000, TokensOut: 0,
	})
	require.NoError(t, err)
	assert.Equal(t, float64(0), entry.CostUSD)
}

// ---------- Stats ----------

func TestGetStats(t *testing.T) {
	gw := newTestGateway()

	_, _ = gw.Store(StoreRequest{
		Prompt: "p", Response: "r", Model: "m", Provider: ProviderOpenAI,
	})
	gw.Lookup(LookupRequest{Prompt: "p", Model: "m", Provider: ProviderOpenAI})
	gw.Lookup(LookupRequest{Prompt: "miss", Model: "m", Provider: ProviderOpenAI})

	stats := gw.GetStats()
	assert.Equal(t, int64(1), stats["cache_hits"])
	assert.Equal(t, int64(1), stats["cache_misses"])
	assert.Equal(t, int64(2), stats["total_calls"])
	assert.Equal(t, 1, stats["cache_size"])
	assert.Equal(t, 0, stats["templates"])
	assert.Equal(t, 0, stats["ab_tests"])
	assert.InDelta(t, 0.5, stats["hit_rate"], 0.01)
}

// ---------- Helpers ----------

func TestExtractVariables(t *testing.T) {
	tests := []struct {
		tmpl string
		want []string
	}{
		{"no vars", nil},
		{"{{.a}}", []string{"a"}},
		{"{{.x}} and {{.y}}", []string{"x", "y"}},
		{"{{.dup}} {{.dup}}", []string{"dup"}},
	}
	for _, tt := range tests {
		t.Run(tt.tmpl, func(t *testing.T) {
			assert.Equal(t, tt.want, extractVariables(tt.tmpl))
		})
	}
}

func TestHashToBucket(t *testing.T) {
	b := hashToBucket("test-entity")
	assert.GreaterOrEqual(t, b, 0)
	assert.Less(t, b, 100)
	// Deterministic
	assert.Equal(t, b, hashToBucket("test-entity"))
}
