package engine

import (
	"testing"

	"github.com/openziti/fablab/kernel/model"
	"github.com/openziti/fablab/kernel/store"
)

func TestReconciler_Diff_NoChanges(t *testing.T) {
	desired := createTestModel("test", 1, 1)
	current := createTestModel("test", 1, 1)

	diff := ComputeDiff(desired, current)

	if len(diff.ToCreate) != 0 {
		t.Errorf("expected 0 creates, got %d", len(diff.ToCreate))
	}
	if len(diff.ToDelete) != 0 {
		t.Errorf("expected 0 deletes, got %d", len(diff.ToDelete))
	}
	if len(diff.ToUpdate) != 0 {
		t.Errorf("expected 0 updates, got %d", len(diff.ToUpdate))
	}
}

func TestReconciler_Diff_CreateNew(t *testing.T) {
	desired := createTestModel("test", 1, 2) // 2 hosts
	current := createTestModel("test", 1, 1) // 1 host

	diff := ComputeDiff(desired, current)

	if len(diff.ToCreate) != 1 {
		t.Errorf("expected 1 create, got %d", len(diff.ToCreate))
	}
}

func TestReconciler_Diff_Delete(t *testing.T) {
	desired := createTestModel("test", 1, 1) // 1 host
	current := createTestModel("test", 1, 2) // 2 hosts

	diff := ComputeDiff(desired, current)

	if len(diff.ToDelete) != 1 {
		t.Errorf("expected 1 delete, got %d", len(diff.ToDelete))
	}
}

func TestReconciler_Apply(t *testing.T) {
	memStore := store.NewMemoryStore()
	r := NewReconciler(memStore)

	m := createTestModel("apply-test", 1, 1)
	ctx := model.NewContext(m, nil, nil)

	result, err := r.Reconcile(ctx)
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	if result.Created != 1 {
		t.Errorf("expected 1 created, got %d", result.Created)
	}
}

func TestReconciler_Idempotent(t *testing.T) {
	memStore := store.NewMemoryStore()
	r := NewReconciler(memStore)

	m := createTestModel("idempotent-test", 1, 1)
	ctx := model.NewContext(m, nil, nil)

	// First reconcile
	result1, _ := r.Reconcile(ctx)
	// Second reconcile (should be no-op)
	result2, _ := r.Reconcile(ctx)

	if result2.Created != 0 {
		t.Errorf("second reconcile should create 0, got %d", result2.Created)
	}
	if result1.Created != result2.Unchanged+result2.Created {
		t.Errorf("total resources should match")
	}
}

func createTestModel(id string, regions, hostsPerRegion int) *model.Model {
	m := &model.Model{
		Id:      id,
		Regions: make(model.Regions),
	}

	for r := 0; r < regions; r++ {
		regionId := "region-" + string(rune('a'+r))
		region := &model.Region{
			Id:    regionId,
			Hosts: make(model.Hosts),
		}
		for h := 0; h < hostsPerRegion; h++ {
			hostId := regionId + "-host-" + string(rune('0'+h))
			host := &model.Host{
				Id:         hostId,
				Components: make(model.Components),
				Region:     region,
			}
			region.Hosts[hostId] = host
		}
		m.Regions[regionId] = region
	}

	return m
}
