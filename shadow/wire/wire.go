//go:build wireinject
// +build wireinject

package wire

import (
	"gorm.io/gorm"
	"ruff.io/tio/shadow"

	"github.com/google/wire"
)

func InitSvc(dbConn *gorm.DB, conn connector.Connectivity) shadow.Service {
	wire.Build(
		shadow.NewSvc,
		shadow.NewShadowRepo,
	)
	return nil
}
