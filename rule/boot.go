package rule

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"sync"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/spf13/viper"
	"ruff.io/tio/rule/connector"
	"ruff.io/tio/rule/model"
	"ruff.io/tio/rule/process"
	"ruff.io/tio/rule/sink"
	"ruff.io/tio/rule/source"
	"ruff.io/tio/shadow"
)

const (
	TypeRule      = "rule"
	TypeConnector = "connector"
	TypeSource    = "source"
	TypeSink      = "sink"
)

type RuleMgr struct {
	ctx          context.Context
	cfg          Config
	shadowGetter shadow.CacheService

	conns   map[string]connector.Conn
	sinks   map[string]sink.Sink
	sources map[string]source.Source
	rules   map[string]Rule

	mu sync.RWMutex

	initErrors map[string]map[string]error // type=>name=>error
}

type RuleStatusInfo map[string][]RuleStatusItem // type=>[RuleStatusItem]
type RuleStatusItem struct {
	Name   string           `json:"name"`
	Status model.StatusInfo `json:"status"`
}

// ComponentChanges stores component change information
type ComponentChanges struct {
	Removed map[string][]string // type -> name list
	Changed map[string][]string // type -> name list
}

func NewRuleMgr() *RuleMgr {
	m := RuleMgr{
		conns:   make(map[string]connector.Conn),
		sinks:   make(map[string]sink.Sink),
		sources: make(map[string]source.Source),
		rules:   make(map[string]Rule),
		initErrors: map[string]map[string]error{ // type=>name=>error
			TypeConnector: {},
			TypeSource:    {},
			TypeSink:      {},
			TypeRule:      {},
		},
	}
	m.loadConfig()
	return &m
}

func NewComponentChanges() ComponentChanges {
	return ComponentChanges{
		Removed: map[string][]string{
			TypeConnector: {},
			TypeSource:    {},
			TypeSink:      {},
			TypeRule:      {},
		},
		Changed: map[string][]string{
			TypeConnector: {},
			TypeSource:    {},
			TypeSink:      {},
			TypeRule:      {},
		},
	}
}

// Boot Read rule config, assemble rules and then boot them
//
// If config file is not exist, give up
func (r *RuleMgr) Boot(ctx context.Context, shadowGetter shadow.CacheService) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.ctx = ctx
	r.shadowGetter = shadowGetter

	if err := r.loadConfig(); err != nil {
		slog.Error("Rule boot", "error", err)
		return
	}
	r.start(ctx)
}

func (r *RuleMgr) GetConfig() Config {
	return r.cfg
}

func (r *RuleMgr) GetStatus() RuleStatusInfo {
	rs := RuleStatusInfo{
		TypeConnector: make([]RuleStatusItem, 0),
		TypeSource:    make([]RuleStatusItem, 0),
		TypeSink:      make([]RuleStatusItem, 0),
		TypeRule:      make([]RuleStatusItem, 0),
	}

	get := func(typ string, c StatusGetter) {
		st := RuleStatusItem{Name: c.Name()}
		if err, ok := r.initErrors[typ][c.Name()]; ok {
			if typ == TypeRule {
				st.Status = model.StatusInfo{Status: "init-failed", Reason: err.Error(), Error: err}
			} else {
				st.Status = model.StatusDisconnected("init failed", err)
			}
		} else {
			st.Status = c.Status()
		}
		rs[typ] = append(rs[typ], st)
	}

	for _, c := range r.conns {
		get(TypeConnector, c)
	}
	for _, c := range r.sources {
		get(TypeSource, c)
	}
	for _, c := range r.sinks {
		get(TypeSink, c)
	}
	for _, c := range r.rules {
		get(TypeRule, c)
	}

	return rs
}

// ApplyConfig update rule config, and reload rules
func (r *RuleMgr) ApplyConfig(cfg Config) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := SetConfig(cfg); err != nil {
		return errors.WithMessagef(err, "save config to file")
	}

	changes := r.diffConfig(cfg)
	slog.Info("Compare configs", "changes", changes)

	r.cfg = cfg
	r.stopComponents(changes)
	r.startComponents(changes)

	return nil
}

