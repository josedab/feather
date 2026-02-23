package gitopsdefs

import (
	"strings"
	"testing"
)

func TestGenerateCICD_GitHub(t *testing.T) {
	cfg := CICDConfig{
		Provider:   "github",
		Branch:     "main",
		FeatureDir: "features/",
		AutoApply:  true,
	}

	output, err := GenerateCICD(cfg)
	if err != nil {
		t.Fatalf("GenerateCICD: %v", err)
	}

	checks := []string{
		"name: Feather Feature Store",
		"branches: [main]",
		"feather-plan:",
		"feather-apply:",
		"feather apply --dir features/",
		"FEATHER_URL",
		"actions/checkout@v4",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("GitHub Actions output missing %q", check)
		}
	}
}

func TestGenerateCICD_GitHub_PlanOnly(t *testing.T) {
	cfg := CICDConfig{
		Provider:   "github",
		Branch:     "develop",
		FeatureDir: "defs/",
		AutoApply:  false,
	}

	output, err := GenerateCICD(cfg)
	if err != nil {
		t.Fatalf("GenerateCICD: %v", err)
	}

	if !strings.Contains(output, "branches: [develop]") {
		t.Error("expected branch develop")
	}
	if !strings.Contains(output, "feather plan --dir defs/") {
		t.Error("expected plan step with custom dir")
	}
	if strings.Contains(output, "feather apply") {
		t.Error("expected plan-only mode, but found apply")
	}
}

func TestGenerateCICD_GitLab(t *testing.T) {
	cfg := CICDConfig{
		Provider:     "gitlab",
		Branch:       "main",
		FeatureDir:   "features/",
		FeatherImage: "feather-store/feather:v1.0",
		AutoApply:    true,
	}

	output, err := GenerateCICD(cfg)
	if err != nil {
		t.Fatalf("GenerateCICD: %v", err)
	}

	checks := []string{
		"image: feather-store/feather:v1.0",
		"stages:",
		"feather-plan:",
		"feather-apply:",
		"feather apply --dir features/",
		"merge_request_event",
		"CI_COMMIT_BRANCH",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("GitLab CI output missing %q", check)
		}
	}
}

func TestGenerateCICD_GitLab_PlanOnly(t *testing.T) {
	cfg := CICDConfig{
		Provider:   "gitlab",
		Branch:     "main",
		FeatureDir: "features/",
		AutoApply:  false,
	}

	output, err := GenerateCICD(cfg)
	if err != nil {
		t.Fatalf("GenerateCICD: %v", err)
	}

	if strings.Contains(output, "feather apply") {
		t.Error("expected plan-only mode, but found apply")
	}
}

func TestGenerateCICD_Generic(t *testing.T) {
	cfg := CICDConfig{
		Provider:   "generic",
		FeatureDir: "features/",
		AutoApply:  true,
	}

	output, err := GenerateCICD(cfg)
	if err != nil {
		t.Fatalf("GenerateCICD: %v", err)
	}

	checks := []string{
		"#!/usr/bin/env bash",
		"set -euo pipefail",
		"feather validate",
		"feather plan",
		"feather apply",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("Generic CI output missing %q", check)
		}
	}
}

func TestGenerateCICD_Unsupported(t *testing.T) {
	_, err := GenerateCICD(CICDConfig{Provider: "jenkins"})
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("expected 'unsupported' in error, got %q", err.Error())
	}
}

func TestGenerateCICD_Defaults(t *testing.T) {
	cfg := CICDConfig{Provider: "generic"}

	output, err := GenerateCICD(cfg)
	if err != nil {
		t.Fatalf("GenerateCICD: %v", err)
	}

	if !strings.Contains(output, "feather-store/feather:latest") {
		t.Error("expected default image")
	}
	if !strings.Contains(output, "features/") {
		t.Error("expected default feature dir")
	}
}

func TestGenerateGitHubActions_Direct(t *testing.T) {
	output := GenerateGitHubActions(CICDConfig{
		Branch:     "main",
		FeatureDir: "features/",
		AutoApply:  false,
	})

	if !strings.Contains(output, "pull_request") {
		t.Error("expected pull_request trigger")
	}
}

func TestGenerateGitLabCI_Direct(t *testing.T) {
	output := GenerateGitLabCI(CICDConfig{
		Branch:       "main",
		FeatureDir:   "features/",
		FeatherImage: "feather:latest",
	})

	if !strings.Contains(output, "validate") {
		t.Error("expected validate stage")
	}
	if !strings.Contains(output, "deploy") {
		t.Error("expected deploy stage")
	}
}
