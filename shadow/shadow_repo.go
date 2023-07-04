package shadow

import (
	"context"
	"fmt"
	"strings"

	"ruff.io/tio/pkg/log"
	"ruff.io/tio/pkg/model"

	"github.com/pkg/errors"
	"gorm.io/gorm"
)

type shadowRepo struct {
	db *gorm.DB
}

type Where struct {
	Field []string
	value []string
}

func (w *Where) Expr() (string, []string) {
	expr := strings.Join(w.Field, "AND")
	return expr, w.value
}

func (r shadowRepo) Create(ctx context.Context, thingId string, s Shadow) (*Shadow, error) {
	en, err := toEntity(s)
	if err != nil {
		return nil, err
	}
	re := r.db.Create(&en)
	if re.Error != nil {
		return nil, errors.Wrap(re.Error, "create shadow "+thingId)
	}
	ss, err := toShadow(en)
	if err != nil {
		return nil, err
	}
	return &ss, nil
}

func (r shadowRepo) Update(ctx context.Context, thingId string, version int64, s Shadow) (*Shadow, error) {
	en, err := toEntity(s)
	if err != nil {
		return nil, err
	}

	err = r.db.Transaction(func(tx *gorm.DB) error {
		oldEn := Entity{ThingId: thingId}
		res := tx.First(&oldEn)
		if res.Error != nil {
			return errors.Wrap(res.Error, "find shadow "+thingId)
		}
		if version > 0 && oldEn.Version != version {
			return errors.Wrap(model.ErrConflict,
				fmt.Sprintf("expect version %d but got %d", oldEn.Version, version))
		}
		// update when version match
		res = tx.Save(&en)
		if version > 0 {
			res.Where("version = ?", version)
		}
		if res.Error != nil {
			return errors.Wrap(res.Error, "update in db")
		}
		if res.RowsAffected != 1 {
			log.Errorf("Update shadow %s got unexpected affected row %d", thingId, res.RowsAffected)
			return errors.Wrap(model.ErrConflict, "")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	n, err := toShadow(en)
	if err != nil {
		return nil, errors.Wrap(err, "convert entity to shadow")
	}

	return &n, nil
}

func (r shadowRepo) Get(ctx context.Context, thingId string) (*Shadow, error) {
	en := Entity{ThingId: thingId}
	res := r.db.First(&en)
	err := res.Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	s, err := toShadow(en)
	if err != nil {
		return nil, err
	}
	return &s, err
}

func (r shadowRepo) Delete(ctx context.Context, thingId string) error {
	res := r.db.Delete(&Entity{ThingId: thingId})
	err := res.Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if res.RowsAffected == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (r shadowRepo) Query(ctx context.Context, pq model.PageQuery, q ParsedQuerySql) (model.PageData[Entity], error) {
	offset := pq.Offset()
	limit := pq.Limit()
	var page model.PageData[Entity]
	var total int64

	db := r.db.WithContext(ctx).Model(&Entity{})
	if q.Where != "" {
		db.Where(q.Where)
	}

	res := db.Count(&total)
	if res.Error != nil {
		return page, res.Error
	}
	if total == 0 {
		return page, nil
	}
	page.Total = total

	if q.OrderBy != "" {
		db.Order(q.OrderBy)
	}

	results := make([]Entity, 0)
	res = db.Offset(offset).
		Limit(limit).
		Find(&results)
	if res.Error != nil {
		return page, res.Error
	}
	page.Content = results

	return page, nil
}

var _ Repo = (*shadowRepo)(nil)

func NewShadowRepo(db *gorm.DB) Repo {
	return shadowRepo{db}
}