func (r *RuleMgr) diffConfig(newCfg Config) ComponentChanges {
	changes := NewComponentChanges()

	// Compare connectors
	oldConnectors := lo.SliceToMap(r.cfg.Connectors, func(c connector.Config) (string, connector.Config) {
		return c.Name, c
	})
	for _, newConn := range newCfg.Connectors {
		if oldConn, exists := oldConnectors[newConn.Name]; !exists {
			changes.Changed[TypeConnector] = append(changes.Changed[TypeConnector], newConn.Name) // New connector
		} else if oldConn.Type != newConn.Type || !compareOptions(oldConn.Options, newConn.Options) {
			changes.Changed[TypeConnector] = append(changes.Changed[TypeConnector], newConn.Name) // Changed connector
		}
		delete(oldConnectors, newConn.Name)
	}
	for name := range oldConnectors {
		changes.Removed[TypeConnector] = append(changes.Removed[TypeConnector], name) // Removed connector
	}

	// Compare sources
	oldSources := lo.SliceToMap(r.cfg.Sources, func(c source.Config) (string, source.Config) {
		return c.Name, c
	})
	for _, newSrc := range newCfg.Sources {
		if oldSrc, exists := oldSources[newSrc.Name]; !exists {
			changes.Changed[TypeSource] = append(changes.Changed[TypeSource], newSrc.Name) // New source
		} else if oldSrc.Type != newSrc.Type || !compareOptions(oldSrc.Options, newSrc.Options) {
			changes.Changed[TypeSource] = append(changes.Changed[TypeSource], newSrc.Name) // Changed source
		}
		delete(oldSources, newSrc.Name)
	}
	for name := range oldSources {
		changes.Removed[TypeSource] = append(changes.Removed[TypeSource], name) // Removed source
	}

	// Compare sinks
	oldSinks := lo.SliceToMap(r.cfg.Sinks, func(c sink.Config) (string, sink.Config) {
		return c.Name, c
	})
	for _, newSink := range newCfg.Sinks {
		if oldSink, exists := oldSinks[newSink.Name]; !exists {
			changes.Changed[TypeSink] = append(changes.Changed[TypeSink], newSink.Name) // New sink
		} else if oldSink.Type != newSink.Type || !compareOptions(oldSink.Options, newSink.Options) {
			changes.Changed[TypeSink] = append(changes.Changed[TypeSink], newSink.Name) // Changed sink
		}
		delete(oldSinks, newSink.Name)
	}
	for name := range oldSinks {
		changes.Removed[TypeSink] = append(changes.Removed[TypeSink], name) // Removed sink
	}

	// Compare rules
	oldRules := lo.SliceToMap(r.cfg.Rules, func(c RuleConfig) (string, RuleConfig) {
		return c.Name, c
	})
	for _, newRule := range newCfg.Rules {
		if oldRule, exists := oldRules[newRule.Name]; !exists {
			changes.Changed[TypeRule] = append(changes.Changed[TypeRule], newRule.Name) // New rule
		} else if !compareRuleConfig(oldRule, newRule) {
			changes.Changed[TypeRule] = append(changes.Changed[TypeRule], newRule.Name) // Changed rule
		}
		delete(oldRules, newRule.Name)
	}
	for name := range oldRules {
		changes.Removed[TypeRule] = append(changes.Removed[TypeRule], name) // Removed rule
	}

	// -------- Get components that depend on changed components --------

	// Get sources and sinks that depend on changed connectors
	for _, name := range changes.Changed[TypeConnector] {
		for _, src := range r.cfg.Sources {
			if src.Connector == name {
				changes.Changed[TypeSource] = append(changes.Changed[TypeSource], src.Name)
			}
		}
		for _, sink := range r.cfg.Sinks {
			if sink.Connector == name {
				changes.Changed[TypeSink] = append(changes.Changed[TypeSink], sink.Name)
			}
		}
	}

	// Get rules that depend on changed sources and sinks
	for _, name := range changes.Changed[TypeSource] {
		for _, rule := range r.cfg.Rules {
			if lo.Contains(rule.Sources, name) {
				changes.Changed[TypeRule] = append(changes.Changed[TypeRule], rule.Name)
			}
		}
	}
	for _, name := range changes.Changed[TypeSink] {
		for _, rule := range r.cfg.Rules {
			if lo.Contains(rule.Sinks, name) {
				changes.Changed[TypeRule] = append(changes.Changed[TypeRule], rule.Name)
			}
		}
	}

	// Distinct changed components
	changes.Changed[TypeConnector] = lo.Uniq(changes.Changed[TypeConnector])
	changes.Changed[TypeSource] = lo.Uniq(changes.Changed[TypeSource])
	changes.Changed[TypeSink] = lo.Uniq(changes.Changed[TypeSink])
	changes.Changed[TypeRule] = lo.Uniq(changes.Changed[TypeRule])

	return changes
}

