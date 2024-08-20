package process

import (
	"fmt"
)

const (
	TypeFilter = "filter"
	TypeTrans  = "transform"

	RunnerJq = "jq"
	RunnerJs = "js"
)

type Process interface {
	Type() string
	Name() string
	Run(in any) (out any, err error)
}

type Config struct {
	Name   string
	Type   string
	Runner string // jq or js
	Jq     string
	Js     string
}

func NewProcess(cfg Config) (Process, error) {
	BuildConfig(&cfg)

	if cfg.Runner != RunnerJs && cfg.Runner != RunnerJq {
		return nil, fmt.Errorf("unsupported runner %q, should be \"js\" or \"jq\"", cfg.Runner)
	}
	if cfg.Runner == RunnerJs && cfg.Js == "" {
		return nil, fmt.Errorf("js runner must have js script")
	}
	if cfg.Runner == RunnerJq && cfg.Jq == "" {
		return nil, fmt.Errorf("jq runner must have jq script")
	}

	switch cfg.Type {
	case TypeFilter:
		return NewFilter(cfg)
	case TypeTrans:
		return NewTrans(cfg)
	default:
		return nil, fmt.Errorf("unsupported process type %q", cfg.Type)
	}
}

func BuildConfig(cfg *Config) {
	if cfg.Runner == "" {
		if cfg.Jq == "" && cfg.Js != "" {
			cfg.Runner = RunnerJs
		}
		if cfg.Jq != "" && cfg.Js == "" {
			cfg.Runner = RunnerJq
		}
	}
}
