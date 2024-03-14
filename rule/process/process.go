package process

import (
	"fmt"
)

const (
	TypeFilter = "filter"
	TypeTrans  = "transform"
)

type Process interface {
	Type() string
	Name() string
	Run(in any) (out any, err error)
}

type Config struct {
	Name string
	Type string
	Jq   string
}

func NewProcess(cfg Config) (Process, error) {
	switch cfg.Type {
	case TypeFilter:
		return NewFilter(cfg.Name, cfg.Jq)
	case TypeTrans:
		return NewTrans(cfg.Name, cfg.Jq)
	default:
		return nil, fmt.Errorf("unsupported process type %q", cfg.Type)
	}
}