func compareOptions(oldComponent map[string]any, newOptions map[string]any) bool {
	return reflect.DeepEqual(normalizeOptions(oldComponent), normalizeOptions(newOptions))
}

func compareRuleConfig(oldRule RuleConfig, newRule RuleConfig) bool {
	return reflect.DeepEqual(oldRule, newRule)
}

// normalizeOptions normalize options
// Convert all numbers to float64
func normalizeOptions(options any) any {
	switch v := options.(type) {
	case map[string]any:
		normalized := make(map[string]any)
		for key, value := range v {
			normalized[key] = normalizeOptions(value)
		}
		return normalized
	case []any:
		for i, value := range v {
			v[i] = normalizeOptions(value)
		}
		return v
	case int:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case float32:
		return float64(v)
	case float64:
		return v
	default:
		return v
	}
}

func (r *RuleMgr) stopComponents(changes ComponentChanges) {
	// stop and remove deleted components
	r.stopComponentsByType(TypeConnector, changes.Removed[TypeConnector])
	r.stopComponentsByType(TypeSource, changes.Removed[TypeSource])
	r.stopComponentsByType(TypeSink, changes.Removed[TypeSink])
	r.stopComponentsByType(TypeRule, changes.Removed[TypeRule])

	// stop changed components
	r.stopComponentsByType(TypeConnector, changes.Changed[TypeConnector])
	r.stopComponentsByType(TypeSource, changes.Changed[TypeSource])
	r.stopComponentsByType(TypeSink, changes.Changed[TypeSink])
	r.stopComponentsByType(TypeRule, changes.Changed[TypeRule])
}

func (r *RuleMgr) stopComponentsByType(typ string, names []string) {
	for _, name := range names {
		switch typ {
		case TypeConnector:
			if conn, exists := r.conns[name]; exists {
				err := conn.Stop()
				if err != nil {
					slog.Error("Rule failed to stop connector", "name", name, "error", err)
				}
				delete(r.conns, name)
			}
		case TypeSource:
			if src, exists := r.sources[name]; exists {
				src.Stop()
				delete(r.sources, name)
			}
		case TypeSink:
			if sink, exists := r.sinks[name]; exists {
				err := sink.Stop()
				if err != nil {
					slog.Error("Rule failed to stop sink", "name", name, "error", err)
				}
				delete(r.sinks, name)
			}
		case TypeRule:
			if rule, exists := r.rules[name]; exists {
				rule.Stop()
				delete(r.rules, name)
			}
		}
	}
}

// start new or changed components
func (r *RuleMgr) startComponents(changes ComponentChanges) {
	r.startComponentsByType(TypeConnector, changes.Changed[TypeConnector])
	r.startComponentsByType(TypeSource, changes.Changed[TypeSource])
	r.startComponentsByType(TypeSink, changes.Changed[TypeSink])
	r.startComponentsByType(TypeRule, changes.Changed[TypeRule])
}

