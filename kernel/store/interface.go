package store

import "github.com/openziti/fablab/kernel/model"

type StateStore interface {
	GetStatus(instanceId string) (*model.Label, error)
	SaveStatus(instanceId string, label *model.Label) error
	ListInstances() ([]string, error)
}
