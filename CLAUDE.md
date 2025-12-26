# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Fablab is a Go-based infrastructure orchestration framework for deploying and managing large-scale, distributed OpenZiti networks. It operates as a "programming toolkit" where infrastructure, configuration, and operational workflows are expressed as actual Go code rather than DSLs or YAML.

The project is undergoing an **architectural transition** from code-centric CLI to configuration-driven platform with AI integration (MCP Server) and state management.

## Build and Test Commands

### Building the Project
```bash
# Build the fablab binary
go build -o fablab ./cmd/fablab

# Build is implicit in Go - the binary is already built as ./fablab if you see it
```

### Running Tests
```bash
# Run all tests
go test ./...

# Run tests with API test tags (as used in CI)
go test ./... --tags apitests

# Run a specific package's tests
go test ./kernel/model

# Run a specific test
go test ./kernel/model -run TestScopedVariableResolver
```

### Linting
```bash
# Project uses golangci-lint (see .golangci.yml for configuration)
golangci-lint run
```

## Core Architecture

Fablab's architecture is based on three orthogonal concepts that combine to create powerful distributed system deployments:

### 1. The Three-Layer Model Architecture

**Structural Model** (`kernel/model/model.go`)
- Digital twin of the distributed environment as Go data structures
- Hierarchy: `Model` → `Region` → `Host` → `Component`
- Each entity has a `Scope` with variables, tags, and lifecycle tracking
- Entities form a tree with parent/child relationships

**Behavioral Model** (interfaces in `kernel/model/`)
- `ComponentType`: Core interface every component must implement
  - Required: `Label()`, `GetVersion()`, `Dump()`, `IsRunning()`, `Stop()`
  - Optional: `ServerComponent`, `FileStagingComponent`, `HostInitializingComponent`
- `Factory`: Constructs and configures model structures
- `Action`: Discrete functions executable against a model
- `Stage`: Lifecycle phase implementations

**Instance Model** (`kernel/model/instance.go`, `kernel/model/label.go`)
- Manages multiple instances of models with unique operational state
- `Label`: Instance metadata and current lifecycle state
- Persisted across sessions in `~/.fablab/` directory

### 2. Lifecycle Stages (Deployment Pipeline)

Every model instance progresses through these stages:

1. **Infrastructure** (`Express`): Provision infrastructure (AWS, Terraform)
2. **Configuration** (`Build`): Generate config files, PKI, binaries
3. **Distribution** (`Sync`): Transfer artifacts to hosts via rsync/sftp
4. **Activation** (`Activate`): Start processes and initialize components
5. **Operation** (`Operate`): Run operational workflows
6. **Disposal** (`Dispose`): Tear down infrastructure

Each stage is represented by a `Stages` slice in the `Model` struct, executed sequentially.

### 3. Component Type System

**Critical Principle**: Components cannot be deployed unless they implement the required interfaces.

**Component Registry Pattern** (NEW - `kernel/model/registry.go`):
- `RegisterComponentType(name, factory)`: Register component implementations
- `GetComponentType(name)`: Retrieve component factory by name
- This enables YAML-driven deployments while maintaining type safety

**Component Interface Hierarchy**:
```
ComponentType (required for all)
  ├── ServerComponent (can be started)
  ├── FileStagingComponent (contributes files during Build)
  ├── HostInitializingComponent (configures host during Sync)
  ├── InitializingComponent (initializes during Activate)
  ├── InitializingComponentType (model-init hook)
  └── ActionsComponent (custom actions)
```

## Key Design Patterns

### Entity Hierarchy and Variable Resolution

Variables are resolved hierarchically through the entity tree:
1. Command-line arguments (`-V variable=value`)
2. Environment variables
3. Label data (instance-specific)
4. Bindings (global configuration)
5. Hierarchical scope (walks up parent chain)

The `Scope` struct on every entity provides variable storage and template rendering.

### Context-Based Architecture (NEW)

The project is transitioning from global singleton `model.GetModel()` to context passing:
- `Context` struct (`kernel/model/context.go`): Holds `Model`, `Label`, `Config`
- Enables multi-tenancy and daemon/server modes
- Breaking change: All `subcmd` implementations need updating

