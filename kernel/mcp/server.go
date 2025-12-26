package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/openziti/fablab/kernel/store"
)

type FablabMCPServer struct {
	server *server.MCPServer
	store  store.StateStore
}

func NewFablabMCPServer(s store.StateStore) *FablabMCPServer {
	srv := server.NewMCPServer(
		"Fablab Digital Twin",
		"v1.0.0",
		server.WithResourceCapabilities(true, true),
		server.WithToolCapabilities(true),
	)

	fs := &FablabMCPServer{
		server: srv,
		store:  s,
	}

	fs.registerTools()
	fs.registerResources()

	return fs
}

func (fs *FablabMCPServer) ServeStdio() error {
	return server.ServeStdio(fs.server)
}

func (fs *FablabMCPServer) registerTools() {
	tool := mcp.NewTool("create_network",
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
	fs.server.AddTool(tool, fs.createNetworkHandler)
}

func (fs *FablabMCPServer) registerResources() {
	resource := mcp.NewResource("fablab://status", "Fablab Status",
		mcp.WithResourceDescription("Current status of all Fablab network instances"),
		mcp.WithMIMEType("application/json"),
	)
	fs.server.AddResource(resource, fs.statusHandler)
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

func (fs *FablabMCPServer) statusHandler(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	instances, err := fs.store.ListInstances()
	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}

	result := fmt.Sprintf(`{"count": %d, "instances": %v}`, len(instances), instances)

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "fablab://status",
			MIMEType: "application/json",
			Text:     result,
		},
	}, nil
}
