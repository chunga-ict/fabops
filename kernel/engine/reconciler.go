package engine

import (
	"github.com/openziti/fablab/kernel/model"
	"github.com/openziti/fablab/kernel/store"
	"github.com/sirupsen/logrus"
)

// ResourceChange represents a single resource that needs to be created, updated, or deleted.
type ResourceChange struct {
	Id       string
	Type     string // "host", "component"
	RegionId string
	HostId   string
}

// Diff represents the difference between desired and current state.
type Diff struct {
	ToCreate []ResourceChange
	ToUpdate []ResourceChange
	ToDelete []ResourceChange
}

// ReconcileResult contains the summary of reconciliation actions.
type ReconcileResult struct {
	Created   int
	Updated   int
	Deleted   int
	Unchanged int
}

// Reconciler manages the reconciliation between desired and current infrastructure state.
type Reconciler struct {
	Store store.ResourceStore
}

// NewReconciler creates a new Reconciler with the given store.
func NewReconciler(s store.ResourceStore) *Reconciler {
	return &Reconciler{Store: s}
}

// ComputeDiff calculates the difference between desired and current model states.
func ComputeDiff(desired, current *model.Model) *Diff {
	diff := &Diff{
		ToCreate: []ResourceChange{},
		ToUpdate: []ResourceChange{},
		ToDelete: []ResourceChange{},
	}

	desiredHosts := collectHosts(desired)
	currentHosts := collectHosts(current)

	// Find hosts to create (in desired but not in current)
	for id, host := range desiredHosts {
		if _, exists := currentHosts[id]; !exists {
			diff.ToCreate = append(diff.ToCreate, ResourceChange{
				Id:       id,
				Type:     "host",
				RegionId: host.Region.Id,
				HostId:   host.Id,
			})
		}
	}

	// Find hosts to delete (in current but not in desired)
	for id, host := range currentHosts {
		if _, exists := desiredHosts[id]; !exists {
			diff.ToDelete = append(diff.ToDelete, ResourceChange{
				Id:       id,
				Type:     "host",
				RegionId: host.Region.Id,
				HostId:   host.Id,
			})
		}
	}

	return diff
}

// collectHosts returns a map of all hosts in the model keyed by their Id.
func collectHosts(m *model.Model) map[string]*model.Host {
	hosts := make(map[string]*model.Host)
	if m == nil || m.Regions == nil {
		return hosts
	}
	for _, region := range m.Regions {
		for _, host := range region.Hosts {
			hosts[host.Id] = host
		}
	}
	return hosts
}

// Reconcile compares desired state with current state and applies necessary changes.
func (r *Reconciler) Reconcile(ctx *model.Context) (*ReconcileResult, error) {
	result := &ReconcileResult{}

	instanceId := ctx.GetModel().Id
	currentResources, err := r.Store.GetResources(instanceId)
	if err != nil {
		logrus.Warnf("Unable to load resources for instance [%s]: %v. Assuming fresh start.", instanceId, err)
		currentResources = make(map[string]store.ResourceState)
	}

	// Build current model from stored resources
	currentModel := buildModelFromResources(currentResources)

	// Compute diff
	diff := ComputeDiff(ctx.GetModel(), currentModel)

	// Apply creates
	for _, change := range diff.ToCreate {
		resource := store.ResourceState{
			Id:     change.Id,
			Type:   change.Type,
			Status: "running",
			Metadata: map[string]string{
				"regionId": change.RegionId,
			},
		}
		if err := r.Store.SaveResource(instanceId, resource); err != nil {
			return result, err
		}
		result.Created++
		logrus.Infof("Created resource [%s] type=%s", change.Id, change.Type)
	}

	// Apply deletes
	for _, change := range diff.ToDelete {
		if err := r.Store.DeleteResource(instanceId, change.Id); err != nil {
			return result, err
		}
		result.Deleted++
		logrus.Infof("Deleted resource [%s] type=%s", change.Id, change.Type)
	}

	// Count unchanged
	result.Unchanged = len(currentResources) - result.Deleted

	return result, nil
}

// buildModelFromResources reconstructs a Model from stored ResourceState entries.
func buildModelFromResources(resources map[string]store.ResourceState) *model.Model {
	m := &model.Model{
		Regions: make(model.Regions),
	}

	for _, res := range resources {
		if res.Type != "host" {
			continue
		}

		regionId := res.Metadata["regionId"]
		if regionId == "" {
			regionId = "default"
		}

		region, exists := m.Regions[regionId]
		if !exists {
			region = &model.Region{
				Id:    regionId,
				Hosts: make(model.Hosts),
			}
			m.Regions[regionId] = region
		}

		host := &model.Host{
			Id:     res.Id,
			Region: region,
		}
		region.Hosts[res.Id] = host
	}

	return m
}
