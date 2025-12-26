package loader

import (
	"fmt"
	"os"

	"github.com/openziti/fablab/kernel/model"
	"gopkg.in/yaml.v2"
)

// FablabYaml is the root YAML structure
type FablabYaml struct {
	Model   ModelYaml            `yaml:"model"`
	Regions map[string]RegionYaml `yaml:"regions"`
}

// ModelYaml contains model-level configuration
type ModelYaml struct {
	Id string `yaml:"id"`
}

// RegionYaml represents a deployment region
type RegionYaml struct {
	Site  string              `yaml:"site"`
	Hosts map[string]HostYaml `yaml:"hosts"`
}

// HostYaml represents a host/VM configuration
type HostYaml struct {
	InstanceType string          `yaml:"instanceType"`
	Components   []ComponentYaml `yaml:"components"`
}

// ComponentYaml represents a component configuration
type ComponentYaml struct {
	Type   string `yaml:"type"`
	Id     string `yaml:"id"`
}

// LoadModel creates a Model from a YAML configuration file
func LoadModel(path string) (*model.Model, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var config FablabYaml
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return buildModel(&config)
}

// LoadModelFromBytes creates a Model from YAML bytes
func LoadModelFromBytes(data []byte) (*model.Model, error) {
	var config FablabYaml
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return buildModel(&config)
}

func buildModel(config *FablabYaml) (*model.Model, error) {
	m := &model.Model{
		Id:      config.Model.Id,
		Regions: make(model.Regions),
	}

	for regionId, regionYaml := range config.Regions {
		region, err := buildRegion(regionId, &regionYaml)
		if err != nil {
			return nil, fmt.Errorf("region '%s': %w", regionId, err)
		}
		m.Regions[regionId] = region
	}

	return m, nil
}

func buildRegion(id string, config *RegionYaml) (*model.Region, error) {
	region := &model.Region{
		Id:    id,
		Site:  config.Site,
		Hosts: make(model.Hosts),
	}

	for hostId, hostYaml := range config.Hosts {
		host, err := buildHost(hostId, &hostYaml)
		if err != nil {
			return nil, fmt.Errorf("host '%s': %w", hostId, err)
		}
		region.Hosts[hostId] = host
	}

	return region, nil
}

func buildHost(id string, config *HostYaml) (*model.Host, error) {
	host := &model.Host{
		Id:           id,
		InstanceType: config.InstanceType,
		Components:   make(model.Components),
	}

	for i, compYaml := range config.Components {
		comp, err := buildComponent(i, &compYaml)
		if err != nil {
			return nil, fmt.Errorf("component[%d]: %w", i, err)
		}
		host.Components[comp.Id] = comp
	}

	return host, nil
}

func buildComponent(index int, config *ComponentYaml) (*model.Component, error) {
	// Get component type from registry
	compType, err := model.GetComponentType(config.Type)
	if err != nil {
		return nil, fmt.Errorf("unknown component type '%s'", config.Type)
	}

	// Generate component ID if not specified
	compId := config.Id
	if compId == "" {
		compId = fmt.Sprintf("%s-%d", config.Type, index)
	}

	return &model.Component{
		Id:   compId,
		Type: compType,
	}, nil
}
