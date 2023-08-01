// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package wire

import (
	"gorm.io/gorm"
	"ruff.io/tio/connector"
	"ruff.io/tio/shadow"
)

// Injectors from wire.go:

func InitSvc(dbConn *gorm.DB, conn connector.Connectivity) shadow.Service {
	repo := shadow.NewShadowRepo(dbConn)
	service := shadow.NewSvc(repo, conn)
	return service
}
