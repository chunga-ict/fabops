package model

import (
	"fmt"
	"strings"
)

// ZitiControllerType implements ComponentType for Ziti Controller
type ZitiControllerType struct {
	Version string `yaml:"version"`
}

func (c *ZitiControllerType) Label() string {
	return "ziti-controller"
}

func (c *ZitiControllerType) GetVersion() string {
	return c.Version
}

func (c *ZitiControllerType) Dump() any {
	return map[string]string{"version": c.Version}
}

func (c *ZitiControllerType) IsRunning(run Run, comp *Component) (bool, error) {
	host := comp.GetHost()
	output, _ := host.ExecLogged("pgrep -f ziti-controller || true")
	return len(strings.TrimSpace(output)) > 0, nil
}

func (c *ZitiControllerType) Stop(run Run, comp *Component) error {
	host := comp.GetHost()
	return host.KillProcesses("-TERM", func(line string) bool {
		return strings.Contains(line, "ziti-controller")
	})
}

// ServerComponent implementation
func (c *ZitiControllerType) Start(run Run, comp *Component) error {
	host := comp.GetHost()

	// Kill existing process
	_ = c.Stop(run, comp)

	// Start controller
	startCmd := fmt.Sprintf(
		"nohup %s/bin/ziti-controller run %s/cfg/controller.yml > %s/logs/controller.log 2>&1 &",
		run.GetWorkingDir(), run.GetWorkingDir(), run.GetWorkingDir(),
	)
	if _, err := host.ExecLogged(startCmd); err != nil {
		return fmt.Errorf("failed to start controller: %w", err)
	}

	return nil
}

func init() {
	RegisterComponentType("ziti-controller", func() ComponentType {
		return &ZitiControllerType{}
	})
}
