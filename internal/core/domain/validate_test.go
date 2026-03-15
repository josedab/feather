package domain

import (
	"strings"
	"testing"
)

func TestValidateEntityKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr string
	}{
		{"valid simple", "user:123", ""},
		{"valid with dots", "user.profile.v2", ""},
		{"valid unicode", "用户:123", ""},
		{"empty", "", "must not be empty"},
		{"whitespace only", "   ", "whitespace-only"},
		{"too long", strings.Repeat("x", 513), "exceeds maximum length"},
		{"control char null", "user\x00123", "control characters"},
		{"control char newline", "user\n123", "control characters"},
		{"path traversal", "../etc/passwd", "path traversal"},
		{"path traversal mid", "user/../admin", "path traversal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEntityKey(tt.key)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatalf("expected error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

func TestValidateFeatureName(t *testing.T) {
	tests := []struct {
		name    string
		feature string
		wantErr string
	}{
		{"valid simple", "click_count", ""},
		{"valid with colon", "feature:v2", ""},
		{"empty", "", "must not be empty"},
		{"whitespace only", " \t ", "whitespace-only"},
		{"too long", strings.Repeat("a", 257), "exceeds maximum length"},
		{"control char", "feat\x01name", "control characters"},
		{"path traversal", "../secret", "path traversal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFeatureName(tt.feature)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatalf("expected error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

func TestValidateFeatureNames(t *testing.T) {
	err := ValidateFeatureNames([]string{"valid_a", "valid_b"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = ValidateFeatureNames([]string{"valid", "", "also_valid"})
	if err == nil {
		t.Fatal("expected error for empty feature name in slice")
	}
}
