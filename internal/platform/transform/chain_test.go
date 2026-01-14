package transform

import (
	"context"
	"testing"
)

func TestChainEngine_CreateChain(t *testing.T) {
	catalog := NewCatalog()
	engine := NewChainEngine(catalog)

	chain := &Chain{
		ID:   "test-chain-1",
		Name: "Test Chain",
		Steps: []*ChainStep{
			{Name: "step1", Order: 0, Config: map[string]interface{}{"key": "value"}},
		},
	}

	if err := engine.CreateChain(chain); err != nil {
		t.Fatalf("CreateChain failed: %v", err)
	}

	got, err := engine.GetChain("test-chain-1")
	if err != nil {
		t.Fatalf("GetChain failed: %v", err)
	}
	if got.Name != "Test Chain" {
		t.Errorf("Name = %q, want %q", got.Name, "Test Chain")
	}
	if got.Status != ChainStatusDraft {
		t.Errorf("Status = %q, want %q", got.Status, ChainStatusDraft)
	}
	if got.Version != 1 {
		t.Errorf("Version = %d, want 1", got.Version)
	}
}

func TestChainEngine_CreateChain_Empty(t *testing.T) {
	catalog := NewCatalog()
	engine := NewChainEngine(catalog)

	chain := &Chain{
		ID:    "empty-chain",
		Name:  "Empty Chain",
		Steps: []*ChainStep{},
	}

	err := engine.CreateChain(chain)
	if err != ErrEmptyChain {
		t.Fatalf("expected ErrEmptyChain, got %v", err)
	}
}

func TestChainEngine_Execute(t *testing.T) {
	catalog := NewCatalog()
	engine := NewChainEngine(catalog)

	chain := &Chain{
		ID:   "exec-chain",
		Name: "Execution Chain",
		Steps: []*ChainStep{
			{Name: "step1", Order: 0, Config: map[string]interface{}{"added_by_step1": "v1"}},
			{Name: "step2", Order: 1, Config: map[string]interface{}{"added_by_step2": "v2"}},
		},
		Status: ChainStatusActive,
	}

	if err := engine.CreateChain(chain); err != nil {
		t.Fatalf("CreateChain failed: %v", err)
	}

	// Chain was created with Active status, but CreateChain defaults to draft if empty.
	// Since we set it explicitly, it should be active.
	if err := engine.ActivateChain("exec-chain"); err != nil {
		t.Fatalf("ActivateChain failed: %v", err)
	}

	input := map[string]interface{}{"initial": "data"}
	result, err := engine.Execute(context.Background(), "exec-chain", input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.ChainID != "exec-chain" {
		t.Errorf("ChainID = %q, want %q", result.ChainID, "exec-chain")
	}
	if result.StepCount != 2 {
		t.Errorf("StepCount = %d, want 2", result.StepCount)
	}
	if len(result.StepResults) != 2 {
		t.Fatalf("StepResults length = %d, want 2", len(result.StepResults))
	}

	// Final output should contain initial data plus config from both steps.
	if result.FinalOutput["initial"] != "data" {
		t.Errorf("FinalOutput missing 'initial' key")
	}
	if result.FinalOutput["added_by_step1"] != "v1" {
		t.Errorf("FinalOutput missing 'added_by_step1'")
	}
	if result.FinalOutput["added_by_step2"] != "v2" {
		t.Errorf("FinalOutput missing 'added_by_step2'")
	}

	for _, sr := range result.StepResults {
		if !sr.Success {
			t.Errorf("step %q failed: %s", sr.StepName, sr.Error)
		}
	}
}

func TestChainEngine_ExecuteNotActive(t *testing.T) {
	catalog := NewCatalog()
	engine := NewChainEngine(catalog)

	chain := &Chain{
		ID:   "draft-chain",
		Name: "Draft Chain",
		Steps: []*ChainStep{
			{Name: "step1", Order: 0},
		},
	}

	if err := engine.CreateChain(chain); err != nil {
		t.Fatalf("CreateChain failed: %v", err)
	}

	// Chain is draft by default; executing should fail.
	_, err := engine.Execute(context.Background(), "draft-chain", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error executing draft chain, got nil")
	}
}

func TestChainEngine_RemoveChain(t *testing.T) {
	catalog := NewCatalog()
	engine := NewChainEngine(catalog)

	chain := &Chain{
		ID:   "removable",
		Name: "To Remove",
		Steps: []*ChainStep{
			{Name: "step1", Order: 0},
		},
	}

	if err := engine.CreateChain(chain); err != nil {
		t.Fatalf("CreateChain failed: %v", err)
	}

	if err := engine.RemoveChain("removable"); err != nil {
		t.Fatalf("RemoveChain failed: %v", err)
	}

	_, err := engine.GetChain("removable")
	if err != ErrChainNotFound {
		t.Fatalf("expected ErrChainNotFound after removal, got %v", err)
	}

	err = engine.RemoveChain("nonexistent")
	if err != ErrChainNotFound {
		t.Fatalf("expected ErrChainNotFound for nonexistent, got %v", err)
	}
}

func TestChainEngine_PauseResume(t *testing.T) {
	catalog := NewCatalog()
	engine := NewChainEngine(catalog)

	chain := &Chain{
		ID:   "pause-chain",
		Name: "Pausable Chain",
		Steps: []*ChainStep{
			{Name: "step1", Order: 0},
		},
	}

	if err := engine.CreateChain(chain); err != nil {
		t.Fatalf("CreateChain failed: %v", err)
	}

	// Activate.
	if err := engine.ActivateChain("pause-chain"); err != nil {
		t.Fatalf("ActivateChain failed: %v", err)
	}
	got, _ := engine.GetChain("pause-chain")
	if got.Status != ChainStatusActive {
		t.Errorf("Status = %q, want %q", got.Status, ChainStatusActive)
	}

	// Pause.
	if err := engine.PauseChain("pause-chain"); err != nil {
		t.Fatalf("PauseChain failed: %v", err)
	}
	got, _ = engine.GetChain("pause-chain")
	if got.Status != ChainStatusPaused {
		t.Errorf("Status = %q, want %q", got.Status, ChainStatusPaused)
	}

	// Execute should fail while paused.
	_, err := engine.Execute(context.Background(), "pause-chain", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error executing paused chain, got nil")
	}

	// Re-activate.
	if err := engine.ActivateChain("pause-chain"); err != nil {
		t.Fatalf("ActivateChain failed: %v", err)
	}
	got, _ = engine.GetChain("pause-chain")
	if got.Status != ChainStatusActive {
		t.Errorf("Status = %q, want %q after re-activate", got.Status, ChainStatusActive)
	}
}
