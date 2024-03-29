package shadow

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	"ruff.io/tio/connector"

	"github.com/pkg/errors"
	"ruff.io/tio/pkg/log"
	"ruff.io/tio/pkg/model"
)

const (
	StateTypeDesired  = "desired"
	StateTypeReported = "reported"
)

type Service interface {
	StateService
	CrudService
	TagsService
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
	SyncConnStatus(ctx context.Context) error
}

type StateDesiredSetter interface {
	SetDesired(ctx context.Context, thingId string, sr StateReq) (Shadow, error)
}

type CrudService interface {
	Create(ctx context.Context, thingId string) (Shadow, error)
	Delete(ctx context.Context, thingId string) error
	Query(ctx context.Context, page model.PageQuery, query string) (Page, error)
	Get(ctx context.Context, thingId string, opt GetOption) (ShadowWithStatus, error)
}

type TagsService interface {
	SetTag(ctx context.Context, thingId string, tag TagsReq) error
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
	Get(ctx context.Context, thingId string) (*ShadowWithEnable, error)
	Query(ctx context.Context, q model.PageQuery, query ParsedQuerySql) (model.PageData[ShadowWithStatus], error)

	UpdateConnStatus(ctx context.Context, s []connector.ClientInfo) error
	UpdateAllConnStatusDisconnect(ctx context.Context, updateTimeBefore time.Time) error
}

var _ Service = (*shadowSvc)(nil)

type shadowSvc struct {
	repo                Repo
	connectorChecker    connector.ConnectChecker
	updateSubscribers   []StateUpdateSubscribe
	deltaSubscribers    []StateDeltaSubscribe
	acceptedSubscribers []StateAcceptedSubscribe
	rejectedSubscribers []StateRejectedSubscribe
}

var svcSingleton *shadowSvc
var svcOnce sync.Once

func NewSvc(r Repo, a connector.ConnectChecker) Service {
	svcOnce.Do(func() {
		u := make([]StateUpdateSubscribe, 0)
		d := make([]StateDeltaSubscribe, 0)
		acp := make([]StateAcceptedSubscribe, 0)
		rjt := make([]StateRejectedSubscribe, 0)
		svcSingleton = &shadowSvc{
			repo:                r,
			connectorChecker:    a,
			updateSubscribers:   u,
			deltaSubscribers:    d,
			acceptedSubscribers: acp,
			rejectedSubscribers: rjt,
		}
	})
	return svcSingleton
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
			Metadata:    Metadata{Reported: updatedMeta},
			Timestamp:   time.Now().UnixMilli(),
			ClientToken: sr.ClientToken,
			Version:     ss.Version,
		}
		s.notifyAccepted(thingId, sr.ClientToken, sar)
	}
	return ss, err
}

