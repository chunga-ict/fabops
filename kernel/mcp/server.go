package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/openziti/fablab/kernel/engine"
	"github.com/openziti/fablab/kernel/loader"
	"github.com/openziti/fablab/kernel/model"
	"github.com/openziti/fablab/kernel/store"
)

// FablabMCPServer provides MCP interface for fablab infrastructure management.
type FablabMCPServer struct {
	server     *server.MCPServer
	store      store.ResourceStore
	reconciler *engine.Reconciler
}

// NewFablabMCPServer creates a new MCP server with the given store.
func NewFablabMCPServer(s store.ResourceStore) *FablabMCPServer {
	srv := server.NewMCPServer(
		"Fablab Digital Twin",
		"v1.0.0",
		server.WithResourceCapabilities(true, true),
		server.WithToolCapabilities(true),
	)

	fs := &FablabMCPServer{
		server:     srv,
		store:      s,
		reconciler: engine.NewReconciler(s),
	}

	fs.registerTools()
	fs.registerResources()

	return fs
}

// ServeStdio starts the MCP server on stdio.
func (fs *FablabMCPServer) ServeStdio() error {
	return server.ServeStdio(fs.server)
}

func (fs *FablabMCPServer) registerTools() {
	// list_instances tool
	listTool := mcp.NewTool("list_instances",
		mcp.WithDescription("List all fablab instances"),
	)
	fs.server.AddTool(listTool, fs.listInstancesHandler)

	// get_instance tool
	getTool := mcp.NewTool("get_instance",
		mcp.WithDescription("Get details of a specific instance"),
		mcp.WithString("instance_id",
			mcp.Description("ID of the instance"),
			mcp.Required(),
		),
	)
	fs.server.AddTool(getTool, fs.getInstanceHandler)

	// apply_config tool
	applyTool := mcp.NewTool("apply_config",
		mcp.WithDescription("Apply a YAML configuration to create/update infrastructure"),
		mcp.WithString("config_path",
			mcp.Description("Path to YAML configuration file"),
			mcp.Required(),
		),
		mcp.WithBoolean("dry_run",
			mcp.Description("Validate without applying changes"),
		),
	)
	fs.server.AddTool(applyTool, fs.applyConfigHandler)

	// get_resources tool
	resourcesTool := mcp.NewTool("get_resources",
		mcp.WithDescription("Get all resources for an instance"),
		mcp.WithString("instance_id",
			mcp.Description("ID of the instance"),
			mcp.Required(),
		),
	)
	fs.server.AddTool(resourcesTool, fs.getResourcesHandler)

	// create_network tool (legacy)
	createTool := mcp.NewTool("create_network",
		mcp.WithDescription("Create a new OpenZiti network instance"),
		mcp.WithString("name",
			mcp.Description("Name of the network instance"),
			mcp.Required(),
		),
		mcp.WithString("model",
			mcp.Description("Model type (e.g., aws, local)"),
			mcp.Required(),
		),
	)
	fs.server.AddTool(createTool, fs.createNetworkHandler)

	// delete_instance tool
	deleteTool := mcp.NewTool("delete_instance",
		mcp.WithDescription("Delete an instance and all its resources"),
		mcp.WithString("instance_id",
			mcp.Description("ID of the instance to delete"),
			mcp.Required(),
		),
		mcp.WithBoolean("force",
			mcp.Description("Force delete without confirmation"),
		),
	)
	fs.server.AddTool(deleteTool, fs.deleteInstanceHandler)

	// get_diff tool
	diffTool := mcp.NewTool("get_diff",
		mcp.WithDescription("Get the diff between desired YAML config and current state"),
		mcp.WithString("config_path",
			mcp.Description("Path to YAML configuration file"),
			mcp.Required(),
		),
	)
	fs.server.AddTool(diffTool, fs.getDiffHandler)
}

func (fs *FablabMCPServer) registerResources() {
	// Status resource
	statusResource := mcp.NewResource("fablab://status", "Fablab Status",
		mcp.WithResourceDescription("Current status of all Fablab network instances"),
		mcp.WithMIMEType("application/json"),
	)
	fs.server.AddResource(statusResource, fs.statusHandler)

	// Instances resource template
	instancesResource := mcp.NewResourceTemplate(
		"fablab://instances/{instance_id}",
		"Instance Details",
		mcp.WithTemplateDescription("Details of a specific instance"),
		mcp.WithTemplateMIMEType("application/json"),
	)
	fs.server.AddResourceTemplate(instancesResource, fs.instanceHandler)
}

// Tool Handlers

func (fs *FablabMCPServer) listInstancesHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	instances, err := fs.store.ListInstances()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list instances: %v", err)), nil
	}

	result, _ := json.MarshalIndent(map[string]interface{}{
		"count":     len(instances),
		"instances": instances,
	}, "", "  ")

	return mcp.NewToolResultText(string(result)), nil
}

func (fs *FablabMCPServer) getInstanceHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	instanceId, err := request.RequireString("instance_id")
	if err != nil {
		return mcp.NewToolResultError("instance_id is required"), nil
	}

	label, err := fs.store.GetStatus(instanceId)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("instance not found: %v", err)), nil
	}

	resources, _ := fs.store.GetResources(instanceId)

	result, _ := json.MarshalIndent(map[string]interface{}{
		"instance_id": instanceId,
		"state":       label.State,
		"model_id":    label.Model,
		"resources":   resources,
	}, "", "  ")

	return mcp.NewToolResultText(string(result)), nil
}

