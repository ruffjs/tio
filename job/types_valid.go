package job

import (
	"fmt"
	"github.com/pkg/errors"
	"regexp"
	"ruff.io/tio/pkg/model"
	"strings"
)

var jobIdRegexp = regexp.MustCompile("^[0-9a-zA-Z_-]{1,64}$")
var operationRegexp = regexp.MustCompile("^[0-9a-zA-Z_-]{1,64}$")

func operationValid(op string) bool {
	return operationRegexp.MatchString(op)
}

func jobIdValid(op string) bool {
	return jobIdRegexp.MatchString(op)
}

func (r CreateReq) valid() error {
	if r.JobId != "" && !jobIdValid(r.JobId) {
		return errors.WithMessage(model.ErrInvalidParams, "field `jobId` should match regex: "+jobIdRegexp.String())
	}
	if r.Operation == "" {
		return errors.WithMessage(model.ErrInvalidParams, "field `operation` can't be empty")
	}
	if r.Operation != SysOpDirectMethod && r.Operation != SysOpUpdateShadow {
		if strings.HasPrefix(r.Operation, "$") {
			return errors.WithMessage(model.ErrInvalidParams,
				"only the system retention `operation` allows the beginning of the $ character")
		}
		if !operationValid(r.Operation) {
			return errors.WithMessage(model.ErrInvalidParams,
				"`operation` should match regex: "+operationRegexp.String())
		}
	}

	if len(r.Description) > 250 {
		return errors.WithMessage(model.ErrInvalidParams, "field `description` length should be less than 250")
	}
	if len(r.JobDoc) > 60000 {
		return errors.WithMessage(model.ErrInvalidParams, "field `jobDoc` length should be less than 60000")
	}

	if r.TargetConfig.Type == "" {
		return errors.WithMessage(model.ErrInvalidParams, "targetConfig type can't be empty")
	}
	if r.TargetConfig.Type != TargetTypeThingId {
		return errors.WithMessage(model.ErrInvalidParams,
			"targetConfig type can only be \""+TargetTypeThingId+"\" at present")
	}
	if len(r.TargetConfig.Things) == 0 {
		return errors.WithMessage(model.ErrInvalidParams, "targetConfig things can't be empty")
	}

	if err := r.SchedulingConfig.valid(); err != nil {
		return err
	}

	if err := r.TimeoutConfig.valid(); err != nil {
		return err
	}
	if err := r.RetryConfig.valid(); err != nil {
		return err
	}

	return nil
}

func (r UpdateReq) valid() error {
	if len(r.Description) > 250 {
		return errors.WithMessage(model.ErrInvalidParams, "field `description` length should be less than 250")
	}
	if err := r.TimeoutConfig.valid(); err != nil {
		return err
	}
	if err := r.RetryConfig.valid(); err != nil {
		return err
	}
	return nil
}

func (r CancelReq) valid() error {
	if len(r.ReasonCode) > 64 {
		return errors.WithMessage(model.ErrInvalidParams,
			"reasonCode length should be less than 64")
	}
	if len(r.Comment) > 250 {
		return errors.WithMessage(model.ErrInvalidParams,
			"comment length should be less than 250")
	}

	return nil
}

func (s *SchedulingConfig) valid() error {
	if s == nil {
		return nil
	}
	if s.StartTime.IsZero() || s.EndBehavior == "" {
		return errors.WithMessagef(model.ErrInvalidParams,
			"scheduleConfig startTime and endBehavior can't be empty")
	}
	if _, err := ScheduleEndBehavior.Of("", string(s.EndBehavior)); err != nil {
		return errors.WithMessage(model.ErrInvalidParams, "scheduleConfig endBehavior value is invalid: "+err.Error())
	}
	if s.EndTime != nil && s.StartTime.After(*s.EndTime) {
		return errors.WithMessage(model.ErrInvalidParams, "scheduleConfig startTime should before endTime")
	}

	return nil
}

func (c *TimeoutConfig) valid() error {
	if c == nil {
		return nil
	}
	if c.InProgressMinutes < 1 || c.InProgressMinutes > 10080 {
		return errors.WithMessage(model.ErrInvalidParams,
			"timoutConfig inProgressMinutes should between 1 and 10080")
	}
	return nil
}

func (c *RetryConfig) valid() error {
	if c == nil {
		return nil
	}
	for _, l := range c.CriteriaList {
		if l.FailureType != TaskFailed.String() && l.FailureType != TaskTimeOut.String() && l.FailureType != "ALL" {
			return errors.WithMessage(model.ErrInvalidParams,
				fmt.Sprintf("retryConfig failureType should be ALL, %s or %s", TaskFailed, TaskTimeOut))
		}
		if l.NumberOfRetries < 0 || l.NumberOfRetries > 10 {
			return errors.WithMessage(model.ErrInvalidParams,
				"retryConfig numberOfRetries should between 0 and 10")
		}
	}
	return nil
}
