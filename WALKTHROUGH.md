# Fablab Intelligent Operator Walkthrough

This document outlines the major architectural changes implemented to transform Fablab into an AI-ready Intelligent Operator Platform.

## 1. Core Refactoring: Context-based Architecture

We moved away from global singletons to a dependency injection model.

- **Context Struct**: Defined in [`kernel/model/context.go`](file:///Users/luxmaior/fablab/kernel/model/context.go).
- **NewRun Injection**: Updated `NewRun` in `kernel/model/model.go` to accept `Model`, `Label`, and `Config` explicitly.
- **Subcommand Updates**: All subcommands now pass dependencies explicitly, paving the way for easier testing and multi-tenancy.

## 2. Configuration & Registry

We introduced a flexible component registry to support YAML-driven configuration.

- **Registry**: Implemented in [`kernel/model/registry.go`](file:///Users/luxmaior/fablab/kernel/model/registry.go).
- **YAML Loader**: Skeleton implementation in [`kernel/loader/yaml_loader.go`](file:///Users/luxmaior/fablab/kernel/loader/yaml_loader.go).
- **Generic Component**: Added a sample `GenericComponent` to demonstrate registration.

## 3. State Management (Digital Twin)

We formalized the Digital Twin concept with a dedicated State Store.

- **StateStore Interface**: Defined in [`kernel/store/interface.go`](file:///Users/luxmaior/fablab/kernel/store/interface.go).
- **FileStore**: Implemented backward-compatible file storage in [`kernel/store/file_store.go`](file:///Users/luxmaior/fablab/kernel/store/file_store.go).
- **Reconciler**: Created the foundation for state reconciliation in [`kernel/engine/reconciler.go`](file:///Users/luxmaior/fablab/kernel/engine/reconciler.go).

## 4. MCP Server (AI Interface)

We enabled direct AI interaction via the Model Context Protocol (MCP).

- **MCP Server**: Implemented in [`kernel/mcp/server.go`](file:///Users/luxmaior/fablab/kernel/mcp/server.go), utilizing `mark3labs/mcp-go`.
- **Capabilities**:
    - **Resource**: `fablab://status` for real-time network status.
    - **Tool**: `create_network` for AI-driven deployment.
- **Serve Command**: Added `fablab serve` command in [`cmd/fablab/subcmd/serve.go`](file:///Users/luxmaior/fablab/cmd/fablab/subcmd/serve.go).

## Verification

To verify the new capabilities:

1.  **Build**: `go build ./cmd/fablab`
2.  **Run Serve**: `./fablab serve` (Starts MCP server on Stdio)
3.  **Connect AI**: Configure Claude Desktop or Cursor to connect to the MCP server.

```bash
# Example MCP Configuration for Claude Desktop
{
  "mcpServers": {
    "fablab": {
      "command": "/path/to/fablab",
      "args": ["serve"]
    }
  }
}
```
