package model

type GenericComponent struct {
	Type    string `yaml:"type"`
	Version string `yaml:"version"`
}

func (c *GenericComponent) Label() string {
	return c.Type
}

func (c *GenericComponent) GetVersion() string {
	return c.Version
}

func (c *GenericComponent) Dump() any {
	return c
}

func (c *GenericComponent) IsRunning(run Run, component *Component) (bool, error) {
	// Basic implementation: always return false for now
	return false, nil
}

func (c *GenericComponent) Stop(run Run, component *Component) error {
	return nil
}

func init() {
	RegisterComponentType("generic", func() ComponentType { return &GenericComponent{} })
}
