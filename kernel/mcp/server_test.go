package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/openziti/fablab/kernel/model"
	"github.com/openziti/fablab/kernel/store"
)

func TestNewFablabMCPServer(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := NewFablabMCPServer(memStore)

	if server == nil {
		t.Fatal("expected server to be created")
	}
	if server.store == nil {
		t.Error("expected store to be set")
	}
	if server.reconciler == nil {
		t.Error("expected reconciler to be set")
	}
}

func TestListInstancesHandler(t *testing.T) {
	memStore := store.NewMemoryStore()

	// Add test instances
	memStore.SaveStatus("instance-1", &model.Label{InstanceId: "instance-1", State: model.Created})
	memStore.SaveStatus("instance-2", &model.Label{InstanceId: "instance-2", State: model.Disposed})

	server := NewFablabMCPServer(memStore)

	request := mcp.CallToolRequest{}
	result, err := server.listInstancesHandler(context.Background(), request)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var response map[string]interface{}
	json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &response)

	if int(response["count"].(float64)) != 2 {
		t.Errorf("expected 2 instances, got %v", response["count"])
	}
}

func TestGetInstanceHandler(t *testing.T) {
	memStore := store.NewMemoryStore()
	memStore.SaveStatus("test-instance", &model.Label{
		InstanceId: "test-instance",
		Model:      "test-model",
		State:      model.Created,
	})

	server := NewFablabMCPServer(memStore)

	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"instance_id": "test-instance",
			},
		},
	}

	result, err := server.getInstanceHandler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var response map[string]interface{}
	json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &response)

	if response["instance_id"] != "test-instance" {
		t.Errorf("expected instance_id 'test-instance', got %v", response["instance_id"])
	}
}

func TestGetInstanceHandler_NotFound(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := NewFablabMCPServer(memStore)

	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"instance_id": "nonexistent",
			},
		},
	}

	result, err := server.getInstanceHandler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("expected error result for nonexistent instance")
	}
}

func TestApplyConfigHandler_DryRun(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := NewFablabMCPServer(memStore)

	// Create test YAML file
	tmpDir, err := os.MkdirTemp("", "mcp-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "test.yaml")
	configContent := `
model:
  id: test-model
regions:
  us-east-1:
    hosts:
      host1:
        components:
          - type: ziti-controller
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"config_path": configPath,
				"dry_run":     true,
			},
		},
	}

	result, err := server.applyConfigHandler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Errorf("unexpected error: %v", result.Content)
	}

	var response map[string]interface{}
	json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &response)

	if response["dry_run"] != true {
		t.Error("expected dry_run to be true")
	}
	if response["model_id"] != "test-model" {
		t.Errorf("expected model_id 'test-model', got %v", response["model_id"])
	}
}

func TestGetResourcesHandler(t *testing.T) {
	memStore := store.NewMemoryStore()
	memStore.SaveResource("test-instance", store.ResourceState{
		Id:     "host-1",
		Type:   "host",
		Status: store.StatusRunning,
	})
	memStore.SaveResource("test-instance", store.ResourceState{
		Id:     "host-2",
		Type:   "host",
		Status: store.StatusDeleted,
	})

	server := NewFablabMCPServer(memStore)

	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"instance_id": "test-instance",
			},
		},
	}

	result, err := server.getResourcesHandler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var response map[string]interface{}
	json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &response)

	if int(response["count"].(float64)) != 2 {
		t.Errorf("expected 2 resources, got %v", response["count"])
	}
}

func TestStatusHandler(t *testing.T) {
	memStore := store.NewMemoryStore()
	memStore.SaveStatus("instance-1", &model.Label{InstanceId: "instance-1"})

	server := NewFablabMCPServer(memStore)

	request := mcp.ReadResourceRequest{}
	contents, err := server.statusHandler(context.Background(), request)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(contents))
	}

	textContent := contents[0].(mcp.TextResourceContents)
	if textContent.URI != "fablab://status" {
		t.Errorf("expected URI 'fablab://status', got %s", textContent.URI)
	}

	var response map[string]interface{}
	json.Unmarshal([]byte(textContent.Text), &response)

	if int(response["count"].(float64)) != 1 {
		t.Errorf("expected count 1, got %v", response["count"])
	}
}

func TestDeleteInstanceHandler(t *testing.T) {
	memStore := store.NewMemoryStore()
	memStore.SaveStatus("test-instance", &model.Label{InstanceId: "test-instance"})
	memStore.SaveResource("test-instance", store.ResourceState{
		Id:     "host-1",
		Type:   "host",
		Status: store.StatusRunning,
	})
	memStore.SaveResource("test-instance", store.ResourceState{
		Id:     "host-2",
		Type:   "host",
		Status: store.StatusRunning,
	})

	server := NewFablabMCPServer(memStore)

	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"instance_id": "test-instance",
			},
		},
	}

	result, err := server.deleteInstanceHandler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Error("unexpected error result")
	}

	var response map[string]interface{}
	json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &response)

	if int(response["deleted_count"].(float64)) != 2 {
		t.Errorf("expected deleted_count 2, got %v", response["deleted_count"])
	}

	// Verify resources are deleted
	resources, _ := memStore.GetResources("test-instance")
	if len(resources) != 0 {
		t.Errorf("expected 0 resources after delete, got %d", len(resources))
	}
}

func TestDeleteInstanceHandler_NotFound(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := NewFablabMCPServer(memStore)

	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"instance_id": "nonexistent",
			},
		},
	}

	result, err := server.deleteInstanceHandler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("expected error result for nonexistent instance")
	}
}

func TestGetDiffHandler(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := NewFablabMCPServer(memStore)

	// Create test YAML file
	tmpDir, err := os.MkdirTemp("", "mcp-diff-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "test.yaml")
	configContent := `
model:
  id: diff-test

regions:
  us-east-1:
    hosts:
      host1:
        components:
          - type: ziti-controller
      host2:
        components:
          - type: ziti-router
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"config_path": configPath,
			},
		},
	}

	result, err := server.getDiffHandler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Errorf("unexpected error: %v", result.Content)
	}

	var response map[string]interface{}
	json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &response)

	if response["model_id"] != "diff-test" {
		t.Errorf("expected model_id 'diff-test', got %v", response["model_id"])
	}

	if response["has_changes"] != true {
		t.Error("expected has_changes to be true for new config")
	}

	// Should have creates for hosts
	createCount := int(response["create_count"].(float64))
	if createCount < 2 {
		t.Errorf("expected at least 2 creates, got %d", createCount)
	}
}
