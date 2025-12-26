package model

// Context holds all runtime state for a fablab session.
// It replaces the global singleton pattern with explicit dependency injection.
type Context struct {
	Model          *Model
	Label          *Label
	Config         *FablabConfig
	InstanceConfig *InstanceConfig
}

// NewContext creates a new context with all dependencies.
func NewContext(m *Model, l *Label, c *FablabConfig) *Context {
	return &Context{
		Model:  m,
		Label:  l,
		Config: c,
	}
}

// DefaultContext creates a context using global state (for backwards compatibility).
func DefaultContext() *Context {
	return &Context{
		Model:  model,
		Label:  label,
		Config: config,
	}
}

// GetModel returns the model.
func (c *Context) GetModel() *Model {
	return c.Model
}

// GetLabel returns the label.
func (c *Context) GetLabel() *Label {
	return c.Label
}

// GetConfig returns the config.
func (c *Context) GetConfig() *FablabConfig {
	return c.Config
}

// WithModel returns a new context with the given model.
func (c *Context) WithModel(m *Model) *Context {
	return &Context{
		Model:  m,
		Label:  c.Label,
		Config: c.Config,
	}
}

// WithLabel returns a new context with the given label.
func (c *Context) WithLabel(l *Label) *Context {
	return &Context{
		Model:  c.Model,
		Label:  l,
		Config: c.Config,
	}
}

// WithConfig returns a new context with the given config.
func (c *Context) WithConfig(cfg *FablabConfig) *Context {
	return &Context{
		Model:  c.Model,
		Label:  c.Label,
		Config: cfg,
	}
}

// NewRun creates a new Run instance for this context.
func (c *Context) NewRun() Run {
	return &runImpl{
		model: c.Model,
		label: c.Label,
	}
}

// MustBootstrapContext calls Bootstrap and returns a Context with all global state.
// This is the primary entry point for CLI commands during migration.
func MustBootstrapContext() (*Context, error) {
	if err := Bootstrap(); err != nil {
		return nil, err
	}
	return &Context{
		Model:          model,
		Label:          label,
		Config:         config,
		InstanceConfig: GetActiveInstanceConfig(),
	}, nil
}

// MustRun creates a Run from this context. It requires Model, Label, and InstanceConfig.
func (c *Context) MustRun() (Run, error) {
	return NewRun(c.Model, c.Label, c.InstanceConfig)
}
