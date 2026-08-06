package shadow

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"ruff.io/tio/connector"

	"github.com/pkg/errors"
	"ruff.io/tio/pkg/model"
)

const (
	StateTypeDesired  = "desired"
	StateTypeReported = "reported"

	MaxShadowCount = 100_000
)

type Service interface {
	Init(ctx context.Context)
	HandleLocalPresence(ci connector.ClientInfo)
	StateService
	CrudService
	TagsService
	CacheService
}

type StateUpdateSubscribe func(thingId string, state StateUpdatedNotice)
type StateDeltaSubscribe func(thingId string, delta DeltaStateNotice)
type StateAcceptedSubscribe func(thingId string, msg StateAcceptedRespMsg)
type StateRejectedSubscribe func(thingId string, msg ErrRespMsg)

type StateService interface {
	StateDesiredSetter
	SetReported(ctx context.Context, thingId string, sr StateReq) (Shadow, error)
	SubscribeUpdate(StateUpdateSubscribe)
	SubscribeDelta(StateDeltaSubscribe)
	SubAccepted(StateAcceptedSubscribe)
	SubRejected(StateRejectedSubscribe)
}

type StateDesiredSetter interface {
	SetDesired(ctx context.Context, thingId string, sr StateReq) (Shadow, error)
}

type CrudService interface {
	Create(ctx context.Context, thingId string) (Shadow, error)
	Delete(ctx context.Context, thingId string) error
	Query(ctx context.Context, page model.PageQuery, query string) (Page, error)
	Get(ctx context.Context, thingId string) (ShadowWithStatus, error)
}
type CacheGetter interface {
	GetFromCache(thingId string) (ShadowWithStatus, bool)
}

type TagsService interface {
	SetTag(ctx context.Context, thingId string, tag TagsReq) error
}

type CacheService interface {
	CacheGetter

	// NotifyCreated notify shadow created for cache update
	NotifyCreated(thingId string, s ShadowWithEnable)
	// NotifyDeleted notify shadow deleted for cache update
	NotifyDeleted(thingId string)
	// NotifyUpdate notify shadow updated for cache update
	NotifyUpdate(thingId string, enable bool)
}

type GetOption struct {
	WithStatus bool
}
type Query struct {
	MaxResults  uint   `json:"maxResults"`
	NextToken   string `json:"nextToken"`
	QueryString string `json:"queryString"`
}

type Page = model.PageData[any]

type Repo interface {
	ExecWithTx(f func(txtRepo Repo) error) error
	Create(ctx context.Context, thingId string, s Shadow) (*Shadow, error)
	Delete(ctx context.Context, thingId string) error
	Update(ctx context.Context, thingId string, version int64, s Shadow) (*Shadow, error)
	Get(ctx context.Context, thingId string) (*Shadow, error)
	GetWithStatus(ctx context.Context, thingId string) (*ShadowWithStatus, error)
	Query(ctx context.Context, q model.PageQuery, query ParsedQuerySql) (model.PageData[ShadowWithStatus], error)

	UpdateConnStatus(ctx context.Context, s []connector.ClientInfo) error
	UpdateAllConnStatusDisconnect(ctx context.Context, updateTimeBefore time.Time) error
}

type Config struct {
	IgnoreMetadataFor []string `json:"ignoreMetadataFor"`
}

var _ Service = (*shadowSvc)(nil)

type shadowSvc struct {
	repo                Repo
	cache               Cache
	cfg                 Config
	connectorChecker    connector.ConnectChecker
	ctx                 context.Context
	updateSubscribers   []StateUpdateSubscribe
	deltaSubscribers    []StateDeltaSubscribe
	acceptedSubscribers []StateAcceptedSubscribe
	rejectedSubscribers []StateRejectedSubscribe
}

var svcSingleton *shadowSvc
var svcOnce sync.Once

