package loader

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/openziti/fablab/kernel/model"
	"gopkg.in/yaml.v2"
)

// ValidationError represents a validation issue.
type ValidationError struct {
	Path    string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Path, e.Message)
}

// ValidationResult contains all validation errors.
type ValidationResult struct {
	Errors   []ValidationError
	Warnings []ValidationError
}

func (r *ValidationResult) IsValid() bool {
	return len(r.Errors) == 0
}

func (r *ValidationResult) AddError(path, message string) {
	r.Errors = append(r.Errors, ValidationError{Path: path, Message: message})
}

func (r *ValidationResult) AddWarning(path, message string) {
	r.Warnings = append(r.Warnings, ValidationError{Path: path, Message: message})
}

// Valid ID pattern: alphanumeric, hyphens, underscores
var validIdPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

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

// ValidateConfig validates the YAML configuration without building the model.
func ValidateConfig(path string) (*ValidationResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return ValidateConfigBytes(data)
}

// ValidateConfigBytes validates YAML bytes.
func ValidateConfigBytes(data []byte) (*ValidationResult, error) {
	var config FablabYaml
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}
	return validateConfig(&config), nil
}

func validateConfig(config *FablabYaml) *ValidationResult {
	result := &ValidationResult{
		Errors:   []ValidationError{},
		Warnings: []ValidationError{},
	}

	// Validate model section
	validateModel(config, result)

	// Validate regions
	validateRegions(config, result)

	return result
}

func validateModel(config *FablabYaml, result *ValidationResult) {
	if config.Model.Id == "" {
		result.AddError("model.id", "model id is required")
	} else if !validIdPattern.MatchString(config.Model.Id) {
		result.AddError("model.id", "invalid id format: must start with letter and contain only alphanumeric, hyphens, underscores")
	}
}

func validateRegions(config *FablabYaml, result *ValidationResult) {
	if len(config.Regions) == 0 {
		result.AddWarning("regions", "no regions defined")
		return
	}

	seenRegionIds := make(map[string]bool)
	for regionId, region := range config.Regions {
		path := fmt.Sprintf("regions.%s", regionId)

		// Check for duplicate region IDs (shouldn't happen with map, but validate anyway)
		if seenRegionIds[regionId] {
			result.AddError(path, "duplicate region id")
		}
		seenRegionIds[regionId] = true

		// Validate region ID format
		if !validIdPattern.MatchString(regionId) {
			result.AddError(path, "invalid region id format")
		}

		// Validate hosts
		validateHosts(&region, path, result)
	}
}

func validateHosts(region *RegionYaml, basePath string, result *ValidationResult) {
	if len(region.Hosts) == 0 {
		result.AddWarning(basePath+".hosts", "no hosts defined in region")
		return
	}

	seenHostIds := make(map[string]bool)
	for hostId, host := range region.Hosts {
		path := fmt.Sprintf("%s.hosts.%s", basePath, hostId)

		// Check for duplicate host IDs within the region
		if seenHostIds[hostId] {
			result.AddError(path, "duplicate host id within region")
		}
		seenHostIds[hostId] = true

		// Validate host ID format
		if !validIdPattern.MatchString(hostId) {
			result.AddError(path, "invalid host id format")
		}

		// Validate components
		validateComponents(&host, path, result)
	}
}

func validateComponents(host *HostYaml, basePath string, result *ValidationResult) {
	seenCompIds := make(map[string]bool)

	for i, comp := range host.Components {
		path := fmt.Sprintf("%s.components[%d]", basePath, i)

		// Validate component type is registered
		if comp.Type == "" {
			result.AddError(path+".type", "component type is required")
		} else {
			if _, err := model.GetComponentType(comp.Type); err != nil {
				validTypes := model.ListComponentTypes()
				result.AddError(path+".type", fmt.Sprintf("unknown component type '%s'. Valid types: %s",
					comp.Type, strings.Join(validTypes, ", ")))
			}
		}

		// Validate component ID if specified
		if comp.Id != "" {
			if !validIdPattern.MatchString(comp.Id) {
				result.AddError(path+".id", "invalid component id format")
			}
			if seenCompIds[comp.Id] {
				result.AddError(path+".id", "duplicate component id within host")
			}
			seenCompIds[comp.Id] = true
		}
	}
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
		host.Region = region // Set parent reference
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
