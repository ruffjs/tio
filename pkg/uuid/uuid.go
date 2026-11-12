package uuid

import (
	stduuid "uuid"

	"ruff.io/tio"
)

var _ tio.IdProvider = (*uuidProvider)(nil)

type uuidProvider struct{}

func New() tio.IdProvider {
	return &uuidProvider{}
}

func (up *uuidProvider) ID() (string, error) {
	return stduuid.NewV7().String(), nil
}
