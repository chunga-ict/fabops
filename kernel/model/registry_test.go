package model

import (
	"testing"
)

func TestGetComponentType_Generic(t *testing.T) {
	// generic은 init()에서 이미 등록됨
	comp, err := GetComponentType("generic")
	if err != nil {
		t.Fatalf("expected generic to be registered, got error: %v", err)
	}
	if comp.Label() != "generic" {
		t.Errorf("expected label 'generic', got '%s'", comp.Label())
	}
}

func TestGetComponentType_NotFound(t *testing.T) {
	_, err := GetComponentType("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent component type")
	}
}

func TestGetComponentType_Controller(t *testing.T) {
	comp, err := GetComponentType("ziti-controller")
	if err != nil {
		t.Fatalf("expected ziti-controller to be registered, got error: %v", err)
	}
	if comp.Label() != "ziti-controller" {
		t.Errorf("expected label 'ziti-controller', got '%s'", comp.Label())
	}
}

func TestGetComponentType_Router(t *testing.T) {
	comp, err := GetComponentType("ziti-router")
	if err != nil {
		t.Fatalf("expected ziti-router to be registered, got error: %v", err)
	}
	if comp.Label() != "ziti-router" {
		t.Errorf("expected label 'ziti-router', got '%s'", comp.Label())
	}
}

func TestComponentType_Interfaces(t *testing.T) {
	// controller는 ServerComponent 구현해야 함
	ctrl, _ := GetComponentType("ziti-controller")
	if _, ok := ctrl.(ServerComponent); !ok {
		t.Error("ziti-controller should implement ServerComponent")
	}

	// router도 ServerComponent 구현해야 함
	router, _ := GetComponentType("ziti-router")
	if _, ok := router.(ServerComponent); !ok {
		t.Error("ziti-router should implement ServerComponent")
	}
}
