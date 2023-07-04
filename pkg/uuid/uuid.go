package uuid

import (
	"github.com/gofrs/uuid"
	"github.com/pkg/errors"
	"ruff.io/tio"
)

var ErrGeneratingID = errors.New("failed to generate uuid")

var _ tio.IdProvider = (*uuidProvider)(nil)

type uuidProvider struct{}

func New() tio.IdProvider {
	return &uuidProvider{}
}

func (up *uuidProvider) ID() (string, error) {
	id, err := uuid.NewV4()
	if err != nil {
		return "", errors.Wrap(ErrGeneratingID, err.Error())
	}

	return id.String(), nil
}
