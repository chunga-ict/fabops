/*
	(c) Copyright NetFoundry Inc. Inc.

	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at

	https://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
*/

package subcmd

import (
	"os"
	"path/filepath"

	"github.com/openziti/fablab/kernel/mcp"
	"github.com/openziti/fablab/kernel/model"
	"github.com/openziti/fablab/kernel/store"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(NewMCPServerCommand())
}

func NewMCPServerCommand() *cobra.Command {
	mcpCmd := &MCPServerCommand{}

	cmd := &cobra.Command{
		Use:   "mcp-server",
		Short: "Start MCP server for AI-driven infrastructure management",
		Long: `Start an MCP (Model Context Protocol) server that exposes fablab
infrastructure management capabilities to AI assistants like Claude.

The server provides tools for:
  - list_instances: List all fablab instances
  - get_instance: Get details of a specific instance
  - apply_config: Apply YAML configuration to create/update infrastructure
  - get_resources: Get all resources for an instance
  - create_network: Create a new network instance

And resources:
  - fablab://status: Current status of all instances
  - fablab://instances/{id}: Details of a specific instance`,
		RunE: mcpCmd.run,
	}

	cmd.Flags().BoolVar(&mcpCmd.UseMemoryStore, "memory", false, "use in-memory store (for testing)")

	return cmd
}

type MCPServerCommand struct {
	UseMemoryStore bool
}

func (m *MCPServerCommand) run(cmd *cobra.Command, args []string) error {
	var resourceStore store.ResourceStore

	if m.UseMemoryStore {
		logrus.Info("using in-memory store")
		resourceStore = store.NewMemoryStore()
	} else {
		cfg := tryLoadConfig()
		if cfg == nil {
			logrus.Warn("could not load config, using memory store")
			resourceStore = store.NewMemoryStore()
		} else {
			resourceStore = store.NewFileStore(cfg)
		}
	}

	logrus.Info("starting MCP server on stdio...")
	server := mcp.NewFablabMCPServer(resourceStore)
	return server.ServeStdio()
}

func tryLoadConfig() *model.FablabConfig {
	cfgDir, err := model.ConfigDir()
	if err != nil {
		return nil
	}
	configFile := filepath.Join(cfgDir, model.ConfigFileName)
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return nil
	}
	cfg, err := model.LoadConfig(configFile)
	if err != nil {
		return nil
	}
	return cfg
}
