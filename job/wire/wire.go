//go:build wireinject
// +build wireinject

package wire

import (
	"github.com/google/wire"
	"gorm.io/gorm"
	"ruff.io/tio/job"
	"ruff.io/tio/pkg/uuid"
)

func InitSvc(dbConn *gorm.DB, jc job.Center) job.MgrService {
	wire.Build(
		uuid.New,
		job.NewRepo,
		job.NewMgrService,
	)
	return nil
}
