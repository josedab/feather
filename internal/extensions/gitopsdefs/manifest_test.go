package gitopsdefs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFile_ValidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "features.yaml")
	content := `apiVersion: feather/v1
kind: FeatureGroup
metadata:
  name: user_features
spec:
  entityType: user
  features:
    - name: age
      type: int64
    - name: email
      type: string
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	ml := NewManifestLoader()
	m, err := ml.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if len(m.Specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(m.Specs))
	}
	if m.Specs[0].Metadata.Name != "user_features" {
		t.Errorf("expected name user_features, got %q", m.Specs[0].Metadata.Name)
	}
	if m.Specs[0].Spec.EntityType != "user" {
		t.Errorf("expected entityType user, got %q", m.Specs[0].Spec.EntityType)
	}
	if len(m.Specs[0].Spec.Features) != 2 {
		t.Errorf("expected 2 features, got %d", len(m.Specs[0].Spec.Features))
	}
}

func TestLoadFile_MultiDocument(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "multi.yaml")
	content := `apiVersion: feather/v1
kind: FeatureGroup
metadata:
  name: users
spec:
  entityType: user
  features:
    - name: age
      type: int64
---
apiVersion: feather/v1
kind: FeatureGroup
metadata:
  name: orders
spec:
  entityType: order
  features:
    - name: total
      type: float64
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	ml := NewManifestLoader()
	m, err := ml.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if len(m.Specs) != 2 {
		t.Fatalf("expected 2 specs, got %d", len(m.Specs))
	}
	if m.Specs[0].Metadata.Name != "users" {
		t.Errorf("expected first spec name users, got %q", m.Specs[0].Metadata.Name)
	}
	if m.Specs[1].Metadata.Name != "orders" {
		t.Errorf("expected second spec name orders, got %q", m.Specs[1].Metadata.Name)
	}
}

func TestLoadFile_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	content := `apiVersion: feather/v1
kind: FeatureGroup
  invalid_indent: [
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	ml := NewManifestLoader()
	m, err := ml.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile should not return error for parse issues: %v", err)
	}
	if len(m.Errors) == 0 {
		t.Fatal("expected parse errors for invalid YAML")
	}
}

func TestLoadFile_NotFound(t *testing.T) {
	ml := NewManifestLoader()
	_, err := ml.LoadFile("/nonexistent/path.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadDirectory(t *testing.T) {
	dir := t.TempDir()

	yaml1 := `apiVersion: feather/v1
kind: FeatureGroup
metadata:
  name: users
spec:
  entityType: user
  features:
    - name: age
      type: int64
`
	yaml2 := `apiVersion: feather/v1
kind: FeatureGroup
metadata:
  name: orders
spec:
  entityType: order
  features:
    - name: total
      type: float64
`

	os.WriteFile(filepath.Join(dir, "users.yaml"), []byte(yaml1), 0644)
	os.WriteFile(filepath.Join(dir, "orders.yml"), []byte(yaml2), 0644)
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("ignore me"), 0644)

	ml := NewManifestLoader()
	manifests, err := ml.LoadDirectory(dir)
	if err != nil {
		t.Fatalf("LoadDirectory: %v", err)
	}
	if len(manifests) != 2 {
		t.Fatalf("expected 2 manifests, got %d", len(manifests))
	}

	names := make(map[string]bool)
	for _, m := range manifests {
		for _, s := range m.Specs {
			names[s.Metadata.Name] = true
		}
	}
	if !names["users"] || !names["orders"] {
		t.Errorf("expected users and orders specs, got %v", names)
	}
}

func TestLoadDirectory_NotFound(t *testing.T) {
	ml := NewManifestLoader()
	_, err := ml.LoadDirectory("/nonexistent/dir")
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
}

func TestLoadDirectory_SkipsSubdirs(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "subdir"), 0755)
	os.WriteFile(filepath.Join(dir, "subdir", "nested.yaml"), []byte("apiVersion: v1"), 0644)

	ml := NewManifestLoader()
	manifests, err := ml.LoadDirectory(dir)
	if err != nil {
		t.Fatalf("LoadDirectory: %v", err)
	}
	if len(manifests) != 0 {
		t.Errorf("expected 0 manifests (subdir should be skipped), got %d", len(manifests))
	}
}

func TestValidateManifest_Valid(t *testing.T) {
	ml := NewManifestLoader()
	m := &Manifest{
		FilePath: "test.yaml",
		Specs: []DeclarativeSpec{
			{
				APIVersion: "feather/v1",
				Kind:       "FeatureGroup",
				Metadata:   SpecMetadata{Name: "valid_group"},
				Spec: FeatureGroupSpec{
					EntityType: "user",
					Features:   []FeatureSpec{{Name: "age", Type: "int64"}},
				},
			},
		},
	}

	errs := ml.ValidateManifest(m)
	if len(errs) != 0 {
		t.Fatalf("expected no validation errors, got %v", errs)
	}
}

func TestValidateManifest_Invalid(t *testing.T) {
	ml := NewManifestLoader()
	m := &Manifest{
		FilePath: "bad.yaml",
		Specs: []DeclarativeSpec{
			{
				Kind:     "FeatureGroup",
				Metadata: SpecMetadata{Name: "missing_version"},
			},
		},
	}

	errs := ml.ValidateManifest(m)
	if len(errs) == 0 {
		t.Fatal("expected validation errors")
	}

	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "apiVersion") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected apiVersion error, got %v", errs)
	}
}

func TestValidateManifest_Empty(t *testing.T) {
	ml := NewManifestLoader()
	m := &Manifest{FilePath: "empty.yaml"}

	errs := ml.ValidateManifest(m)
	if len(errs) != 1 {
		t.Fatalf("expected 1 validation error for empty manifest, got %d", len(errs))
	}
	if !strings.Contains(errs[0].Message, "no specs") {
		t.Errorf("expected 'no specs' error, got %q", errs[0].Message)
	}
}

func TestValidationError_Error(t *testing.T) {
	ve := ValidationError{File: "test.yaml", Line: 10, Field: "kind", Message: "is required"}
	if !strings.Contains(ve.Error(), "test.yaml:10") {
		t.Errorf("expected line number in error, got %q", ve.Error())
	}

	ve2 := ValidationError{File: "test.yaml", Field: "kind", Message: "is required"}
	if strings.Contains(ve2.Error(), ":0") {
		t.Errorf("expected no line number when Line=0, got %q", ve2.Error())
	}
}

func TestLoadFile_WithLabelsAndAnnotations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "labeled.yaml")
	content := `apiVersion: feather/v1
kind: FeatureGroup
metadata:
  name: labeled_group
  namespace: production
  labels:
    team: ml
    env: prod
  annotations:
    description: A labeled feature group
spec:
  entityType: user
  owner: ml-team
  ttl: 1h
  features:
    - name: score
      type: float64
      required: true
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	ml := NewManifestLoader()
	m, err := ml.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if len(m.Specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(m.Specs))
	}

	spec := m.Specs[0]
	if spec.Metadata.Namespace != "production" {
		t.Errorf("expected namespace production, got %q", spec.Metadata.Namespace)
	}
	if spec.Metadata.Labels["team"] != "ml" {
		t.Errorf("expected label team=ml, got %v", spec.Metadata.Labels)
	}
	if spec.Spec.Owner != "ml-team" {
		t.Errorf("expected owner ml-team, got %q", spec.Spec.Owner)
	}
	if !spec.Spec.Features[0].Required {
		t.Error("expected feature to be required")
	}
}
