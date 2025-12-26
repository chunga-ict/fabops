package subcmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyCommand_ValidYAML(t *testing.T) {
	yaml := `
model:
  id: test-apply

regions:
  us-east-1:
    site: aws
    hosts:
      controller:
        instanceType: t3.medium
        components:
          - type: ziti-controller
`
	path := writeTempYaml(t, yaml)
	defer os.Remove(path)

	cmd := NewApplyCommand()
	cmd.SetArgs([]string{"--config", path, "--dry-run"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("apply command failed: %v", err)
	}
}

func TestApplyCommand_InvalidPath(t *testing.T) {
	cmd := NewApplyCommand()
	cmd.SetArgs([]string{"--config", "/nonexistent/path.yaml", "--dry-run"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestApplyCommand_InvalidYAML(t *testing.T) {
	path := writeTempYaml(t, "invalid: yaml: content:")
	defer os.Remove(path)

	cmd := NewApplyCommand()
	cmd.SetArgs([]string{"--config", path, "--dry-run"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestApplyCommand_UnknownComponent(t *testing.T) {
	yaml := `
model:
  id: test-unknown

regions:
  test:
    hosts:
      host1:
        components:
          - type: unknown-type
`
	path := writeTempYaml(t, yaml)
	defer os.Remove(path)

	cmd := NewApplyCommand()
	cmd.SetArgs([]string{"--config", path, "--dry-run"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown component type")
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