func (s *shadowSvc) SyncConnStatus(ctx context.Context) error {
	if err := s.doFirstSyncStatus(ctx); err != nil {
		return err
	}
	connEventCh := s.connectorChecker.OnConnect()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case e := <-connEventCh:
				c := toClientInfo(e)
				err := s.repo.UpdateConnStatus(ctx, []connector.ClientInfo{c})
				if err != nil {
					log.Errorf("update conn for %s error: %v", c.ClientId, err)
				} else {
					log.Debugf("updated conn status %#v", c)
				}
			}
		}
	}()
	return nil
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
	ss := Shadow{
		ThingId:  thingId,
		State:    NewStateDR(),
		Metadata: Metadata{},
		Version:  1,
	}
	re, err := s.repo.Create(ctx, thingId, ss)
	if err != nil {
		return Shadow{}, err
	}
	log.Infof("Successfully created shadow %s", thingId)
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
func entityToMap(list []ShadowWithStatus) ([]map[string]interface{}, error) {
	j, err := json.Marshal(list)
	if err != nil {
		return nil, err
	}
	res := make([]map[string]interface{}, len(list))
	err = json.Unmarshal(j, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (s *shadowSvc) Get(ctx context.Context, thingId string, opt GetOption) (ShadowWithStatus, error) {
	ss, err := s.repo.Get(ctx, thingId)
	if err != nil {
		return ShadowWithStatus{}, err
	}
	if ss == nil {
		return ShadowWithStatus{}, model.ErrNotFound
	}
	res := ShadowWithStatus{Shadow: ss.Shadow, Enabled: ss.Enabled}
	if opt.WithStatus {
		ci, err := s.connectorChecker.ClientInfo(thingId)
		if err == nil {
			res.Connected = &ci.Connected
			res.ConnectedAt = ci.ConnectedAt
			res.DisconnectedAt = ci.DisconnectedAt
			res.RemoteAddr = ci.RemoteAddr
		}
	}
	return res, nil
}

func (s *shadowSvc) Delete(ctx context.Context, thingId string) error {
	return s.repo.Delete(ctx, thingId)
}

func (s *shadowSvc) setState(
	ctx context.Context, thingId string,
	sr StateReq, isDesired bool) (Shadow, MetaValue, error) {

	resCh := make(chan struct {
		pre Shadow
		cur Shadow
		me  MetaValue
	}, 1)
	version := sr.Version
	err := s.repo.ExecWithTx(func(txtRepo Repo) error {
		// match version
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

		// merge shadow

		pre := Shadow{
			ThingId:   ss.ThingId,
			Version:   ss.Version,
			CreatedAt: ss.CreatedAt,
			UpdatedAt: ss.UpdatedAt,
			Metadata:  NewMetadata(),
			State:     NewStateDR(),
		}
		// copy to pre
		pre.State.Desired = cloneStateValue(ss.State.Desired)
		pre.State.Reported = cloneStateValue(ss.State.Reported)
		pre.Metadata = cloneMetadata(ss.Metadata)

		var updatedMeta MetaValue
		if isDesired {
			if sr.State.Desired == nil {
				return model.ErrShadowFormat
			}
			MergeState(&ss.State.Desired, sr.State.Desired, &ss.Metadata.Desired, &updatedMeta)
		} else {
			if sr.State.Reported == nil {
				return model.ErrShadowFormat
			}
			MergeState(&ss.State.Reported, sr.State.Reported, &ss.Metadata.Reported, &updatedMeta)
		}

		// update

		ss.Version++
		reS, err := txtRepo.Update(ctx, thingId, version, ss.Shadow)
		if err != nil {
			return err
		}

		resCh <- struct {
			pre Shadow
			cur Shadow
			me  MetaValue
		}{pre: pre, cur: *reS, me: updatedMeta}

		return nil
	})
	if err != nil {
		return Shadow{}, nil, err
	}
	re := <-resCh
	preShadow, resShadow, resMeta := re.pre, re.cur, re.me

	typ := StateTypeReported
	if isDesired {
		typ = StateTypeDesired
	}
	log.Infof("Successfully set shadow %s, %s, content %#v", typ, thingId, sr)

	// notify regardless of whether there is a field update or not.
	s.notifyDeltaState(thingId, sr.ClientToken, &resShadow)
	s.notifyStateUpdate(thingId, sr.ClientToken, &preShadow, &resShadow)

	return resShadow, resMeta, nil
}

func (s *shadowSvc) notifyStateUpdate(thingId, clientToken string, pre *Shadow, rs *Shadow) {
	for _, f := range s.updateSubscribers {
		f(thingId, StateUpdatedNotice{
			Previous: StatePrevious{
				State:   StateDR{Desired: pre.State.Desired, Reported: pre.State.Reported},
				Version: pre.Version, Metadata: pre.Metadata,
			},
			Current: StateCurrent{
				State:   StateDR{Desired: rs.State.Desired, Reported: rs.State.Reported},
				Version: rs.Version, Metadata: rs.Metadata,
			},
			Timestamp:   time.Now().UnixMilli(),
			ClientToken: clientToken,
		})
	}
}

func (s *shadowSvc) notifyDeltaState(thingId, clientToken string, rs *Shadow) {
	delta, deltaMeta := DeltaState(rs.State.Desired, rs.State.Reported, rs.Metadata.Desired)
	if IsStateValueEmpty(delta) {
		// ignore empty delta
		return
	}

	for _, f := range s.deltaSubscribers {
		f(thingId, DeltaStateNotice{
			State:       delta,
			Metadata:    deltaMeta,
			Timestamp:   time.Now().UnixMilli(),
			ClientToken: clientToken,
			Version:     rs.Version,
		})
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

		mergerShadow := MergeTags(cur.Tags, t.Tags)
		cur.Version++
		cur.Tags = mergerShadow
		_, err = txtRepo.Update(ctx, thingId, t.Version, cur.Shadow)
		return err
	})
	if err != nil {
		return err
	}

	return nil
}

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
	tgt := make(map[string]any)
	for k, v := range src {
		if v == nil {
			tgt[k] = v
			continue
		}
		switch reflect.TypeOf(v).Kind() {
		case reflect.Map:
			vm, ok := v.(map[string]any)
			if !ok {
				log.Fatalf("deepCopyMap: %v is not a map[string]any", v)
				continue
			}
			tgt[k] = DeepCopyMap(vm)
		default:
			tgt[k] = v
		}
	}
	return tgt
}

func toClientInfo(e connector.PresenceEvent) connector.ClientInfo {
	conn := false
	if e.EventType == connector.EventConnected {
		conn = true
	}
	t := time.UnixMilli(e.Timestamp)
	c := connector.ClientInfo{
		ClientId:         e.ThingId,
		Connected:        conn,
		DisconnectReason: e.DisconnectReason,
		RemoteAddr:       e.RemoteAddr,
	}
	if conn {
		c.ConnectedAt = &t
	} else {
		c.DisconnectedAt = &t
	}
	return c
}