func (r *RuleMgr) startComponentsByType(typ string, names []string) {
	for _, name := range names {
		switch typ {
		case TypeConnector:
			for _, cfg := range r.cfg.Connectors {
				if cfg.Name == name {
					if !cfg.Enabled {
						continue
					}
					conn, err := r.initConn(r.ctx, cfg)
					if err != nil {
						slog.Error("Rule failed to initialize connector", "name", name, "error", err)
						continue
					}
					r.conns[name] = conn
					err = conn.Start()
					if err != nil {
						slog.Error("Rule failed to start connector", "name", name, "error", err)
					}
					break
				}
			}
		case TypeSource:
			for _, cfg := range r.cfg.Sources {
				if cfg.Name == name {
					if !cfg.Enabled {
						continue
					}
					src, err := r.initSource(r.ctx, cfg)
					if err != nil {
						slog.Error("Rule failed to initialize source", "name", name, "error", err)
						continue
					}
					r.sources[name] = src
					err = src.Start()
					if err != nil {
						slog.Error("Rule failed to start source", "name", name, "error", err)
					}
					break
				}
			}
		case TypeSink:
			for _, cfg := range r.cfg.Sinks {
				if cfg.Name == name {
					if !cfg.Enabled {
						continue
					}
					sink, err := r.initSink(r.ctx, cfg)
					if err != nil {
						slog.Error("Rule failed to initialize sink", "name", name, "error", err)
						continue
					}
					r.sinks[name] = sink
					err = sink.Start()
					if err != nil {
						slog.Error("Rule failed to start sink", "name", name, "error", err)
					}
					break
				}
			}
		case TypeRule:
			for _, cfg := range r.cfg.Rules {
				if cfg.Name == name {
					if !cfg.Enabled {
						continue
					}
					rule, err := r.initRule(cfg, r.shadowGetter)
					if err != nil {
						slog.Error("Rule failed to initialize rule", "name", name, "error", err)
						continue
					}
					r.rules[name] = rule
					rule.Start(r.ctx)
					break
				}
			}
		}
	}
}

// Enable enable or disable connector,source,sink or rule by type and name
func (r *RuleMgr) Enable(typ string, name string, enable bool) (err error) {
	// TODO optimize duplicated code
	switch typ {
	case TypeRule:
		if rule, ok := r.rules[name]; ok {
			if enable {
				rule.Start(r.ctx)
			} else {
				rule.Stop()
			}
			for i, c := range r.cfg.Rules {
				if c.Name == name {
					r.cfg.Rules[i].Enabled = enable
					break
				}
			}
		} else {
			return fmt.Errorf("rule %q not found", name)
		}

	case TypeConnector:
		if conn, ok := r.conns[name]; ok {
			if enable {
				err = conn.Start()
			} else {
				err = conn.Stop()
			}
			for i, c := range r.cfg.Connectors {
				if c.Name == name {
					r.cfg.Connectors[i].Enabled = enable
					break
				}
			}
		} else {
			return fmt.Errorf("connector %q not found", name)
		}
	case TypeSource:
		if source, ok := r.sources[name]; ok {
			if enable {
				err = source.Start()
			} else {
				source.Stop()
			}
			for i, c := range r.cfg.Sources {
				if c.Name == name {
					r.cfg.Sources[i].Enabled = enable
					break
				}
			}
		} else {
			return fmt.Errorf("source %q not found", name)
		}
	case TypeSink:
		if sinks, ok := r.sinks[name]; ok {
			if enable {
				err = sinks.Start()
			} else {
				err = sinks.Stop()
			}
			for i, c := range r.cfg.Sinks {
				if c.Name == name {
					r.cfg.Sinks[i].Enabled = enable
					break
				}
			}
		} else {
			return fmt.Errorf("sink %q not found", name)
		}
	default:
		return fmt.Errorf("unknown type %q", typ)
	}

	// save to config file
	if e := SetConfig(r.cfg); e != nil {
		return errors.WithMessagef(e, "save config to file")
	}
	return
}

func (r *RuleMgr) loadConfig() error {
	cfg, err := ReadConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return fmt.Errorf("config file not found, if you do't need it, ignore this log")
		} else {
			return fmt.Errorf("failed load config: %w", err)
		}
	}
	r.cfg = cfg
	return nil
}

