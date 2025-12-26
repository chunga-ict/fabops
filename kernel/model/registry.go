package model

import (
	"fmt"
	"sync"
)

// ComponentFactory creates a new instance of a Component Type (which is an interface)
type ComponentFactory func() ComponentType

var (
	registryMu sync.RWMutex
	registry   = make(map[string]ComponentFactory)
)

// RegisterComponentType registers a factory for a given component type name.
// e.g. RegisterComponentType("ziti-router", func() ComponentType { return &ZitiRouter{} })
func RegisterComponentType(typeName string, factory ComponentFactory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, dup := registry[typeName]; dup {
		panic("RegisterComponentType called twice for " + typeName)
	}
	registry[typeName] = factory
}

// GetComponentType creates a new instance of the component type by name.
func GetComponentType(typeName string) (ComponentType, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	factory, ok := registry[typeName]
	if !ok {
		return nil, fmt.Errorf("component type '%s' not found in registry", typeName)
	}
	return factory(), nil
}
