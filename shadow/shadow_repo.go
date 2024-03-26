package shadow

import (
	"context"
	"time"

	"ruff.io/tio/connector"

	"ruff.io/tio/pkg/log"
	"ruff.io/tio/pkg/model"

	"github.com/pkg/errors"
	"gorm.io/gorm"
)

type shadowRepo struct {
	db *gorm.DB
}

type EntityWithEnable struct {
	Entity
	Enabled bool
}

func (r shadowRepo) ExecWithTx(f func(txtRepo Repo) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		txRepo := NewShadowRepo(tx)
		return f(txRepo)
	})
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

	// update when version match
	ex := r.db
	if version > 0 {
		ex = ex.Where("version = ?", version)
	}
	res := ex.Save(&en)
	if res.Error != nil {
		return nil, errors.Wrap(res.Error, "update in db")
	}
	if res.RowsAffected != 1 {
		log.Errorf("Update shadow %s got unexpected affected row %d", thingId, res.RowsAffected)
		return nil, errors.Wrap(model.ErrVersionConflict, "")
	}
	n, err := toShadow(en)
	if err != nil {
		return nil, errors.Wrap(err, "convert entity to shadow")
	}

	return &n, nil
}

// UpdateConnStatus batch update in a transaction
func (r shadowRepo) UpdateConnStatus(ctx context.Context, s []connector.ClientInfo) error {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		for _, c := range s {
			ex := tx.Model(&ConnStatusEntity{ThingId: c.ClientId})
			ts := c.ConnectedAt
			if c.DisconnectedAt != nil && (c.ConnectedAt == nil || c.ConnectedAt.Before(*c.DisconnectedAt)) {
				ts = c.DisconnectedAt
			}
			// This update cannot cover newer data than it.
			if ts != nil {
				ex.Where("(connected_at IS NULL OR connected_at <= ?) "+
					"AND (disconnected_at IS NULL OR disconnected_at <= ?)",
					ts, ts,
				)
			}
			var up map[string]any
			if c.Connected {
				up = map[string]any{
					"connected":    1,
					"connected_at": c.ConnectedAt,
					"remote_addr":  c.RemoteAddr,
				}
			} else {
				up = map[string]any{
					"connected":         0,
					"disconnected_at":   c.DisconnectedAt,
					"disconnect_reason": c.DisconnectReason,
					"remote_addr":       c.RemoteAddr,
				}
			}
			if err := ex.Updates(up).Error; err != nil {
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

func (r shadowRepo) Get(ctx context.Context, thingId string) (*ShadowWithEnable, error) {
	e := EntityWithEnable{}
	res := r.db.Model(&Entity{}).
		Select("t.enabled", "shadow.*").
		Joins("LEFT JOIN thing t ON t.id=shadow.thing_id").
		Where("shadow.thing_id=?", thingId).
		First(&e)

	err := res.Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	s, err := toShadow(e.Entity)
	se := ShadowWithEnable{Enabled: e.Enabled, Shadow: s}

	return &se, err
}

func (r shadowRepo) GetVersion(ctx context.Context, thingId string) (version int64, err error) {
	err = r.db.Select("version").Where("thing_id = ?", thingId).First(&version).Error
	return
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

func (r shadowRepo) Query(ctx context.Context, pq model.PageQuery, q ParsedQuerySql) (model.PageData[ShadowWithStatus], error) {
	offset := pq.Offset()
	limit := pq.Limit()
	var page model.PageData[ShadowWithStatus]
	var total int64

	db := r.db.WithContext(ctx).
		Model(&Entity{}).
		Select("t.enabled", "shadow.*").
		Joins("ConnStatus").
		Preload("ConnStatus").
		Joins("INNER JOIN thing t ON t.id=shadow.thing_id")
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

	results := make([]EntityWithEnable, 0)
	res = db.Offset(offset).
		Limit(limit).
		Find(&results)
	if res.Error != nil {
		return page, res.Error
	}

	l, err := toShadowWithStatus(results)
	page.Content = l

	return page, err
}

func toShadowWithStatus(list []EntityWithEnable) ([]ShadowWithStatus, error) {
	res := make([]ShadowWithStatus, len(list))
	for i, v := range list {
		ss := ShadowWithStatus{}
		if s, err := toShadow(v.Entity); err != nil {
			return res, errors.WithMessage(err, "entity toShadow")
		} else {
			ss.Shadow = s
		}
		ss.Enabled = v.Enabled
		cs := v.ConnStatus
		ss.Connected = &cs.Connected
		ss.ConnectedAt = cs.ConnectedAt
		ss.DisconnectedAt = cs.DisconnectedAt
		ss.RemoteAddr = cs.RemoteAddr
		res[i] = ss
	}
	return res, nil
}

var _ Repo = (*shadowRepo)(nil)

func NewShadowRepo(db *gorm.DB) Repo {
	return shadowRepo{db}
}
