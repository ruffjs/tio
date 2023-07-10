package shadow

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	err = r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&en).Error; err != nil {
			return err
		}
		conn := ConnStatusEntity{ThingId: thingId, Connected: false}
		if err := tx.Create(&conn).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "create shadow")
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

// UpdateConnStatus batch update in a transaction
func (r shadowRepo) UpdateConnStatus(ctx context.Context, s []ClientInfo) error {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		for _, c := range s {
			en := ConnStatusEntity{
				Connected:        c.Connected,
				ConnectedAt:      c.ConnectedAt,
				DisconnectedAt:   c.DisconnectedAt,
				DisconnectReason: c.DisconnectReason,
				RemoteAddr:       c.RemoteAddr,
			}
			ex := tx.Model(&ConnStatusEntity{ThingId: c.ClientId})
			ts := en.ConnectedAt
			if en.DisconnectedAt != nil && (en.ConnectedAt == nil || en.ConnectedAt.Before(*en.DisconnectedAt)) {
				ts = en.DisconnectedAt
			}
			// This update cannot cover newer data than it.
			if ts != nil {
				ex.Where("(connected_at IS NULL OR connected_at <= ?) "+
					"AND (disconnected_at IS NULL OR disconnected_at <= ?)",
					ts, ts,
				)
			}
			if err := ex.Updates(&en).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

func (r shadowRepo) UpdateAllConnStatusDisconnect(ctx context.Context, updateTimeBefore time.Time) error {
	t := time.Now()
	res := r.db.Model(&ConnStatusEntity{}).
		Where("connected=1 AND updated_at < ?", updateTimeBefore).
		Updates(map[string]any{"connected": 0, "disconnected_at": &t, "disconnect_reason": "system"})
	return res.Error
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
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&Entity{ThingId: thingId}).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		if err := tx.Delete(&ConnStatusEntity{ThingId: thingId}).Error; err != nil {
			return err
		}
		return nil
	})

	return errors.Wrap(err, "delete shadow "+thingId)
}

func (r shadowRepo) Query(ctx context.Context, pq model.PageQuery, q ParsedQuerySql) (model.PageData[Entity], error) {
	offset := pq.Offset()
	limit := pq.Limit()
	var page model.PageData[Entity]
	var total int64

	db := r.db.WithContext(ctx).
		Model(&Entity{}).
		Joins("ConnStatus").
		Preload("ConnStatus")
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
