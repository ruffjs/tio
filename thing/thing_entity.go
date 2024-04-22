package thing

import (
	"time"
)

type Entity struct {
	Id             string    `gorm:"primaryKey;size:64"`
	Enabled        bool      `gorm:"not null;"`
	AuthType       string    `gorm:"size=50"`
	AuthValue      string    `gorm:"size=100"`
	IsGateway      bool      `gorm:"not null; default: false"`
	GatewayThingId string    `gorm:"size:64; not null; default: ''"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime"`
	CreatedAt      time.Time `gorm:"autoCreateTime"`
}

func (t Entity) TableName() string {
	return "thing"
}

func ToEntity(th Thing) Entity {
	return Entity{
		Id:             th.Id,
		Enabled:        th.Enabled,
		AuthType:       th.AuthType,
		AuthValue:      th.AuthValue,
		IsGateway:      th.IsGateway,
		GatewayThingId: th.GatewayThingId,
	}
}

func ToThing(en Entity) Thing {
	return Thing{
		Id:             en.Id,
		IsGateway:      en.IsGateway,
		Enabled:        en.Enabled,
		AuthType:       en.AuthType,
		AuthValue:      en.AuthValue,
		GatewayThingId: en.GatewayThingId,
		UpdatedAt:      en.UpdatedAt,
		CreatedAt:      en.CreatedAt,
	}
}
