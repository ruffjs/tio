package job

import (
	"context"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"ruff.io/tio/pkg/model"
	"strings"
)

type jobRepo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) Repo {
	return jobRepo{db}
}

func (r jobRepo) CreateJob(ctx context.Context, j Entity) (Entity, error) {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var oldId = ""
		if res := tx.Model(Entity{}).
			Where("job_id=?", j.JobId).
			Select("job_id").Find(&oldId); res.Error != nil {
			return res.Error
		} else {
			if res.RowsAffected > 0 {
				return errors.WithMessagef(model.ErrDuplicated, "job "+j.JobId)
			}
		}
		if res := tx.WithContext(ctx).Create(&j); res.Error != nil {
			return res.Error
		} else {
			return nil
		}
	})
	if err != nil {
		return Entity{}, err
	}
	e, err := r.GetJob(ctx, j.JobId)
	if err != nil {
		return Entity{}, err
	}
	return e, nil
}

func (r jobRepo) UpdateJob(ctx context.Context, m map[string]any) error {
	for k := range m {
		lk := strings.ToLower(k)
		if lk == "jobid" || lk == "job_id" {
			return errors.New("can't update jobId")
		}
	}
	if err := r.db.WithContext(ctx).Model(Entity{}).Updates(m).Error; err != nil {
		return err
	}
	return nil
}

func (r jobRepo) DeleteJob(ctx context.Context, id string, force bool) error {
	if force {
		if err := r.db.WithContext(ctx).Delete(Entity{JobId: id}).Error; err != nil {
			return err
		}
	} else {
		if err := r.db.WithContext(ctx).
			Where("status != ?", StatusInProgress).
			Delete(Entity{JobId: id}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r jobRepo) GetJob(ctx context.Context, id string) (Entity, error) {
	e := Entity{JobId: id}
	if err := r.db.WithContext(ctx).First(&e).Error; err != nil {
		return Entity{}, err
	} else {
		return e, nil
	}
}

func (r jobRepo) QueryJob(ctx context.Context, pq PageQuery) (model.PageData[Entity], error) {
	offset := pq.Offset()
	limit := pq.Limit()
	var page model.PageData[Entity]
	var total int64
	r.db.WithContext(ctx).Model(&Entity{}).Count(&total)
	if total == 0 {
		page.Content = []Entity{}
		return page, nil
	}
	page.Total = total
	q := r.db.WithContext(ctx).Model(&Entity{}).
		Order("created_at ASC").
		Offset(offset).
		Limit(limit)
	if pq.Status != "" {
		q.Where("status=?", pq.Status)
	}
	if pq.Operation != "" {
		q.Where("operation=?", pq.Operation)
	}
	if err := q.Find(&page.Content).Error; err != nil {
		return page, err
	}
	return page, nil
}

func (r jobRepo) CreateTask(ctx context.Context, t TaskEntity) (TaskEntity, error) {
	if res := r.db.WithContext(ctx).Create(&t); res.Error != nil {
		return TaskEntity{}, res.Error
	} else {
		if e, err := r.GetTask(ctx, t.TaskId); err != nil {
			return TaskEntity{}, err
		} else {
			return e, nil
		}
	}
}

func (r jobRepo) UpdateTask(ctx context.Context, taskId int64, m map[string]any) error {
	for k := range m {
		lk := strings.ToLower(k)
		if lk == "taskid" || lk == "task_id" || lk == "jobid" || lk == "job_id" ||
			lk == "thingid" || lk == "thing_id" {
			return errors.New("can't update taskId, jobId, thingId")
		}
	}
	if err := r.db.WithContext(ctx).Model(TaskEntity{TaskId: taskId}).Updates(m).Error; err != nil {
		return err
	}
	return nil
}

func (r jobRepo) DeleteTask(ctx context.Context, taskId int64) error {
	if err := r.db.WithContext(ctx).Delete(TaskEntity{TaskId: taskId}).Error; err != nil {
		return err
	}
	return nil
}

func (r jobRepo) GetTask(ctx context.Context, taskId int64) (TaskEntity, error) {
	e := TaskEntity{TaskId: taskId}
	if err := r.db.WithContext(ctx).First(&e).Error; err != nil {
		return e, err
	}
	return e, nil
}

func (r jobRepo) QueryTask(
	ctx context.Context,
	jobId, thingId string,
	pq TaskPageQuery,
) (model.PageData[TaskEntity], error) {

	offset := pq.Offset()
	limit := pq.Limit()
	var page model.PageData[TaskEntity]
	var total int64
	r.db.WithContext(ctx).Model(&TaskEntity{}).Count(&total)
	if total == 0 {
		page.Content = []TaskEntity{}
		return page, nil
	}
	page.Total = total
	q := r.db.WithContext(ctx).Model(&TaskEntity{}).
		Order("created_at ASC").
		Offset(offset).
		Limit(limit)
	if pq.Status != "" {
		q.Where("status=?", pq.Status)
	}
	if pq.Operation != "" {
		q.Where("operation=?", pq.Operation)
	}
	if jobId != "" {
		q.Where("job_id=?", jobId)
	}
	if thingId != "" {
		q.Where("thing_id=?", thingId)
	}
	if err := q.Find(&page.Content).Error; err != nil {
		return page, err
	}
	return page, nil
}