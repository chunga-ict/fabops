# Fablab Architecture Transition Plan: "Intelligent Operator"

## Goal Description
Transform Fablab from a code-centric CLI tool into a configuration-driven, AI-enabled infrastructure platform. The new architecture will support:
1.  **YAML Configuration**: Declarative deployment (GitOps ready).
2.  **State Management**: Robust Digital Twin with pluggable State Store.
3.  **MCP Integration**: Native support for AI agents (Claude/Cursor) via Model Context Protocol.
4.  **No Globals**: Refactored kernel for stability and multi-tenancy.

## User Review Required
> [!IMPORTANT]
> **Breaking Change**: The refactoring of `kernel/model` to remove global variables (`model.GetModel()`) will require updates to all existing `subcmd` implementations.
> **Dependency**: This plan introduces `github.com/mark3labs/mcp-go` dependency.

## Proposed Changes

### 1. Kernel Refactoring (The Foundation)
Remove the singleton pattern to enable the Daemon/Server mode.

#### [NEW] `kernel/model/context.go`
- Define `Context` struct holding `Model`, `Label` (State), and `Config`.
- Replace `model.GetModel()` calls with context passing.

#### [MODIFY] `kernel/model/globals.go`
- Deprecate/Remove `var model *Model`.

### 2. Configuration & Registry
Enable "Configuration-driven" deployments.

#### [NEW] `kernel/registry/registry.go`
- Implement `ComponentRegistry` map.
- Functions: `Register(name, factory)`, `Get(name)`.

#### [NEW] `kernel/loader/yaml_loader.go`
- Logic to parse `fablab.yaml` and look up components in Registry.
- Instantiates a `model.Model` from the configuration.

### 3. State Management (Digital Twin)
Formalize the Digital Twin concept.

#### [NEW] `kernel/store/interface.go`
- `StateStore` interface: `GetStatus(id)`, `SaveStatus(id, state)`.

#### [NEW] `kernel/engine/reconciler.go`
- Logic to compare `DesiredState` (from YAML) vs `CurrentState` (from Store).
- "Discovery Phase": Check if the controller exists before modifying.

### 4. MCP Server Implementation
Add the AI interface.

#### [NEW] `kernel/mcp/server.go`
- Use `mark3labs/mcp-go`.
- Implement Resources: `fablab://status/{id}`.
- Implement Tools: `create_network`, `scale`, `logs`.

#### [MODIFY] `cmd/fablab/main.go`
- Add `--mcp` flag or `serve` subcommand to start the MCP server.

## Verification Plan

### Automated Tests
- **Unit Tests**: Test `Registry` lookup and `YAML` parsing.
- **Mock Tests**: Verify `Reconciler` logic using a Mock State Store.

### Manual Verification
1.  **Refactor Check**: Run existing CLI commands (`fablab list`) to ensure no regression after removing globals.
2.  **Config Deployment**: Create a `model.yaml` and deploy using `fablab apply --config model.yaml`.
3.  **MCP Connection**: Connect Claude Desktop to the running `fablab serve` process and ask "What is the status of my network?".
