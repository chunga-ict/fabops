package model

import (
	"fmt"
	"strings"
)

// ZitiRouterType implements ComponentType for Ziti Router
type ZitiRouterType struct {
	Version string `yaml:"version"`
	Mode    string `yaml:"mode"` // "edge" or "fabric"
}

func (r *ZitiRouterType) Label() string {
	return "ziti-router"
}

func (r *ZitiRouterType) GetVersion() string {
	return r.Version
}

func (r *ZitiRouterType) Dump() any {
	return map[string]string{"version": r.Version, "mode": r.Mode}
}

func (r *ZitiRouterType) IsRunning(run Run, comp *Component) (bool, error) {
	host := comp.GetHost()
	output, _ := host.ExecLogged("pgrep -f ziti-router || true")
	return len(strings.TrimSpace(output)) > 0, nil
}

func (r *ZitiRouterType) Stop(run Run, comp *Component) error {
	host := comp.GetHost()
	return host.KillProcesses("-TERM", func(line string) bool {
		return strings.Contains(line, "ziti-router")
	})
}

// ServerComponent implementation
func (r *ZitiRouterType) Start(run Run, comp *Component) error {
	host := comp.GetHost()

	// Kill existing process
	_ = r.Stop(run, comp)

	// Start router
	configPath := fmt.Sprintf("%s/cfg/router-%s.yml", run.GetWorkingDir(), comp.Id)
	logPath := fmt.Sprintf("%s/logs/router-%s.log", run.GetWorkingDir(), comp.Id)

	startCmd := fmt.Sprintf(
		"nohup %s/bin/ziti-router run %s > %s 2>&1 &",
		run.GetWorkingDir(), configPath, logPath,
	)
	if _, err := host.ExecLogged(startCmd); err != nil {
		return fmt.Errorf("failed to start router: %w", err)
	}

	return nil
}

func init() {
	RegisterComponentType("ziti-router", func() ComponentType {
		return &ZitiRouterType{Mode: "edge"}
	})
}