// start sequence: connectors -> sinks -> rules -> sources
func (r *RuleMgr) start(ctx context.Context) {
	for _, cc := range r.cfg.Connectors {
		conn, err := r.initConn(ctx, cc)
		if err != nil {
			r.initErrors[TypeConnector][cc.Name] = err
			slog.Error("Rule init connector failed", "name", cc.Name, "error", err)
		} else {
			r.conns[cc.Name] = conn
			if !cc.Enabled {
				continue
			}
			if err := conn.Start(); err != nil {
				slog.Error("Rule connector start error", "name", cc.Name, "error", err)
			} else {
				slog.Info("Rule connector started", "name", cc.Name)
			}
		}
	}

	for _, sc := range r.cfg.Sinks {
		s, err := r.initSink(ctx, sc)
		if err != nil {
			r.initErrors[TypeSink][sc.Name] = err
			slog.Error("Rule init sinks failed", "name", sc.Name, "error", err)
		} else {
			r.sinks[sc.Name] = s
			if !sc.Enabled {
				continue
			}
			if err := s.Start(); err != nil {
				slog.Error("Rule sink start error", "name", sc.Name, "error", err)
			} else {
				slog.Info("Rule sink started", "name", sc.Name)
			}
		}
	}

	for _, sc := range r.cfg.Sources {
		s, err := r.initSource(ctx, sc)
		if err != nil {
			r.initErrors[TypeSource][sc.Name] = err
			slog.Error("Rule init source failed", "name", sc.Name, "error", err)
		} else {
			r.sources[sc.Name] = s
			if !sc.Enabled {
				continue
			}
			if err := s.Start(); err != nil {
				slog.Error("Rule source start failed", "name", sc.Name, "error", err)
			} else {
				slog.Info("Rule source started", "name", sc.Name)
			}
		}
	}

	// Crete rules
	for _, rc := range r.cfg.Rules {
		rule, err := r.initRule(rc, r.shadowGetter)
		if err != nil {
			r.initErrors[TypeRule][rc.Name] = err
			slog.Error("Init rule failed", "name", rc.Name, "error", err)
		} else {
			r.rules[rc.Name] = rule
			if !rc.Enabled {
				continue
			}
			rule.Start(ctx)
			slog.Info("Rule started", "name", rc.Name)
		}
	}
}

func (r *RuleMgr) initRule(rc RuleConfig, shadowGetter shadow.CacheService) (Rule, error) {
	sks := make([]sink.Sink, 0)
	srcs := make([]source.Source, 0)
	for _, sn := range rc.Sinks {
		if sk, ok := r.sinks[sn]; ok {
			sks = append(sks, sk)
		} else {
			return nil, fmt.Errorf("no sink named %q for rule", sn)
		}
	}
	for _, sn := range rc.Sources {
		if src, ok := r.sources[sn]; ok {
			srcs = append(srcs, src)
		} else {
			return nil, fmt.Errorf("no source named %q", sn)
		}
	}
	plist := make([]process.Process, 0)
	for _, cfg := range rc.Process {
		p, err := process.NewProcess(cfg)
		if err != nil {
			return nil, fmt.Errorf("init process %q failed: %w", cfg.Name, err)
		}
		plist = append(plist, p)
	}
	rule := NewRule(rc.Name, srcs, plist, sks, shadowGetter)
	if _, ok := r.rules[rc.Name]; ok {
		return nil, fmt.Errorf("duplicated rule name")
	}
	return rule, nil
}

func (r *RuleMgr) initConn(ctx context.Context, cfg connector.Config) (connector.Conn, error) {
	c, err := connector.New(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("new connector failed: %w", err)
	}
	if _, ok := r.conns[cfg.Name]; ok {
		return nil, fmt.Errorf("duplicated name")
	}
	return c, nil
}

func (r *RuleMgr) initSink(ctx context.Context, cfg sink.Config) (sink.Sink, error) {
	var c connector.Conn
	if cfg.Connector != "" {
		cc, ok := r.conns[cfg.Connector]
		if !ok {
			return nil, fmt.Errorf("no connector named %q", cfg.Connector)
		} else {
			c = cc
		}
	}
	s, err := sink.New(ctx, cfg, c)
	if err != nil {
		return nil, fmt.Errorf("new sink: %w", err)
	}
	if _, ok := r.sinks[cfg.Name]; ok {
		return nil, fmt.Errorf("duplicated name")
	}
	r.sinks[cfg.Name] = s
	return s, nil
}

func (r *RuleMgr) initSource(ctx context.Context, cfg source.Config) (source.Source, error) {
	var c connector.Conn
	if cfg.Connector != "" {
		cc, ok := r.conns[cfg.Connector]
		if !ok {
			return nil, fmt.Errorf("no connector named %q", cfg.Connector)
		} else {
			c = cc
		}
	}
	s, err := source.New(ctx, cfg, c)
	if err != nil {
		slog.Error("Init rule source", "name", cfg.Name, "type", cfg.Type, "error", err)
		return nil, fmt.Errorf("new source: %w", err)
	}
	if _, ok := r.conns[cfg.Name]; ok {
		return nil, fmt.Errorf("duplicated name")
	}
	return s, nil
}
