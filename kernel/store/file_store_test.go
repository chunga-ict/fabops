package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/openziti/fablab/kernel/model"
)

func TestFileStore_ResourceStore(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "fablab-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	instanceDir := filepath.Join(tmpDir, "test-instance")
	if err := os.MkdirAll(instanceDir, 0755); err != nil {
		t.Fatalf("failed to create instance dir: %v", err)
	}

	cfg := &model.FablabConfig{
		Instances: map[string]*model.InstanceConfig{
			"test-instance": {
				WorkingDirectory: instanceDir,
			},
		},
	}

	store := NewFileStore(cfg)

	// Test GetResources on empty store
	resources, err := store.GetResources("test-instance")
	if err != nil {
		t.Fatalf("GetResources failed: %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}

	// Test SaveResource
	resource := ResourceState{
		Id:     "host-1",
		Type:   "host",
		Status: "running",
		Metadata: map[string]string{
			"regionId": "us-east-1",
		},
	}
	if err := store.SaveResource("test-instance", resource); err != nil {
		t.Fatalf("SaveResource failed: %v", err)
	}

	// Verify saved
	resources, err = store.GetResources("test-instance")
	if err != nil {
		t.Fatalf("GetResources failed: %v", err)
	}
	if len(resources) != 1 {
		t.Errorf("expected 1 resource, got %d", len(resources))
	}
	if resources["host-1"].Status != "running" {
		t.Errorf("expected status 'running', got '%s'", resources["host-1"].Status)
	}

	// Test DeleteResource
	if err := store.DeleteResource("test-instance", "host-1"); err != nil {
		t.Fatalf("DeleteResource failed: %v", err)
	}

	// Verify deleted
	resources, err = store.GetResources("test-instance")
	if err != nil {
		t.Fatalf("GetResources failed: %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources after delete, got %d", len(resources))
	}
}
