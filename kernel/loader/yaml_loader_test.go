package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/openziti/fablab/kernel/model"
)

func TestLoadModel_Basic(t *testing.T) {
	yaml := `
model:
  id: test-network

regions:
  us-east-1:
    site: aws
    hosts:
      controller:
        instanceType: t3.medium
        components:
          - type: ziti-controller
      router-1:
        instanceType: t3.small
        components:
          - type: ziti-router
`
	path := writeTempYaml(t, yaml)
	defer os.Remove(path)

	m, err := LoadModel(path)
	if err != nil {
		t.Fatalf("LoadModel failed: %v", err)
	}

	// Check model ID
	if m.Id != "test-network" {
		t.Errorf("expected model id 'test-network', got '%s'", m.Id)
	}

	// Check regions
	if len(m.Regions) != 1 {
		t.Fatalf("expected 1 region, got %d", len(m.Regions))
	}

	region, ok := m.Regions["us-east-1"]
	if !ok {
		t.Fatal("region 'us-east-1' not found")
	}

	// Check hosts
	if len(region.Hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(region.Hosts))
	}

	ctrl, ok := region.Hosts["controller"]
	if !ok {
		t.Fatal("host 'controller' not found")
	}
	if ctrl.InstanceType != "t3.medium" {
		t.Errorf("expected instanceType 't3.medium', got '%s'", ctrl.InstanceType)
	}

	// Check components
	if len(ctrl.Components) != 1 {
		t.Fatalf("expected 1 component, got %d", len(ctrl.Components))
	}
}

func TestLoadModel_ComponentRegistry(t *testing.T) {
	yaml := `
model:
  id: registry-test

regions:
  test-region:
    hosts:
      host1:
        components:
          - type: ziti-controller
          - type: ziti-router
`
	path := writeTempYaml(t, yaml)
	defer os.Remove(path)

	m, err := LoadModel(path)
	if err != nil {
		t.Fatalf("LoadModel failed: %v", err)
	}

	host := m.Regions["test-region"].Hosts["host1"]
	if len(host.Components) != 2 {
		t.Fatalf("expected 2 components, got %d", len(host.Components))
	}

	// Verify component types from registry
	for _, comp := range host.Components {
		if comp.Type == nil {
			t.Error("component Type should not be nil")
			continue
		}
		label := comp.Type.Label()
		if label != "ziti-controller" && label != "ziti-router" {
			t.Errorf("unexpected component label: %s", label)
		}
	}
}

func TestLoadModel_UnknownComponentType(t *testing.T) {
	yaml := `
model:
  id: error-test

regions:
  test-region:
    hosts:
      host1:
        components:
          - type: unknown-component
`
	path := writeTempYaml(t, yaml)
	defer os.Remove(path)

	_, err := LoadModel(path)
	if err == nil {
		t.Fatal("expected error for unknown component type")
	}
}

func TestLoadModel_FileNotFound(t *testing.T) {
	_, err := LoadModel("/nonexistent/path.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func writeTempYaml(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return path
}

// Ensure model package types are registered
var _ = model.GetModel

// Validation Tests

func TestValidateConfig_Valid(t *testing.T) {
	yaml := `
model:
  id: valid-network

regions:
  us-east-1:
    site: aws
    hosts:
      controller:
        instanceType: t3.medium
        components:
          - type: ziti-controller
`
	result, err := ValidateConfigBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("ValidateConfigBytes failed: %v", err)
	}

	if !result.IsValid() {
		t.Errorf("expected valid config, got errors: %v", result.Errors)
	}
}

func TestValidateConfig_MissingModelId(t *testing.T) {
	yaml := `
model:
  id: ""

regions:
  us-east-1:
    hosts:
      host1:
        components:
          - type: ziti-controller
`
	result, err := ValidateConfigBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("ValidateConfigBytes failed: %v", err)
	}

	if result.IsValid() {
		t.Error("expected validation errors for missing model id")
	}

	hasError := false
	for _, e := range result.Errors {
		if e.Path == "model.id" {
			hasError = true
			break
		}
	}
	if !hasError {
		t.Error("expected error for model.id path")
	}
}

func TestValidateConfig_InvalidIdFormat(t *testing.T) {
	yaml := `
model:
  id: 123-invalid

regions:
  us-east-1:
    hosts:
      host1:
        components:
          - type: ziti-controller
`
	result, err := ValidateConfigBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("ValidateConfigBytes failed: %v", err)
	}

	if result.IsValid() {
		t.Error("expected validation errors for invalid id format")
	}
}

func TestValidateConfig_UnknownComponentType(t *testing.T) {
	yaml := `
model:
  id: test-network

regions:
  us-east-1:
    hosts:
      host1:
        components:
          - type: unknown-component
`
	result, err := ValidateConfigBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("ValidateConfigBytes failed: %v", err)
	}

	if result.IsValid() {
		t.Error("expected validation errors for unknown component type")
	}

	hasCompError := false
	for _, e := range result.Errors {
		if e.Message != "" && len(e.Message) > 0 {
			hasCompError = true
			break
		}
	}
	if !hasCompError {
		t.Error("expected error about unknown component type")
	}
}

func TestValidateConfig_NoRegions(t *testing.T) {
	yaml := `
model:
  id: empty-network

regions: {}
`
	result, err := ValidateConfigBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("ValidateConfigBytes failed: %v", err)
	}

	// Should be valid but have warning
	if len(result.Warnings) == 0 {
		t.Error("expected warning for empty regions")
	}
}

func TestValidateConfig_DuplicateComponentIds(t *testing.T) {
	yaml := `
model:
  id: dup-test

regions:
  us-east-1:
    hosts:
      host1:
        components:
          - type: ziti-controller
            id: comp1
          - type: ziti-router
            id: comp1
`
	result, err := ValidateConfigBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("ValidateConfigBytes failed: %v", err)
	}

	if result.IsValid() {
		t.Error("expected validation errors for duplicate component ids")
	}
}
