package loader

import (
	"os"

	"github.com/openziti/fablab/kernel/model"
	"gopkg.in/yaml.v2"
)

type FablabYaml struct {
	Regions map[string]RegionYaml `yaml:"regions"`
}

type RegionYaml struct {
	Hosts map[string]HostYaml `yaml:"hosts"`
}

type HostYaml struct {
	Components []ComponentYaml `yaml:"components"`
}

type ComponentYaml struct {
	Type   string                 `yaml:"type"`
	Config map[string]interface{} `yaml:",inline"`
}

func LoadModel(path string) (*model.Model, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config FablabYaml
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	m := &model.Model{}
	// Logic to populate m.Regions, m.Hosts, etc.
	// This requires deeper integration with Model's internal structure which is pointer-heavy.
	// For now, this is a skeleton implementation.

	return m, nil
}
