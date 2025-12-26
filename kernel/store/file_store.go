package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/openziti/fablab/kernel/model"
)

type FileStore struct {
	Config *model.FablabConfig
	mu     sync.RWMutex
}

func NewFileStore(cfg *model.FablabConfig) *FileStore {
	return &FileStore{Config: cfg}
}

func (s *FileStore) GetStatus(instanceId string) (*model.Label, error) {
	instanceCfg, ok := s.Config.Instances[instanceId]
	if !ok {
		return nil, fmt.Errorf("instance [%s] not found in config", instanceId)
	}
	return model.LoadLabel(instanceCfg.WorkingDirectory)
}

func (s *FileStore) SaveStatus(instanceId string, label *model.Label) error {
	instanceCfg, ok := s.Config.Instances[instanceId]
	if !ok {
		return fmt.Errorf("instance [%s] not found in config", instanceId)
	}
	if label.InstanceId == "" {
		label.InstanceId = instanceId
	}
	return label.SaveAtPath(instanceCfg.WorkingDirectory)
}

func (s *FileStore) ListInstances() ([]string, error) {
	keys := make([]string, 0, len(s.Config.Instances))
	for k := range s.Config.Instances {
		keys = append(keys, k)
	}
	return keys, nil
}

// GetResources returns all resources for an instance from file.
func (s *FileStore) GetResources(instanceId string) (map[string]ResourceState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resourcesPath := s.resourcesPath(instanceId)
	data, err := os.ReadFile(resourcesPath)
	if os.IsNotExist(err) {
		return make(map[string]ResourceState), nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read resources: %w", err)
	}

	var resources map[string]ResourceState
	if err := json.Unmarshal(data, &resources); err != nil {
		return nil, fmt.Errorf("failed to parse resources: %w", err)
	}

	return resources, nil
}

// SaveResource saves a single resource state to file.
func (s *FileStore) SaveResource(instanceId string, resource ResourceState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	resources, err := s.getResourcesUnsafe(instanceId)
	if err != nil {
		return err
	}

	resources[resource.Id] = resource
	return s.saveResourcesUnsafe(instanceId, resources)
}

// DeleteResource removes a resource from the store.
func (s *FileStore) DeleteResource(instanceId, resourceId string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	resources, err := s.getResourcesUnsafe(instanceId)
	if err != nil {
		return err
	}

	delete(resources, resourceId)
	return s.saveResourcesUnsafe(instanceId, resources)
}

func (s *FileStore) resourcesPath(instanceId string) string {
	instanceCfg, ok := s.Config.Instances[instanceId]
	if !ok {
		return filepath.Join(os.TempDir(), "fablab", instanceId, "resources.json")
	}
	return filepath.Join(instanceCfg.WorkingDirectory, "resources.json")
}

func (s *FileStore) getResourcesUnsafe(instanceId string) (map[string]ResourceState, error) {
	resourcesPath := s.resourcesPath(instanceId)
	data, err := os.ReadFile(resourcesPath)
	if os.IsNotExist(err) {
		return make(map[string]ResourceState), nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read resources: %w", err)
	}

	var resources map[string]ResourceState
	if err := json.Unmarshal(data, &resources); err != nil {
		return nil, fmt.Errorf("failed to parse resources: %w", err)
	}

	return resources, nil
}

func (s *FileStore) saveResourcesUnsafe(instanceId string, resources map[string]ResourceState) error {
	resourcesPath := s.resourcesPath(instanceId)

	if err := os.MkdirAll(filepath.Dir(resourcesPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(resources, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal resources: %w", err)
	}

	if err := os.WriteFile(resourcesPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write resources: %w", err)
	}

	return nil
}
