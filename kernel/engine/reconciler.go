package engine

import (
	"github.com/openziti/fablab/kernel/model"
	"github.com/openziti/fablab/kernel/store"
	"github.com/sirupsen/logrus"
)

// Action represents the type of change to be applied.
type Action string

const (
	ActionCreate Action = "create"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
)

// ResourceChange represents a single resource that needs to be created, updated, or deleted.
type ResourceChange struct {
	Id           string
	Type         string // "host", "component"
	RegionId     string
	HostId       string
	ComponentId  string            // for component-level changes
	Action       Action            // create, update, delete
	Changes      []string          // list of changed fields for updates
	OldMetadata  map[string]string // previous state
	NewMetadata  map[string]string // desired state
}

// Diff represents the difference between desired and current state.
type Diff struct {
	ToCreate []ResourceChange
	ToUpdate []ResourceChange
	ToDelete []ResourceChange
}

// IsEmpty returns true if no changes are needed.
func (d *Diff) IsEmpty() bool {
	return len(d.ToCreate) == 0 && len(d.ToUpdate) == 0 && len(d.ToDelete) == 0
}

// Total returns the total number of changes.
func (d *Diff) Total() int {
	return len(d.ToCreate) + len(d.ToUpdate) + len(d.ToDelete)
}

// ReconcileError represents an error during reconciliation.
type ReconcileError struct {
	ResourceId string
	Action     Action
	Err        error
}

func (e ReconcileError) Error() string {
	return e.Err.Error()
}

// ReconcileResult contains the summary of reconciliation actions.
type ReconcileResult struct {
	Created   int
	Updated   int
	Deleted   int
	Unchanged int
	Errors    []ReconcileError
	DryRun    bool
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

	// Find hosts to create or update
	for id, desiredHost := range desiredHosts {
		currentHost, exists := currentHosts[id]
		if !exists {
			// Host doesn't exist - create it
			diff.ToCreate = append(diff.ToCreate, ResourceChange{
				Id:       id,
				Type:     "host",
				RegionId: desiredHost.Region.Id,
				HostId:   desiredHost.Id,
				Action:   ActionCreate,
				NewMetadata: map[string]string{
					"instanceType": desiredHost.InstanceType,
				},
			})
			// Also create all components
			for compId, comp := range desiredHost.Components {
				diff.ToCreate = append(diff.ToCreate, ResourceChange{
					Id:          id + "/" + compId,
					Type:        "component",
					RegionId:    desiredHost.Region.Id,
					HostId:      desiredHost.Id,
					ComponentId: compId,
					Action:      ActionCreate,
					NewMetadata: map[string]string{
						"componentType": comp.Type.Label(),
					},
				})
			}
		} else {
			// Host exists - check for updates
			changes := detectHostChanges(desiredHost, currentHost)
			if len(changes) > 0 {
				diff.ToUpdate = append(diff.ToUpdate, ResourceChange{
					Id:       id,
					Type:     "host",
					RegionId: desiredHost.Region.Id,
					HostId:   desiredHost.Id,
					Action:   ActionUpdate,
					Changes:  changes,
					OldMetadata: map[string]string{
						"instanceType": currentHost.InstanceType,
					},
					NewMetadata: map[string]string{
						"instanceType": desiredHost.InstanceType,
					},
				})
			}

			// Check component-level changes
			compDiff := computeComponentDiff(desiredHost, currentHost)
			diff.ToCreate = append(diff.ToCreate, compDiff.ToCreate...)
			diff.ToUpdate = append(diff.ToUpdate, compDiff.ToUpdate...)
			diff.ToDelete = append(diff.ToDelete, compDiff.ToDelete...)
		}
	}

	// Find hosts to delete
	for id, currentHost := range currentHosts {
		if _, exists := desiredHosts[id]; !exists {
			// Delete all components first
			for compId := range currentHost.Components {
				diff.ToDelete = append(diff.ToDelete, ResourceChange{
					Id:          id + "/" + compId,
					Type:        "component",
					RegionId:    currentHost.Region.Id,
					HostId:      currentHost.Id,
					ComponentId: compId,
					Action:      ActionDelete,
				})
			}
			// Then delete the host
			diff.ToDelete = append(diff.ToDelete, ResourceChange{
				Id:       id,
				Type:     "host",
				RegionId: currentHost.Region.Id,
				HostId:   currentHost.Id,
				Action:   ActionDelete,
			})
		}
	}

	return diff
}

// detectHostChanges returns a list of changed fields between desired and current host.
func detectHostChanges(desired, current *model.Host) []string {
	var changes []string
	if desired.InstanceType != current.InstanceType {
		changes = append(changes, "instanceType")
	}
	return changes
}