func NewSvc(r Repo, a connector.ConnectChecker, cfg Config) Service {
	svcOnce.Do(func() {
		u := make([]StateUpdateSubscribe, 0)
		d := make([]StateDeltaSubscribe, 0)
		acp := make([]StateAcceptedSubscribe, 0)
		rjt := make([]StateRejectedSubscribe, 0)
		svcSingleton = &shadowSvc{
			repo:                r,
			cache:               newCache(),
			cfg:                 cfg,
			connectorChecker:    a,
			updateSubscribers:   u,
			deltaSubscribers:    d,
			acceptedSubscribers: acp,
			rejectedSubscribers: rjt,
		}
	})
	return svcSingleton
}

func NewTestSvc(r Repo, a connector.ConnectChecker, cfg Config) Service {
	u := make([]StateUpdateSubscribe, 0)
	d := make([]StateDeltaSubscribe, 0)
	acp := make([]StateAcceptedSubscribe, 0)
	rjt := make([]StateRejectedSubscribe, 0)
	return &shadowSvc{
		repo:                r,
		cache:               newCache(),
		cfg:                 cfg,
		connectorChecker:    a,
		updateSubscribers:   u,
		deltaSubscribers:    d,
		acceptedSubscribers: acp,
		rejectedSubscribers: rjt,
	}
}

func (s *shadowSvc) Init(ctx context.Context) {
	s.ctx = ctx
	if err := s.doFirstSyncStatus(ctx); err != nil {
		slog.Error("sync conn status on init", "error", err)
	}
}

func (s *shadowSvc) SubscribeUpdate(subscribe StateUpdateSubscribe) {
	s.updateSubscribers = append(s.updateSubscribers, subscribe)
}

func (s *shadowSvc) SubscribeDelta(subscribe StateDeltaSubscribe) {
	s.deltaSubscribers = append(s.deltaSubscribers, subscribe)
}

func (s *shadowSvc) SubAccepted(subscribe StateAcceptedSubscribe) {
	s.acceptedSubscribers = append(s.acceptedSubscribers, subscribe)
}

func (s *shadowSvc) SubRejected(subscribe StateRejectedSubscribe) {
	s.rejectedSubscribers = append(s.rejectedSubscribers, subscribe)
}

func (s *shadowSvc) SetDesired(ctx context.Context, thingId string, sr StateReq) (Shadow, error) {
	ss, _, err := s.setState(ctx, thingId, sr, true)
	return ss, err
}

func (s *shadowSvc) SetReported(ctx context.Context, thingId string, sr StateReq) (Shadow, error) {
	ss, updatedMeta, err := s.setState(ctx, thingId, sr, false)
	if err != nil {
		s.notifyRejected(thingId, sr.ClientToken, err)
	} else {
		sar := StateAcceptedResp{
			State:       StateDRD{Reported: sr.State.Reported},
			Timestamp:   time.Now().UnixMilli(),
			ClientToken: sr.ClientToken,
			Version:     ss.Version,
		}
		if updatedMeta != nil && !slices.Contains(s.cfg.IgnoreMetadataFor, TopicUpdateAccepted) {
			sar.Metadata = Metadata{Reported: updatedMeta}
		}
		s.notifyAccepted(thingId, sr.ClientToken, sar)
	}
	return ss, err
}

func (s *shadowSvc) HandleLocalPresence(ci connector.ClientInfo) {
	ctx := s.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	err := s.repo.UpdateConnStatus(ctx, []connector.ClientInfo{ci})
	if err != nil {
		slog.Error("update conn error", "clientId", ci.ClientId, "error", err)
	} else {
		s.cache.UpdateConnStatus(ci.ClientId, ci)
	}
}

func (s *shadowSvc) doFirstSyncStatus(ctx context.Context) error {
	now := time.Now()
	clients, err := s.connectorChecker.AllClientInfo()
	if err != nil {
		return errors.Wrap(err, "get all client info for sync conn status")
	}

	batch := 100
	for from, to := 0, batch; from < len(clients); from, to = to, to+batch {
		if to > len(clients) {
			to = len(clients)
		}
		l := clients[from:to]
		err := s.repo.UpdateConnStatus(ctx, l)
		if err != nil {
			return errors.Wrap(err, "update conn status")
		}
	}

	err = s.repo.UpdateAllConnStatusDisconnect(ctx, now)
	if err != nil {
		return errors.Wrap(err, "update all conn status disconnect")
	}

	return nil
}

