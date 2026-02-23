package gitopsdefs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ManifestLoader loads and validates declarative feature manifests from YAML files.
type ManifestLoader struct{}

// Manifest represents a parsed manifest file containing one or more specs.
type Manifest struct {
	Specs    []DeclarativeSpec `json:"specs"`
	FilePath string            `json:"file_path"`
	Errors   []string          `json:"errors,omitempty"`
}

// ValidationError describes a validation issue in a manifest.
type ValidationError struct {
	File    string `json:"file"`
	Line    int    `json:"line,omitempty"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Error implements the error interface.
func (ve ValidationError) Error() string {
	if ve.Line > 0 {
		return fmt.Sprintf("%s:%d: %s: %s", ve.File, ve.Line, ve.Field, ve.Message)
	}
	return fmt.Sprintf("%s: %s: %s", ve.File, ve.Field, ve.Message)
}

// NewManifestLoader creates a new ManifestLoader.
func NewManifestLoader() *ManifestLoader {
	return &ManifestLoader{}
}

// LoadFile loads and parses a YAML manifest file, supporting multi-document YAML.
func (ml *ManifestLoader) LoadFile(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest %s: %w", path, err)
	}

	manifest := &Manifest{
		FilePath: path,
	}

	specs, errs := parseMultiDocYAML(data)
	manifest.Specs = specs
	manifest.Errors = errs

	return manifest, nil
}

// LoadDirectory scans a directory for .yaml/.yml files and loads each one.
func (ml *ManifestLoader) LoadDirectory(dir string) ([]*Manifest, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
	}

	manifests := make([]*Manifest, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		m, err := ml.LoadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			manifests = append(manifests, &Manifest{
				FilePath: filepath.Join(dir, entry.Name()),
				Errors:   []string{err.Error()},
			})
			continue
		}
		manifests = append(manifests, m)
	}

	return manifests, nil
}

// ValidateManifest validates all specs within a manifest.
func (ml *ManifestLoader) ValidateManifest(m *Manifest) []ValidationError {
	var errs []ValidationError

	if len(m.Specs) == 0 && len(m.Errors) == 0 {
		errs = append(errs, ValidationError{
			File:    m.FilePath,
			Field:   "document",
			Message: "manifest contains no specs",
		})
		return errs
	}

	for i, spec := range m.Specs {
		result := ValidateSpec(&spec)
		for _, e := range result.Errors {
			errs = append(errs, ValidationError{
				File:    m.FilePath,
				Field:   fmt.Sprintf("specs[%d]", i),
				Message: e,
			})
		}
	}

	return errs
}

// parseMultiDocYAML splits on "---" separators and parses each document.
func parseMultiDocYAML(data []byte) ([]DeclarativeSpec, []string) {
	var specs []DeclarativeSpec
	var errs []string

	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	docIndex := 0
	for {
		var spec DeclarativeSpec
		err := decoder.Decode(&spec)
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			errs = append(errs, fmt.Sprintf("document %d: %s", docIndex, err.Error()))
			break
		}
		// Skip empty documents
		if spec.APIVersion == "" && spec.Kind == "" && spec.Metadata.Name == "" {
			docIndex++
			continue
		}
		specs = append(specs, spec)
		docIndex++
	}

	return specs, errs
}
