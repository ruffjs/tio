package thing

import (
	"context"

	"github.com/pkg/errors"
	"gorm.io/gorm"
	"ruff.io/tio/pkg/model"
	"ruff.io/tio/shadow"
)

type thingRepo struct {
	db *gorm.DB
}

func NewThingRepo(db *gorm.DB) Repo {
	return thingRepo{db: db}
}

var _ Repo = (*thingRepo)(nil)

func (t thingRepo) Create(ctx context.Context, th Thing) (Thing, error) {
	en := ToEntity(th)
	err := t.db.Transaction(func(tx *gorm.DB) error {
		// create Thing
		if er := tx.Create(&en).Error; er != nil {
			return er
		}

		// create Shadow
		defaultObj := []byte("{}")
		shd := shadow.Entity{
			ThingId:  th.Id,
			Desired:  defaultObj,
			Reported: defaultObj,
			Metadata: defaultObj,
			Tags:     defaultObj,
			Version:  1,
		}
		if err := tx.Create(&shd).Error; err != nil {
			return err
		}
		// create Shadow ConnStatus
		conn := shadow.ConnStatusEntity{ThingId: th.Id, Connected: new(bool)}
		if err := tx.Create(&conn).Error; err != nil {
			return err
		}
		return nil
	})
	return ToThing(en), err
}

func (t thingRepo) Delete(ctx context.Context, id string) error {
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

func (t thingRepo) Query(ctx context.Context, pq PageQuery) (model.PageData[Thing], error) {
	offset := pq.Offset()
	limit := pq.Limit()
	var page model.PageData[Thing]
	var total int64
	t.db.WithContext(ctx).Model(&Entity{}).Count(&total)
	if total == 0 {
		page.Content = []Thing{}
		return page, nil
	}
	page.Total = total
	t.db.WithContext(ctx).Model(&Entity{}).
		Order("created_at ASC").
		Offset(offset).
		Limit(limit).
		Find(&page.Content)
	if !pq.WithAuthValue {
		for i := range page.Content {
			page.Content[i].AuthValue = ""
		}
	}
	return page, nil
}

func (t thingRepo) Get(ctx context.Context, id string) (*Thing, error) {
	en := Entity{Id: id}
	res := t.db.First(&en)
	err := res.Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	th := ToThing(en)
	return &th, err
}

func (t thingRepo) Exist(ctx context.Context, id string) (bool, error) {
	var exists bool
	err := t.db.Model(&Entity{}).
		Select("count(*) > 0").
		Where("id = ?", id).
		Find(&exists).
		Error
	return exists, err
}