func (s *shadowSvc) Create(ctx context.Context, thingId string) (Shadow, error) {
	ss := DefaultShadow(thingId)
	re, err := s.repo.Create(ctx, thingId, ss)
	if err != nil {
		return Shadow{}, err
	}
	slog.Info("Successfully created shadow", "thingId", thingId)
	s.NotifyCreated(thingId, ShadowWithEnable{Shadow: *re, Enabled: true})
	return *re, nil
}

func (s *shadowSvc) Query(ctx context.Context, pq model.PageQuery, query string) (Page, error) {
	var parsedQ ParsedQuerySql
	if query != "" {
		var err error
		parsedQ, err = parseQuerySql(query)
		if err != nil {
			return Page{}, errors.WithMessage(model.ErrInvalidParams, err.Error())
		}
	}

	p, err := s.repo.Query(ctx, pq, parsedQ)
	if err != nil {
		return Page{}, err
	}

	// no need to transform
	if len(parsedQ.OriginSelectAlias) == 0 {
		l := make([]any, len(p.Content))
		for i, r := range p.Content {
			l[i] = r
		}
		return Page{Total: p.Total, Content: l}, nil
	}

	// transform based on select

	mList, err := entityToMap(p.Content)
	if err != nil {
		return Page{}, err
	}
	resList := make([]any, len(mList))
	for i, r := range mList {
		t := transMap(r, parsedQ.OriginSelectAlias)
		resList[i] = t
	}

	resP := Page{Total: p.Total, Content: resList}
	return resP, nil
}

