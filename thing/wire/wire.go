//go:build wireinject
// +build wireinject

package wire

import (
	"context"
	"gorm.io/gorm"
	"ruff.io/tio/connector"
	"ruff.io/tio/thing"

	"github.com/google/wire"
	"ruff.io/tio/pkg/uuid"
	"ruff.io/tio/shadow"
)

func InitSvc(ctx context.Context, dbConn *gorm.DB, shadowSvc shadow.Service, connector connector.Connectivity) thing.Service {
	wire.Build(
		thing.NewThingRepo,
		uuid.New,
		thing.NewSvc,
	)
	return nil
}
