package model

import (
	"testing"
)

func TestNewContext(t *testing.T) {
	m := &Model{Id: "test-model"}
	l := &Label{InstanceId: "test-label"}
	c := &FablabConfig{}

	ctx := NewContext(m, l, c)

	if ctx.GetModel() != m {
		t.Error("GetModel() should return the model")
	}
	if ctx.GetLabel() != l {
		t.Error("GetLabel() should return the label")
	}
}

func TestContext_WithModel(t *testing.T) {
	ctx := &Context{}
	m := &Model{Id: "new-model"}

	newCtx := ctx.WithModel(m)

	if newCtx.GetModel() != m {
		t.Error("WithModel should set the model")
	}
	// Original context should be unchanged
	if ctx.GetModel() != nil {
		t.Error("Original context should not be modified")
	}
}

func TestContext_WithLabel(t *testing.T) {
	ctx := &Context{}
	l := &Label{InstanceId: "new-label"}

	newCtx := ctx.WithLabel(l)

	if newCtx.GetLabel() != l {
		t.Error("WithLabel should set the label")
	}
}

func TestDefaultContext(t *testing.T) {
	// Reset global state for test
	oldModel := model
	defer func() { model = oldModel }()

	m := &Model{Id: "default-test"}
	model = m

	ctx := DefaultContext()

	if ctx.GetModel() != m {
		t.Error("DefaultContext should use global model")
	}
}

func TestContext_Run(t *testing.T) {
	m := &Model{
		Id:      "run-test",
		Regions: make(Regions),
	}
	ctx := NewContext(m, nil, nil)

	run := ctx.NewRun()

	if run.GetModel() != m {
		t.Error("Run should reference context model")
	}
}

func TestContext_MustRun(t *testing.T) {
	m := &Model{
		Id:      "must-run-test",
		Regions: make(Regions),
	}
	l := &Label{InstanceId: "test-instance"}
	cfg := &InstanceConfig{}

	ctx := &Context{
		Model:          m,
		Label:          l,
		InstanceConfig: cfg,
	}

	run, err := ctx.MustRun()
	if err != nil {
		t.Fatalf("MustRun failed: %v", err)
	}
	if run.GetModel() != m {
		t.Error("MustRun should reference context model")
	}
}