// Convert ShadowWithStatus to map
// Use json Marshal and Unmarshal to simplify it, although there is some loss of performance
func entityToMap(list []ShadowWithStatus) ([]map[string]any, error) {
	j, err := json.Marshal(list)
	if err != nil {
		return nil, err
	}
	res := make([]map[string]any, len(list))
	err = json.Unmarshal(j, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (s *shadowSvc) Get(ctx context.Context, thingId string) (ShadowWithStatus, error) {
	ss, err := s.repo.GetWithStatus(ctx, thingId)
	if err != nil {
		return ShadowWithStatus{}, err
	}
	if ss == nil {
		return ShadowWithStatus{}, model.ErrNotFound
	}
	return *ss, nil
}

func (s *shadowSvc) Delete(ctx context.Context, thingId string) error {
	err := s.repo.Delete(ctx, thingId)
	if err == nil {
		s.NotifyDeleted(thingId)
	}
	return err
}

func (s *shadowSvc) setState(
	ctx context.Context, thingId string,
	sr StateReq, isDesired bool) (Shadow, MetaValue, error) {

	var pre Shadow
	var persisted *Shadow
	var updatedMeta MetaValue
	var changed bool

	version := sr.Version
	err := s.repo.ExecWithTx(func(txtRepo Repo) error {
		ss, err := txtRepo.Get(ctx, thingId)
		if err != nil {
			return err
		}
		if ss == nil {
			return model.ErrNotFound
		}
		if version != 0 && ss.Version != version {
			return errors.Wrap(model.ErrVersionConflict,
				fmt.Sprintf("expect version %d but got %d", ss.Version, version))
		}

		pre = cloneShadow(*ss)

		var patch StateValue
		if isDesired {
			if sr.State.Desired == nil {
				return model.ErrShadowFormat
			}
			patch = sr.State.Desired
		} else {
			if sr.State.Reported == nil {
				return model.ErrShadowFormat
			}
			patch = sr.State.Reported
		}

		currentState := ss.State.Desired
		if !isDesired {
			currentState = ss.State.Reported
		}

		merged, didChange := MergePatch(currentState, patch)
		changed = didChange

		if !changed {
			persisted = ss
			return nil
		}

		updatedMeta = applyMergedState(ss, merged, isDesired, patch)
		ss.Version++

		persisted, err = txtRepo.Update(ctx, thingId, version, *ss)
		if err != nil {
			return err
		}

		s.cache.Del(thingId)
		return nil
	})
	if err != nil {
		return Shadow{}, nil, err
	}

	typ := StateTypeReported
	if isDesired {
		typ = StateTypeDesired
	}
	slog.Debug("Successfully set shadow", "type", typ, "thingId", thingId, "content", sr)

	s.notifyDeltaState(thingId, sr.ClientToken, persisted)
	s.notifyStateUpdate(thingId, sr.ClientToken, &pre, persisted)

	return *persisted, updatedMeta, nil
}

func cloneShadow(src Shadow) Shadow {
	dst := Shadow{
		ThingId:   src.ThingId,
		Version:   src.Version,
		CreatedAt: src.CreatedAt,
		UpdatedAt: src.UpdatedAt,
		Tags:      src.Tags,
		State:     NewStateDR(),
		Metadata:  NewMetadata(),
	}
	dst.State.Desired = cloneStateValue(src.State.Desired)
	dst.State.Reported = cloneStateValue(src.State.Reported)
	dst.Metadata.Desired = cloneMetaValue(src.Metadata.Desired)
	dst.Metadata.Reported = cloneMetaValue(src.Metadata.Reported)
	return dst
}

func cloneMetaValue(src MetaValue) MetaValue {
	if src == nil {
		return nil
	}
	return DeepCopyMap(src)
}

func applyMergedState(ss *Shadow, merged map[string]any, isDesired bool, patch StateValue) MetaValue {
	var updatedMeta MetaValue
	if isDesired {
		ss.State.Desired = merged
		ss.Metadata.Desired, updatedMeta = buildMetadata(patch, ss.Metadata.Desired)
	} else {
		ss.State.Reported = merged
		ss.Metadata.Reported, updatedMeta = buildMetadata(patch, ss.Metadata.Reported)
	}
	ss.UpdatedAt = time.Now()
	return updatedMeta
}

func buildMetadata(patch StateValue, existingMeta MetaValue) (MetaValue, MetaValue) {
	if existingMeta == nil {
		existingMeta = make(MetaValue)
	}
	updatedMeta := make(MetaValue)
	now := time.Now().UnixMilli()
	buildMetaRecursive(patch, existingMeta, updatedMeta, now)
	return existingMeta, updatedMeta
}

func buildMetaRecursive(patch map[string]any, allMeta, updatedMeta map[string]any, now int64) {
	for k, v := range patch {
		if v == nil {
			delete(allMeta, k)
			continue
		}
		if subPatch, ok := v.(map[string]any); ok {
			subAll, ok := allMeta[k].(map[string]any)
			if !ok {
				subAll = make(map[string]any)
				allMeta[k] = subAll
			}
			subUpdated := make(map[string]any)
			updatedMeta[k] = subUpdated
			buildMetaRecursive(subPatch, subAll, subUpdated, now)
			if len(subAll) == 0 {
				delete(allMeta, k)
				delete(updatedMeta, k)
			}
		} else {
			allMeta[k] = map[string]any{"timestamp": now}
			updatedMeta[k] = map[string]any{"timestamp": now}
		}
	}
}

func (s *shadowSvc) notifyStateUpdate(thingId, clientToken string, pre *Shadow, rs *Shadow) {
	notice := StateUpdatedNotice{
		Previous: StatePrevious{
			State:   StateDR{Desired: pre.State.Desired, Reported: pre.State.Reported},
			Version: pre.Version,
		},
		Current: StateCurrent{
			State:   StateDR{Desired: rs.State.Desired, Reported: rs.State.Reported},
			Version: rs.Version,
		},
		Timestamp:   time.Now().UnixMilli(),
		ClientToken: clientToken,
	}

	if !slices.Contains(s.cfg.IgnoreMetadataFor, TopicUpdateDocuments) {
		notice.Previous.Metadata = pre.Metadata
		notice.Current.Metadata = rs.Metadata
	}

	for _, f := range s.updateSubscribers {
		f(thingId, notice)
	}
}

func (s *shadowSvc) notifyDeltaState(thingId, clientToken string, rs *Shadow) {
	delta, deltaMeta := DeltaState(rs.State.Desired, rs.State.Reported, rs.Metadata.Desired)
	if IsStateValueEmpty(delta) {
		// ignore empty delta
		return
	}

	deltaNotice := DeltaStateNotice{
		State:       delta,
		Timestamp:   time.Now().UnixMilli(),
		ClientToken: clientToken,
		Version:     rs.Version,
	}
	if !slices.Contains(s.cfg.IgnoreMetadataFor, TopicUpdateDelta) {
		deltaNotice.Metadata = deltaMeta
	}

	for _, f := range s.deltaSubscribers {
		f(thingId, deltaNotice)
	}
}

func (s *shadowSvc) notifyAccepted(thingId, clientToken string, resp StateAcceptedResp) {
	for _, f := range s.acceptedSubscribers {
		f(thingId, StateAcceptedRespMsg{ThingId: thingId, Op: OpUpdate,
			Resp: resp,
		})
	}
}

func (s *shadowSvc) notifyRejected(thingId, clientToken string, err error) {
	res := ErrResp{ClientToken: clientToken, Timestamp: time.Now().UnixMilli()}
	var httpErr model.HttpErr
	if ok := errors.As(err, &httpErr); ok {
		res.Code = httpErr.Code
		res.Message = err.Error()
	} else {
		res.Code = 500
		res.Message = err.Error()
	}

	for _, f := range s.rejectedSubscribers {
		f(thingId, ErrRespMsg{ThingId: thingId, Op: OpUpdate, Resp: res})
	}
}

func (s *shadowSvc) SetTag(ctx context.Context, thingId string, t TagsReq) error {
	err := s.repo.ExecWithTx(func(txtRepo Repo) error {
		cur, err := txtRepo.Get(ctx, thingId)
		if err != nil {
			return err
		}
		if cur == nil {
			return model.ErrNotFound
		}
		if t.Version != 0 && cur.Version != t.Version {
			return errors.Wrap(model.ErrVersionConflict,
				fmt.Sprintf("expect version %d but got %d", cur.Version, t.Version))
		}

		mergedTags := MergeTags(cur.Tags, t.Tags)
		cur.Version++
		cur.Tags = mergedTags
		_, err = txtRepo.Update(ctx, thingId, t.Version, *cur)
		return err
	})
	if err != nil {
		return err
	}
	// update cache
	s.cache.UpdateTags(thingId, t.Tags)

	return nil
}

// ========= CacheService interface =========

func (s *shadowSvc) GetFromCache(thingId string) (ShadowWithStatus, bool) {
	if s, ok := s.cache.Get(thingId); ok {
		return s, true
	}
	ss, err := s.repo.GetWithStatus(context.Background(), thingId)
	if err == nil && ss != nil {
		// TODO: LRU cache
		s.cache.Set(thingId, *ss)
		return *ss, true
	}
	return ShadowWithStatus{}, false
}
func (s *shadowSvc) NotifyCreated(thingId string, sd ShadowWithEnable) {
	// do nothing
}
func (s *shadowSvc) NotifyDeleted(thingId string) {
	s.cache.Del(thingId)
}
func (s *shadowSvc) NotifyUpdate(thingId string, enable bool) {
	s.cache.updateThing(thingId, enable)
}

// ========= CacheService interface end =========

func cloneStateValue(src StateValue) StateValue {
	tgt := DeepCopyMap(src)
	return tgt
}

func cloneMetadata(src Metadata) Metadata {
	dst := Metadata{Desired: make(MetaValue), Reported: make(MetaValue)}
	tgt := DeepCopyMap(src.Desired)
	dst.Desired = tgt
	return dst
}

func DeepCopyMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	tgt := make(map[string]any, len(src))
	for k, v := range src {
		tgt[k] = cloneValue(v)
	}
	return tgt
}
