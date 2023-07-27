package job

import (
	"context"
	"strings"
	"time"

	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"ruff.io/tio/pkg/model"
)

type jobRepo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) Repo {
	return jobRepo{db}
}

func (r jobRepo) ExecWithTx(f func(r Repo) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		txRepo := NewRepo(tx)
		return f(txRepo)
	})
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
	return *e, nil
}

func (r jobRepo) UpdateJob(ctx context.Context, id string, m map[string]any) error {
	for k := range m {
		lk := strings.ToLower(k)
		if lk == "jobid" || lk == "job_id" {
			return errors.New("can't update jobId")
		}
	}
	if err := r.db.WithContext(ctx).Model(Entity{JobId: id}).Updates(m).Error; err != nil {
		return err
	}
	return nil
}

// DeleteJob The tasks under the job will be deleted together cause `ON DELETE CASCADE`
func (r jobRepo) DeleteJob(ctx context.Context, id string, force bool) (*Entity, error) {
	en := Entity{JobId: id}
	if force {
		if err := r.db.WithContext(ctx).Delete(Entity{JobId: id}).Error; err != nil {
			return nil, err
		}
	} else {
		if res := r.db.WithContext(ctx).
			Clauses(clause.Returning{}).
			Where("status != ?", StatusInProgress).
			Delete(&en); res.Error != nil {
			return nil, res.Error
		} else if res.RowsAffected == 0 {
			return nil, errors.WithMessage(model.ErrInvalidParams, "job is IN_PROGRESS or not exist")
		}
	}
	// sqlite does not support CASCADE
	if r.db.Dialector.Name() == "sqlite" {
		r.db.Where("job_id=?", id).Delete(&TaskEntity{})
	}
	return &en, nil
}

func (r jobRepo) GetJob(ctx context.Context, id string) (*Entity, error) {
	e := Entity{JobId: id}
	if err := r.db.WithContext(ctx).First(&e).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return &Entity{}, err
	} else {
		return &e, nil
	}
}

func (r jobRepo) QueryJob(ctx context.Context, pq PageQuery) (model.PageData[Entity], error) {
	offset := pq.Offset()
	limit := pq.Limit()
	var page model.PageData[Entity]
	q := r.db.WithContext(ctx).Model(&Entity{}).
		Order("created_at ASC")
	if pq.Status != "" {
		q.Where("status=?", pq.Status)
	}
	if pq.Operation != "" {
		q.Where("operation=?", pq.Operation)
	}

	var total int64
	q.Count(&total)
	if total == 0 {
		page.Content = []Entity{}
		return page, nil
	}
	page.Total = total

	q.Offset(offset).Limit(limit)
	if err := q.Find(&page.Content).Error; err != nil {
		return page, err
	}
	return page, nil
}

func (r jobRepo) CreateTasks(ctx context.Context, l []TaskEntity) ([]TaskEntity, error) {
	if res := r.db.WithContext(ctx).Create(&l); res.Error != nil {
		return []TaskEntity{}, res.Error
	} else {
		var ids []int64
		for _, t := range l {
			ids = append(ids, t.TaskId)
		}
		var rl []TaskEntity
		if err := r.db.Where("task_id in ?", ids).Find(&rl).Error; err != nil {
			return []TaskEntity{}, err
		} else {
			return rl, nil
		}
	}
}

func (r jobRepo) CancelTasks(ctx context.Context, jobId string, force bool) error {
	up := r.db.WithContext(ctx).Model(TaskEntity{}).Where("job_id = ?", jobId)
	if force {
		up.Where("status in ?", []TaskStatus{TaskQueued, TaskSent, TaskInProgress})
	} else {
		up.Where("status = ?", TaskQueued)
	}

	res := up.Updates(map[string]any{
		"force_canceled": force,
		"status":         TaskCanceled,
		"completed_at":   time.Now(),
	})
	return res.Error
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

func (r jobRepo) GetTask(ctx context.Context, taskId int64) (*TaskEntity, error) {
	e := TaskEntity{TaskId: taskId}
	if err := r.db.WithContext(ctx).First(&e).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return &TaskEntity{}, err
	}
	return &e, nil
}

func (r jobRepo) QueryTask(
	ctx context.Context,
	jobId, thingId string,
	pq TaskPageQuery,
) (model.PageData[TaskEntity], error) {

	offset := pq.Offset()
	limit := pq.Limit()
	var page model.PageData[TaskEntity]
	q := r.db.WithContext(ctx).Model(&TaskEntity{}).
		Order("created_at ASC")
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

	var total int64
	q.Count(&total)
	if total == 0 {
		page.Content = []TaskEntity{}
		return page, nil
	}
	page.Total = total

	q.Offset(offset).Limit(limit)
	if err := q.Find(&page.Content).Error; err != nil {
		return page, err
	}
	return page, nil
}

func (r jobRepo) CountTaskStatus(ctx context.Context, jobId string) ([]TaskStatusCount, error) {
	var tsc []TaskStatusCount
	res := r.db.WithContext(ctx).Model(TaskEntity{}).
		Where("job_id=?", jobId).
		Select("status, COUNT(*) AS count").
		Group("status").
		Scan(&tsc)
	if res.Error != nil {
		return tsc, res.Error
	}
	return tsc, nil
}

func (r jobRepo) GetTasksOfJob(ctx context.Context, jobId string, status []TaskStatus) ([]TaskEntity, error) {
	var l []TaskEntity
	q := r.db.WithContext(ctx).Model(TaskEntity{}).
		Where("job_id=?", jobId)
	if len(status) != 0 {
		q.Where("status in ?", status)
	}
	if err := q.Scan(&l).Error; err != nil {
		return l, err
	}
	return l, nil
}

func (r jobRepo) GetPendingJobs(ctx context.Context) ([]Entity, error) {
	var l []Entity
	q := r.db.WithContext(ctx).Model(Entity{}).
		//Joins("Tasks").
		Preload(
			"Tasks",
			"(status in ? OR  status IS NULL)",
			[]TaskStatus{TaskQueued, TaskSent, TaskInProgress},
		).
		Where("status in ?", []Status{StatusWaiting, StatusInProgress, StatusCanceling, StatusRemoving})

	if err := q.Find(&l).Error; err != nil {
		return l, err
	}
	return l, nil
}
