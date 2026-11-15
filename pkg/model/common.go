package model

import "time"

type PageData[T any] struct {
	Total   int64 `json:"total"`
	Content []T   `json:"content"`
}

const (
	DefaultPageSize = 10
	MaxPageSize     = 10000
)

type PageQuery struct {
	PageIndex int `qs:"pageIndex" json:"pageIndex"`
	PageSize  int `qs:"pageSize" json:"pageSize"`
}

// Normalize bounds user supplied paging so a negative or huge pageSize cannot
// turn into an unbounded query. Callers read the normalized values back.
func (q *PageQuery) Normalize() {
	if q.PageIndex < 1 {
		q.PageIndex = 1
	}
	if q.PageSize < 1 {
		q.PageSize = DefaultPageSize
	}
	if q.PageSize > MaxPageSize {
		q.PageSize = MaxPageSize
	}
}

func (q *PageQuery) Offset() int {
	q.Normalize()
	return (q.PageIndex - 1) * q.PageSize
}

func (q *PageQuery) Limit() int {
	q.Normalize()
	return q.PageSize
}

type TimeSpan struct {
	From time.Time // eg: 2022-11-01T15:00:00Z
	To   time.Time
}

//go:fix inline
func Ref[T any](v T) *T {
	return new(v)
}
