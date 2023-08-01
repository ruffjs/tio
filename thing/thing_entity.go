package thing

import (
	"time"
)

type Entity struct {
	Id        string `gorm:"primaryKey;size:64"`
	Enabled   bool
	AuthType  string    `gorm:"size=50"`
	AuthValue string    `gorm:"size=100"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

func (t Entity) TableName() string {
	return "thing"
}

func ToEntity(th Thing) Entity {
	return Entity{
		Id:        th.Id,
		Enabled:   th.Enabled,
		AuthType:  th.AuthType,
		AuthValue: th.AuthValue,
	}
}

func ToThing(en Entity) Thing {
	return Thing{
		Id:        en.Id,
		Enabled:   en.Enabled,
		AuthType:  en.AuthType,
		AuthValue: en.AuthValue,
		UpdatedAt: en.UpdatedAt,
		CreatedAt: en.CreatedAt,
	}
}
