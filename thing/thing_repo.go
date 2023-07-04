package thing

import (
	"context"

	"github.com/pkg/errors"
	"gorm.io/gorm"
	"ruff.io/tio/pkg/model"
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
	res := t.db.Create(&en)
	if res.Error != nil {
		return Thing{}, res.Error
	}
	return ToThing(en), nil
}

func (t thingRepo) Delete(ctx context.Context, id string) error {
	res := t.db.Delete(&Entity{Id: id})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return model.ErrNotFound
	}
	return nil
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