func (fs *FablabMCPServer) applyConfigHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	configPath, err := request.RequireString("config_path")
	if err != nil {
		return mcp.NewToolResultError("config_path is required"), nil
	}

	dryRun := request.GetBool("dry_run", false)

	// Load model from YAML
	m, err := loader.LoadModel(configPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to load config: %v", err)), nil
	}

	if dryRun {
		result, _ := json.MarshalIndent(map[string]interface{}{
			"dry_run":  true,
			"model_id": m.Id,
			"regions":  len(m.Regions),
			"status":   "valid",
		}, "", "  ")
		return mcp.NewToolResultText(string(result)), nil
	}

	// Create context and reconcile
	modelCtx := model.NewContext(m, nil, nil)
	reconcileResult, err := fs.reconciler.Reconcile(modelCtx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("reconciliation failed: %v", err)), nil
	}

	result, _ := json.MarshalIndent(map[string]interface{}{
		"model_id":  m.Id,
		"created":   reconcileResult.Created,
		"updated":   reconcileResult.Updated,
		"deleted":   reconcileResult.Deleted,
		"unchanged": reconcileResult.Unchanged,
	}, "", "  ")

	return mcp.NewToolResultText(string(result)), nil
}

func (fs *FablabMCPServer) getResourcesHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	instanceId, err := request.RequireString("instance_id")
	if err != nil {
		return mcp.NewToolResultError("instance_id is required"), nil
	}

	resources, err := fs.store.GetResources(instanceId)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get resources: %v", err)), nil
	}

	result, _ := json.MarshalIndent(map[string]interface{}{
		"instance_id": instanceId,
		"count":       len(resources),
		"resources":   resources,
	}, "", "  ")

	return mcp.NewToolResultText(string(result)), nil
}

func (fs *FablabMCPServer) createNetworkHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError("name argument is required"), nil
	}
	modelType, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError("model argument is required"), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Network '%s' (type: %s) created (simulation).", name, modelType)), nil
}

func (fs *FablabMCPServer) deleteInstanceHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	instanceId, err := request.RequireString("instance_id")
	if err != nil {
		return mcp.NewToolResultError("instance_id is required"), nil
	}

	// Check if instance exists
	_, err = fs.store.GetStatus(instanceId)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("instance not found: %v", err)), nil
	}

	// Get all resources for the instance
	resources, err := fs.store.GetResources(instanceId)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get resources: %v", err)), nil
	}

	// Delete all resources
	deletedCount := 0
	var deleteErrors []string
	for resourceId := range resources {
		if err := fs.store.DeleteResource(instanceId, resourceId); err != nil {
			deleteErrors = append(deleteErrors, fmt.Sprintf("%s: %v", resourceId, err))
		} else {
			deletedCount++
		}
	}

	response := map[string]interface{}{
		"instance_id":     instanceId,
		"deleted_count":   deletedCount,
		"total_resources": len(resources),
		"status":          "deleted",
	}

	if len(deleteErrors) > 0 {
		response["errors"] = deleteErrors
		response["status"] = "partial_delete"
	}

	result, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(result)), nil
}

func (fs *FablabMCPServer) getDiffHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	configPath, err := request.RequireString("config_path")
	if err != nil {
		return mcp.NewToolResultError("config_path is required"), nil
	}

	// Load model from YAML
	m, err := loader.LoadModel(configPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to load config: %v", err)), nil
	}

	// Create context and get diff
	modelCtx := model.NewContext(m, nil, nil)
	diff, err := fs.reconciler.GetDiff(modelCtx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to compute diff: %v", err)), nil
	}

	// Build change summaries
	creates := make([]map[string]interface{}, 0, len(diff.ToCreate))
	for _, c := range diff.ToCreate {
		creates = append(creates, map[string]interface{}{
			"id":   c.Id,
			"type": c.Type,
		})
	}

	updates := make([]map[string]interface{}, 0, len(diff.ToUpdate))
	for _, u := range diff.ToUpdate {
		updates = append(updates, map[string]interface{}{
			"id":      u.Id,
			"type":    u.Type,
			"changes": u.Changes,
		})
	}

	deletes := make([]map[string]interface{}, 0, len(diff.ToDelete))
	for _, d := range diff.ToDelete {
		deletes = append(deletes, map[string]interface{}{
			"id":   d.Id,
			"type": d.Type,
		})
	}

	response := map[string]interface{}{
		"model_id":     m.Id,
		"has_changes":  !diff.IsEmpty(),
		"total":        diff.Total(),
		"to_create":    creates,
		"to_update":    updates,
		"to_delete":    deletes,
		"create_count": len(diff.ToCreate),
		"update_count": len(diff.ToUpdate),
		"delete_count": len(diff.ToDelete),
	}

	result, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(result)), nil
}

// Resource Handlers

func (fs *FablabMCPServer) statusHandler(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	instances, err := fs.store.ListInstances()
	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}

	result, _ := json.MarshalIndent(map[string]interface{}{
		"count":     len(instances),
		"instances": instances,
	}, "", "  ")

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "fablab://status",
			MIMEType: "application/json",
			Text:     string(result),
		},
	}, nil
}

func (fs *FablabMCPServer) instanceHandler(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	// Extract instance_id from URI
	instanceId := request.Params.URI[len("fablab://instances/"):]

	label, err := fs.store.GetStatus(instanceId)
	if err != nil {
		return nil, fmt.Errorf("instance not found: %w", err)
	}

	resources, _ := fs.store.GetResources(instanceId)

	result, _ := json.MarshalIndent(map[string]interface{}{
		"instance_id": instanceId,
		"state":       label.State,
		"model_id":    label.Model,
		"resources":   resources,
	}, "", "  ")

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(result),
		},
	}, nil
}
