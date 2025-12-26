package store

import (
	"fmt"

	"github.com/openziti/fablab/kernel/model"
)

type FileStore struct {
	Config *model.FablabConfig
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
	// Ensure the label knows its instance ID and model ID if missing
	if label.InstanceId == "" {
		label.InstanceId = instanceId
	}
	// Utilize SaveAtPath which handles directory creation
	return label.SaveAtPath(instanceCfg.WorkingDirectory)
}

func (s *FileStore) ListInstances() ([]string, error) {
	keys := make([]string, 0, len(s.Config.Instances))
	for k := range s.Config.Instances {
		keys = append(keys, k)
	}
	return keys, nil
}
