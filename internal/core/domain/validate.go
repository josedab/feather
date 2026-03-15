package domain

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	// MaxEntityKeyLength is the maximum allowed length for entity keys.
	MaxEntityKeyLength = 512
	// MaxFeatureNameLength is the maximum allowed length for feature names.
	MaxFeatureNameLength = 256
)

// ValidateEntityKey checks that an entity key is well-formed.
// Entity keys must be non-empty, valid UTF-8, within length limits,
// and must not contain control characters or path traversal sequences.
func ValidateEntityKey(key string) error {
	if key == "" {
		return fmt.Errorf("entity key must not be empty")
	}
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("entity key must not be whitespace-only")
	}
	if len(key) > MaxEntityKeyLength {
		return fmt.Errorf("entity key exceeds maximum length of %d bytes", MaxEntityKeyLength)
	}
	if !utf8.ValidString(key) {
		return fmt.Errorf("entity key must be valid UTF-8")
	}
	if containsControlChars(key) {
		return fmt.Errorf("entity key must not contain control characters")
	}
	if containsPathTraversal(key) {
		return fmt.Errorf("entity key must not contain path traversal sequences")
	}
	return nil
}

// ValidateFeatureName checks that a feature name is well-formed.
// Feature names must be non-empty, valid UTF-8, within length limits,
// and must not contain control characters or path traversal sequences.
func ValidateFeatureName(name string) error {
	if name == "" {
		return fmt.Errorf("feature name must not be empty")
	}
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("feature name must not be whitespace-only")
	}
	if len(name) > MaxFeatureNameLength {
		return fmt.Errorf("feature name exceeds maximum length of %d bytes", MaxFeatureNameLength)
	}
	if !utf8.ValidString(name) {
		return fmt.Errorf("feature name must be valid UTF-8")
	}
	if containsControlChars(name) {
		return fmt.Errorf("feature name must not contain control characters")
	}
	if containsPathTraversal(name) {
		return fmt.Errorf("feature name must not contain path traversal sequences")
	}
	return nil
}

// ValidateFeatureNames validates a slice of feature names.
func ValidateFeatureNames(names []string) error {
	for _, name := range names {
		if err := ValidateFeatureName(name); err != nil {
			return err
		}
	}
	return nil
}

func containsControlChars(s string) bool {
	for _, r := range s {
		if r < 0x20 && r != '\t' {
			return true
		}
	}
	return false
}

func containsPathTraversal(s string) bool {
	return strings.Contains(s, "../") || strings.Contains(s, "..\\")
}
