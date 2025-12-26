package subcmd

import (
	"github.com/openziti/fablab/kernel/mcp"
	"github.com/openziti/fablab/kernel/model"
	"github.com/openziti/fablab/kernel/store"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(serveCmd)
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Fablab MCP Server",
	Run: func(cmd *cobra.Command, args []string) {
		// Initialize Config (Global Load)
		cfg := model.GetConfig()

		// Initialize Store
		s := store.NewFileStore(cfg)

		// Initialize MCP Server
		srv := mcp.NewFablabMCPServer(s)

		logrus.Info("Starting Fablab MCP Server on Stdio...")
		if err := srv.ServeStdio(); err != nil {
			logrus.WithError(err).Fatal("MCP Server Error")
		}
	},
}