### Digital Twin and State Management (PLANNED)

Future architecture includes:
- `StateStore` interface (`kernel/store/interface.go`): Persist deployment state
- `Reconciler` (`kernel/engine/reconciler.go`): Compare desired vs. current state
- Enables GitOps workflows and state-aware operations

## CLI Command Structure

The CLI delegates to instance-specific binaries:

1. Commands like `completion`, `clean`, `serve`, `list instances` run locally
2. All other commands delegate to instance-specific executable
3. Instance configuration stored in `~/.fablab/config.yml`
4. Each instance has its own compiled binary with embedded model

**Common Commands**:
- `fablab create <instance>`: Create new model instance
- `fablab list`: Show all instances
- `fablab use <instance>`: Set default instance
- `fablab up`: Run Express → Build → Sync → Activate
- `fablab run <action>`: Execute custom action
- `fablab dispose`: Tear down infrastructure
- `fablab serve`: Start MCP server (NEW)

## MCP Integration (NEW)

The project implements Model Context Protocol for AI agent integration:

**Location**: `kernel/mcp/server.go`

**Capabilities**:
- **Resources**: `fablab://status` - Query deployment state
- **Tools**: `create_network` - Create new network instances
- **Server**: Stdio-based MCP server for Claude/Cursor integration

**Usage**:
```bash
fablab serve  # Start MCP server
```

This enables AI agents to query deployment status and orchestrate infrastructure.

## File Organization

```
fablab/
├── cmd/fablab/           # CLI entry point
│   ├── main.go           # Command delegation logic
│   └── subcmd/           # Cobra command implementations
├── kernel/               # Core framework
│   ├── model/            # Entity types, lifecycle, interfaces
│   ├── lib/              # Utilities and primitives
│   │   ├── actions/      # Reusable actions
│   │   ├── runlevel/     # Stage implementations (0-6)
│   │   └── parallel/     # Concurrency helpers
│   ├── libssh/           # SSH/SFTP operations
│   ├── loader/           # YAML model loading (NEW)
│   ├── store/            # State persistence (NEW)
│   ├── engine/           # Reconciliation logic (NEW)
│   └── mcp/              # MCP server (NEW)
├── resources/            # Embedded resources
└── docs/                 # Documentation and examples
```

## Important Implementation Notes

### Variable System
- Variables use dot notation: `credentials.ssh.username`
- Templates use Go's `text/template` syntax
- Secret detection: Keys matching `key`, `password`, `secret` patterns are redacted in logs

### SSH and Remote Execution
- Hosts maintain persistent SSH connections (`Host.sshClient`)
- `Host.Exec()`: Execute commands on remote hosts
- `Host.SendFile()`: Transfer files via SFTP
- `Host.KillProcesses()`: Process management with filters

### Concurrent Operations
- `Model.ForEachHost()`: Parallel host operations
- `Model.ForEachComponent()`: Parallel component operations
- Uses `kernel/lib/parallel` for controlled concurrency

### Metrics and Observability
- `MetricsHandler`: Custom metrics collection
- InfluxDB integration in `kernel/lib/runlevel/5_operation/`
- SAR (System Activity Report) integration for host metrics

## Transition Guidance

The codebase is actively evolving from v0.x to v1.x architecture:

**Current (Stable)**:
- Code-defined models in Go
- Singleton `model.GetModel()`
- CLI-based instance management

**In Progress**:
- Component registry system (`kernel/model/registry.go`)
- Context-based model passing (`kernel/model/context.go`)
- MCP server integration (`kernel/mcp/server.go`)

**Planned**:
- YAML-driven deployments (`kernel/loader/yaml_loader.go`)
- State store persistence (`kernel/store/`)
- Reconciliation engine (`kernel/engine/reconciler.go`)

When working on the codebase:
- Understand whether you're working with current or transitional code
- Check architecture documents in `docs/` and root `*.md` files
- New components should use registry pattern
- Avoid new uses of global `model.GetModel()` - prefer context passing

## Testing Philosophy

Tests are minimal (8 test files) and focused on:
- Variable resolution (`kernel/model/*_test.go`)
- Selector/matcher logic
- Core data structures

Infrastructure testing happens via actual deployments rather than unit tests.
