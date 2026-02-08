package contract

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ValidateContractFiles validates contract definition files from a directory.
// This is designed for CI/CD integration (pre-commit hooks, GitHub Actions).
func ValidateContractFiles(paths []string) (*BatchValidationResult, error) {
	result := &BatchValidationResult{
		Files: make([]FileValidationResult, 0, len(paths)),
	}

	for _, path := range paths {
		fr := validateContractFile(path)
		result.Files = append(result.Files, fr)
		if !fr.Valid {
			result.HasErrors = true
		}
	}

	return result, nil
}

// DiscoverContractFiles finds all contract definition files in a directory.
func DiscoverContractFiles(dir string) ([]string, error) {
	var files []string
	patterns := []string{"*.contract.yaml", "*.contract.yml", "*.contract.json"}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			return nil, fmt.Errorf("globbing %s: %w", pattern, err)
		}
		files = append(files, matches...)
	}

	// Also check subdirectories
	entries, err := os.ReadDir(dir)
	if err != nil {
		return files, nil
	}
	for _, entry := range entries {
		if entry.IsDir() {
			subFiles, err := DiscoverContractFiles(filepath.Join(dir, entry.Name()))
			if err != nil {
				continue
			}
			files = append(files, subFiles...)
		}
	}

	return files, nil
}

// BatchValidationResult aggregates validation results for multiple files.
type BatchValidationResult struct {
	Files     []FileValidationResult `json:"files"`
	HasErrors bool                   `json:"has_errors"`
}

// FileValidationResult is the validation result for a single contract file.
type FileValidationResult struct {
	Path       string            `json:"path"`
	Valid      bool              `json:"valid"`
	Contract   string            `json:"contract_name,omitempty"`
	Errors     []ValidationError `json:"errors,omitempty"`
	Warnings   []string          `json:"warnings,omitempty"`
}

// Summary returns a human-readable summary of the batch validation.
func (b *BatchValidationResult) Summary() string {
	valid := 0
	invalid := 0
	for _, f := range b.Files {
		if f.Valid {
			valid++
		} else {
			invalid++
		}
	}
	if invalid == 0 {
		return fmt.Sprintf("✅ All %d contract(s) valid", valid)
	}
	return fmt.Sprintf("❌ %d of %d contract(s) invalid", invalid, valid+invalid)
}

func validateContractFile(path string) FileValidationResult {
	data, err := os.ReadFile(path)
	if err != nil {
		return FileValidationResult{
			Path:   path,
			Valid:  false,
			Errors: []ValidationError{{Field: "file", Message: fmt.Sprintf("cannot read: %v", err)}},
		}
	}

	var def ContractDefinition
	ext := filepath.Ext(path)
	switch ext {
	case ".yaml", ".yml":
		err = yaml.Unmarshal(data, &def)
	case ".json":
		err = json.Unmarshal(data, &def)
	default:
		return FileValidationResult{
			Path:   path,
			Valid:  false,
			Errors: []ValidationError{{Field: "file", Message: fmt.Sprintf("unsupported format: %s", ext)}},
		}
	}
	if err != nil {
		return FileValidationResult{
			Path:   path,
			Valid:  false,
			Errors: []ValidationError{{Field: "file", Message: fmt.Sprintf("parse error: %v", err)}},
		}
	}

	vr := ValidateDefinition(&def)
	return FileValidationResult{
		Path:     path,
		Valid:    vr.Valid,
		Contract: def.Name,
		Errors:   vr.Errors,
		Warnings: vr.Warnings,
	}
}
