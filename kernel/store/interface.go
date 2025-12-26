package store

import "github.com/openziti/fablab/kernel/model"

// StateStore manages the persistent state of fablab instances.
type StateStore interface {
	GetStatus(instanceId string) (*model.Label, error)
	SaveStatus(instanceId string, label *model.Label) error
	ListInstances() ([]string, error)
}

// ResourceStore extends StateStore with resource-level tracking.
type ResourceStore interface {
	StateStore
	GetResources(instanceId string) (map[string]ResourceState, error)
	SaveResource(instanceId string, resource ResourceState) error
	DeleteResource(instanceId, resourceId string) error
}

// ResourceStatus represents the lifecycle status of a resource.
type ResourceStatus string

const (
	StatusPending  ResourceStatus = "pending"
	StatusCreating ResourceStatus = "creating"
	StatusRunning  ResourceStatus = "running"
	StatusUpdating ResourceStatus = "updating"
	StatusDeleting ResourceStatus = "deleting"
	StatusDeleted  ResourceStatus = "deleted"
	StatusError    ResourceStatus = "error"
)

// ResourceState represents the state of a single resource.
type ResourceState struct {
	Id        string
	Type      string         // "host", "component"
	Status    ResourceStatus // lifecycle status
	Metadata  map[string]string
	CreatedAt int64 // Unix timestamp
	UpdatedAt int64 // Unix timestamp
}
