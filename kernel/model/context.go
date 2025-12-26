package model

type Context struct {
	Model  *Model
	Label  *Label
	Config *FablabConfig
}

func NewContext(m *Model, l *Label, c *FablabConfig) *Context {
	return &Context{
		Model:  m,
		Label:  l,
		Config: c,
	}
}

func (c *Context) GetModel() *Model {
	return c.Model
}

func (c *Context) GetLabel() *Label {
	return c.Label
}
