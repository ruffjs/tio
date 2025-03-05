package thing

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	"gorm.io/gorm"
	"ruff.io/tio/pkg/model"
	"ruff.io/tio/shadow"
)

type Repo interface {
	Create(ctx context.Context, th Thing, tags shadow.TagsValue) (Thing, error)
	Update(ctx context.Context, id string, tu thingPatch) error
	UpdateBatch(ctx context.Context, ids []string, tu thingPatch) error
	Delete(ctx context.Context, id string) error
	Query(ctx context.Context, pq PageQuery) (model.PageData[Thing], error)
	Get(ctx context.Context, id string) (*Thing, error)
	GetBatch(ctx context.Context, ids []string) ([]Thing, error)
	Exist(ctx context.Context, id string) (bool, error)
}

type thingPatch struct {
	Enabled        *bool `json:"enabled"`
	AuthType       *string
	AuthValue      *string
	GatewayThingId *string
}

type thingRepo struct {
	db *gorm.DB
}

func NewThingRepo(db *gorm.DB) Repo {
	return &thingRepo{db: db}
}

var _ Repo = (*thingRepo)(nil)

func (t thingRepo) Create(ctx context.Context, th Thing, tags shadow.TagsValue) (Thing, error) {
	en := ToEntity(th)
	err := t.db.Transaction(func(tx *gorm.DB) error {
		// create Thing
		if er := tx.Create(&en).Error; er != nil {
			return er
		}

		// create Shadow
		defaultObj := []byte("{}")
		tagsJson := []byte("{}")
		if tags != nil {
			j, err := json.Marshal(tags)
			if err != nil {
				return err
			}
			tagsJson = j
		}
		shd := shadow.Entity{
			ThingId:  th.Id,
			Desired:  defaultObj,
			Reported: defaultObj,
			Metadata: defaultObj,
			Tags:     tagsJson,
			Version:  1,
		}
		if err := tx.Create(&shd).Error; err != nil {
			return err
		}
		// create Shadow ConnStatus
		conn := shadow.ConnStatusEntity{ThingId: th.Id, Connected: false}
		if err := tx.Create(&conn).Error; err != nil {
			return err
		}
		return nil
	})
	return ToThing(en), err
}

func (t *thingRepo) Update(ctx context.Context, id string, tu thingPatch) error {
	return t.UpdateBatch(ctx, []string{id}, tu)
}

func (t *thingRepo) UpdateBatch(ctx context.Context, ids []string, tu thingPatch) error {
	var u map[string]any = make(map[string]any)
	if tu.Enabled != nil {
		u["enabled"] = *tu.Enabled
	}
	if tu.GatewayThingId != nil {
		u["gateway_thing_id"] = *tu.GatewayThingId
	}
	if tu.AuthType != nil {
		u["auth_type"] = *tu.AuthType
	}
	if tu.AuthValue != nil {
		u["auth_value"] = *tu.AuthValue
	}
	if len(u) == 0 {
		return nil
	}

	res := t.db.Model(&Entity{}).Where("id IN ?", ids).Updates(u)
	return res.Error
}

func (t *thingRepo) Delete(ctx context.Context, id string) error {
	err := t.db.Transaction(func(tx *gorm.DB) error {
		// delete Thing
		if er := tx.Delete(&Entity{Id: id}).Error; er != nil {
			return er
		}
		// delete Shadow
		if er := tx.Delete(&shadow.Entity{ThingId: id}).Error; er != nil {
			return er
		}
		// delete ConnStatus
		if er := tx.Delete(&shadow.ConnStatusEntity{ThingId: id}).Error; er != nil {
			return er
		}
		return nil
	})
	return err
}

func (t *thingRepo) Query(ctx context.Context, pq PageQuery) (model.PageData[Thing], error) {
	offset := pq.Offset()
	limit := pq.Limit()
	var page model.PageData[Thing]
	var total int64

	q := t.db.WithContext(ctx).Model(&Entity{}).
		Order("created_at ASC").
		Offset(offset).
		Limit(limit)
	if pq.Enabled != nil {
		q.Where("enabled = ?", *pq.Enabled)
	}
	if pq.GatewayThingId != nil {
		q.Where("gateway_thing_id = ?", *pq.GatewayThingId)
	}
	if pq.IsGateway != nil {
		q.Where("is_gateway = ?", *pq.IsGateway)
	}

	q.Count(&total)
	if total == 0 {
		page.Content = []Thing{}
		return page, nil
	}
	page.Total = total

	q.Find(&page.Content)
	if !pq.WithAuthValue {
		for i := range page.Content {
			page.Content[i].AuthValue = ""
		}
	}
	return page, nil
}

func (t *thingRepo) Get(ctx context.Context, id string) (*Thing, error) {
	en := Entity{Id: id}
	res := t.db.First(&en)
	err := res.Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	th := ToThing(en)
	return &th, err
}

func (t *thingRepo) GetBatch(ctx context.Context, ids []string) ([]Thing, error) {
	var ens []Entity
	res := t.db.WithContext(ctx).Model(&Entity{}).Where("id IN ?", ids).Find(&ens)
	if res.Error != nil {
		return nil, res.Error
	}
	ths := make([]Thing, 0, len(ens))
	for _, e := range ens {
		ths = append(ths, ToThing(e))
	}
	return ths, nil
}

func (t *thingRepo) Exist(ctx context.Context, id string) (bool, error) {
	var exists bool
	err := t.db.Model(&Entity{}).
		Select("count(*) > 0").
		Where("id = ?", id).
		Find(&exists).
		Error
	return exists, err
}