// computeComponentDiff computes differences at the component level.
func computeComponentDiff(desiredHost, currentHost *model.Host) *Diff {
	diff := &Diff{
		ToCreate: []ResourceChange{},
		ToUpdate: []ResourceChange{},
		ToDelete: []ResourceChange{},
	}

	desiredComps := desiredHost.Components
	currentComps := currentHost.Components
	if currentComps == nil {
		currentComps = make(model.Components)
	}

	// Find components to create or update
	for compId, desiredComp := range desiredComps {
		currentComp, exists := currentComps[compId]
		if !exists {
			diff.ToCreate = append(diff.ToCreate, ResourceChange{
				Id:          desiredHost.Id + "/" + compId,
				Type:        "component",
				RegionId:    desiredHost.Region.Id,
				HostId:      desiredHost.Id,
				ComponentId: compId,
				Action:      ActionCreate,
				NewMetadata: map[string]string{
					"componentType": desiredComp.Type.Label(),
				},
			})
		} else {
			// Check for component type change (rare but possible)
			if desiredComp.Type.Label() != currentComp.Type.Label() {
				diff.ToUpdate = append(diff.ToUpdate, ResourceChange{
					Id:          desiredHost.Id + "/" + compId,
					Type:        "component",
					RegionId:    desiredHost.Region.Id,
					HostId:      desiredHost.Id,
					ComponentId: compId,
					Action:      ActionUpdate,
					Changes:     []string{"componentType"},
					OldMetadata: map[string]string{
						"componentType": currentComp.Type.Label(),
					},
					NewMetadata: map[string]string{
						"componentType": desiredComp.Type.Label(),
					},
				})
			}
		}
	}

	// Find components to delete
	for compId := range currentComps {
		if _, exists := desiredComps[compId]; !exists {
			diff.ToDelete = append(diff.ToDelete, ResourceChange{
				Id:          currentHost.Id + "/" + compId,
				Type:        "component",
				RegionId:    currentHost.Region.Id,
				HostId:      currentHost.Id,
				ComponentId: compId,
				Action:      ActionDelete,
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

// ReconcileOptions configures reconciliation behavior.
type ReconcileOptions struct {
	DryRun         bool
	ContinueOnError bool
}

// Reconcile compares desired state with current state and applies necessary changes.
func (r *Reconciler) Reconcile(ctx *model.Context) (*ReconcileResult, error) {
	return r.ReconcileWithOptions(ctx, ReconcileOptions{})
}

// ReconcileWithOptions compares desired state with current state and applies necessary changes.
func (r *Reconciler) ReconcileWithOptions(ctx *model.Context, opts ReconcileOptions) (*ReconcileResult, error) {
	result := &ReconcileResult{
		DryRun: opts.DryRun,
		Errors: []ReconcileError{},
	}

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

	if opts.DryRun {
		// Just count what would happen
		result.Created = len(diff.ToCreate)
		result.Updated = len(diff.ToUpdate)
		result.Deleted = len(diff.ToDelete)
		result.Unchanged = countUnchanged(currentResources, diff)
		logrus.Infof("Dry-run: would create %d, update %d, delete %d resources",
			result.Created, result.Updated, result.Deleted)
		return result, nil
	}

	// Apply creates
	for _, change := range diff.ToCreate {
		resource := store.ResourceState{
			Id:     change.Id,
			Type:   change.Type,
			Status: store.StatusRunning,
			Metadata: mergeMetadata(map[string]string{
				"regionId": change.RegionId,
				"hostId":   change.HostId,
			}, change.NewMetadata),
		}
		if change.ComponentId != "" {
			resource.Metadata["componentId"] = change.ComponentId
		}
		if err := r.Store.SaveResource(instanceId, resource); err != nil {
			result.Errors = append(result.Errors, ReconcileError{
				ResourceId: change.Id,
				Action:     ActionCreate,
				Err:        err,
			})
			if !opts.ContinueOnError {
				return result, err
			}
			continue
		}
		result.Created++
		logrus.Infof("Created resource [%s] type=%s", change.Id, change.Type)
	}

	// Apply updates
	for _, change := range diff.ToUpdate {
		resource := store.ResourceState{
			Id:     change.Id,
			Type:   change.Type,
			Status: store.StatusRunning,
			Metadata: mergeMetadata(map[string]string{
				"regionId": change.RegionId,
				"hostId":   change.HostId,
			}, change.NewMetadata),
		}
		if change.ComponentId != "" {
			resource.Metadata["componentId"] = change.ComponentId
		}
		if err := r.Store.SaveResource(instanceId, resource); err != nil {
			result.Errors = append(result.Errors, ReconcileError{
				ResourceId: change.Id,
				Action:     ActionUpdate,
				Err:        err,
			})
			if !opts.ContinueOnError {
				return result, err
			}
			continue
		}
		result.Updated++
		logrus.Infof("Updated resource [%s] type=%s changes=%v", change.Id, change.Type, change.Changes)
	}

	// Apply deletes
	for _, change := range diff.ToDelete {
		if err := r.Store.DeleteResource(instanceId, change.Id); err != nil {
			result.Errors = append(result.Errors, ReconcileError{
				ResourceId: change.Id,
				Action:     ActionDelete,
				Err:        err,
			})
			if !opts.ContinueOnError {
				return result, err
			}
			continue
		}
		result.Deleted++
		logrus.Infof("Deleted resource [%s] type=%s", change.Id, change.Type)
	}

	// Count unchanged
	result.Unchanged = countUnchanged(currentResources, diff)

	return result, nil
}

// GetDiff returns the diff between desired and current state without applying.
func (r *Reconciler) GetDiff(ctx *model.Context) (*Diff, error) {
	instanceId := ctx.GetModel().Id
	currentResources, err := r.Store.GetResources(instanceId)
	if err != nil {
		currentResources = make(map[string]store.ResourceState)
	}

	currentModel := buildModelFromResources(currentResources)
	return ComputeDiff(ctx.GetModel(), currentModel), nil
}

// countUnchanged counts resources that weren't modified.
func countUnchanged(currentResources map[string]store.ResourceState, diff *Diff) int {
	changedIds := make(map[string]bool)
	for _, c := range diff.ToCreate {
		changedIds[c.Id] = true
	}
	for _, c := range diff.ToUpdate {
		changedIds[c.Id] = true
	}
	for _, c := range diff.ToDelete {
		changedIds[c.Id] = true
	}

	unchanged := 0
	for id := range currentResources {
		if !changedIds[id] {
			unchanged++
		}
	}
	return unchanged
}

// mergeMetadata merges two metadata maps.
func mergeMetadata(base, override map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range base {
		result[k] = v
	}
	for k, v := range override {
		if v != "" {
			result[k] = v
		}
	}
	return result
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
