package model

import "time"

type PageData[T any] struct {
	Total   int64 `json:"total"`
	Content []T   `json:"content"`
}

type PageQuery struct {
	PageIndex int `qs:"pageIndex" json:"pageIndex"`
	PageSize  int `qs:"pageSize" json:"pageSize"`
}

func (q *PageQuery) Offset() int {
	return (q.PageIndex - 1) * q.PageSize
}

func (q *PageQuery) Limit() int {
	return q.PageSize
}

type TimeSpan struct {
	From time.Time // eg: 2022-11-01T15:00:00Z
	To   time.Time
}

func Ref[T any](v T) *T {
	return &v
}
