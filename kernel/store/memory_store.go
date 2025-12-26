package store

import (
	"fmt"
	"sync"

	"github.com/openziti/fablab/kernel/model"
)

// MemoryStore is an in-memory implementation of ResourceStore for testing.
type MemoryStore struct {
	mu        sync.RWMutex
	instances map[string]*model.Label
	resources map[string]map[string]ResourceState // instanceId -> resourceId -> state
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		instances: make(map[string]*model.Label),
		resources: make(map[string]map[string]ResourceState),
	}
}

func (s *MemoryStore) GetStatus(instanceId string) (*model.Label, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	label, ok := s.instances[instanceId]
	if !ok {
		return nil, fmt.Errorf("instance [%s] not found", instanceId)
	}
	return label, nil
}

func (s *MemoryStore) SaveStatus(instanceId string, label *model.Label) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.instances[instanceId] = label
	return nil
}

func (s *MemoryStore) ListInstances() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.instances))
	for k := range s.instances {
		keys = append(keys, k)
	}
	return keys, nil
}

// GetResources returns all resources for an instance.
func (s *MemoryStore) GetResources(instanceId string) (map[string]ResourceState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resources, ok := s.resources[instanceId]
	if !ok {
		return make(map[string]ResourceState), nil
	}

	// Return a copy to prevent concurrent modification
	result := make(map[string]ResourceState, len(resources))
	for k, v := range resources {
		result[k] = v
	}
	return result, nil
}

// SaveResource saves a single resource state.
func (s *MemoryStore) SaveResource(instanceId string, resource ResourceState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.resources[instanceId] == nil {
		s.resources[instanceId] = make(map[string]ResourceState)
	}
	s.resources[instanceId][resource.Id] = resource
	return nil
}

// DeleteResource removes a resource from the store.
func (s *MemoryStore) DeleteResource(instanceId, resourceId string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.resources[instanceId] != nil {
		delete(s.resources[instanceId], resourceId)
	}
	return nil
}
