package test

import (
	"gorm.io/gorm"
	"ruff.io/tio/db/mock"
	"ruff.io/tio/job"
	"ruff.io/tio/job/wire"
	"ruff.io/tio/pkg/log"
)

func NewTestSvc(jc job.Center) (job.MgrService, job.Repo) {
	db := mock.NewSqliteConnTest()
	return NewTestSvcWithDB(db, jc)
}

func NewTestSvcWithDB(db *gorm.DB, jc job.Center) (job.MgrService, job.Repo) {
	err := db.AutoMigrate(job.Entity{}, job.TaskEntity{})
	if err != nil {
		log.Fatalf("job auto migrate error: %v", err)
	}
	r := job.NewRepo(db)

	s := wire.InitSvc(db, jc)
	return s, r
}
