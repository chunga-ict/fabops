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
	"fmt"

	"github.com/openziti/fablab/kernel/engine"
	"github.com/openziti/fablab/kernel/loader"
	"github.com/openziti/fablab/kernel/model"
	"github.com/openziti/fablab/kernel/store"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(NewApplyCommand())
}

func NewApplyCommand() *cobra.Command {
	applyCmd := &ApplyCommand{}

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply a YAML configuration to create/update infrastructure",
		RunE:  applyCmd.apply,
	}

	cmd.Flags().StringVarP(&applyCmd.ConfigPath, "config", "c", "", "path to YAML configuration file")
	cmd.Flags().BoolVar(&applyCmd.DryRun, "dry-run", false, "validate configuration without applying")
	cmd.MarkFlagRequired("config")

	return cmd
}

type ApplyCommand struct {
	ConfigPath string
	DryRun     bool
}

func (a *ApplyCommand) apply(cmd *cobra.Command, args []string) error {
	m, err := loader.LoadModel(a.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if a.DryRun {
		logrus.Infof("dry-run: loaded model '%s' with %d region(s)", m.Id, len(m.Regions))
		for regionId, region := range m.Regions {
			logrus.Infof("  region '%s': %d host(s)", regionId, len(region.Hosts))
			for hostId, host := range region.Hosts {
				logrus.Infof("    host '%s': %d component(s)", hostId, len(host.Components))
			}
		}
		return nil
	}

	// Create context with the loaded model
	ctx := model.NewContext(m, nil, nil)

	// Create store (use MemoryStore for standalone apply, FileStore for instance-based)
	resourceStore := store.NewMemoryStore()

	// Create and run reconciler
	reconciler := engine.NewReconciler(resourceStore)
	result, err := reconciler.Reconcile(ctx)
	if err != nil {
		return fmt.Errorf("reconciliation failed: %w", err)
	}

	logrus.Infof("apply: model '%s' reconciled successfully", m.Id)
	logrus.Infof("  created: %d, updated: %d, deleted: %d, unchanged: %d",
		result.Created, result.Updated, result.Deleted, result.Unchanged)

	return nil
}
