package engine

import (
	"github.com/openziti/fablab/kernel/model"
	"github.com/openziti/fablab/kernel/store"
	"github.com/sirupsen/logrus"
)

type Reconciler struct {
	Store store.StateStore
}

func NewReconciler(s store.StateStore) *Reconciler {
	return &Reconciler{Store: s}
}

func (r *Reconciler) Reconcile(ctx *model.Context) error {
	// 1. Load Current State
	instanceId := ctx.Config.GetSelectedInstanceId()
	currentState, err := r.Store.GetStatus(instanceId)
	if err != nil {
		logrus.Warnf("Unable to load state for instance [%s]: %v. Assuming fresh start.", instanceId, err)
		// Logic for fresh start or error handling
	} else {
		logrus.Infof("Current state for instance [%s]: %s", instanceId, currentState.State)
	}

	// 2. Discovery / Compare
	// This would involve iterating over ctx.Model.Regions/Hosts and checking against currentState.Bindings/Infrastructure

	// 3. Convergence
	// This would trigger Terraform apply or Component actions

	return nil
}
