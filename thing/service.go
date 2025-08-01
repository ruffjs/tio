package thing

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"time"

	"ruff.io/tio/connector"

	"github.com/pkg/errors"
	"ruff.io/tio"
	"ruff.io/tio/pkg/cache"
	"ruff.io/tio/pkg/model"
	"ruff.io/tio/shadow"
)

type Service interface {
	Create(ctx context.Context, th Thing, tags shadow.TagsValue, upsert bool) (Thing, error)
	Update(ctx context.Context, id string, tu ThingPatch) error
	Delete(ctx context.Context, id string) error
	Query(ctx context.Context, pq PageQuery) (Page, error)
	Get(ctx context.Context, id string) (*Thing, error)
	Exist(ctx context.Context, id string) (bool, error)

	// Binding a thing to a gateway thing means that the gateway has full authority
	// to communicate with the tio on behalf of the device
	// One thing can only be bound to one gateway
	// One gateway can bind multiple things
	BindToGateway(ctx context.Context, thingIds []string, gatewayThingId string) error
	UnbindFromGateway(ctx context.Context, thingIds []string, gatewayThingId string) error
	IsBoundGateway(ctx context.Context, thingId, gatewayThingId string) (bool, error)
}

type Page = model.PageData[Thing]

type PageQuery struct {
	Enabled        *bool   `json:"enabled"`
	IsGateway      *bool   `json:"isGateway"`
	GatewayThingId *string `json:"gatewayThingId"`
	WithAuthValue  bool    `json:"withAuthValue"`
	model.PageQuery
}

type thingSvc struct {
	repo       Repo
	idProvider tio.IdProvider
	shadowSvc  shadow.Service
	connector  connector.Connectivity
}

var _ Service = (*thingSvc)(nil)

func NewSvc(repo Repo, idProvider tio.IdProvider, ss shadow.Service, connector connector.Connectivity) Service {
	return &thingSvc{repo: repo, idProvider: idProvider, shadowSvc: ss, connector: connector}
}

func (t *thingSvc) Create(ctx context.Context, th Thing, tags shadow.TagsValue, upsert bool) (Thing, error) {
	exist := false
	if th.Id == "" {
		id, err := t.idProvider.ID()
		if err != nil {
			return Thing{}, errors.Wrap(err, "id generate")
		}
		th.Id = id
	} else {
		if !IdValid(th.Id) {
			return Thing{}, errors.WithMessagef(model.ErrInvalidParams, "id %q", th.Id)
		}

		old, err := t.repo.Get(ctx, th.Id)
		if err != nil {
			return Thing{}, errors.Wrap(err, "get thing "+th.Id)
		}
		if old != nil {
			exist = true
			if !upsert {
				return Thing{}, model.ErrDuplicated
			}
		}
	}
	if th.AuthType == "" {
		th.AuthType = AuthTypePassword
	}

	var res Thing
	var err error
	// TODO optimize: in one transaction
	if upsert && exist {
		err = t.repo.Update(ctx, th.Id, thingPatch{
			AuthType:       &th.AuthType,
			AuthValue:      &th.AuthValue,
			Enabled:        &th.Enabled,
			GatewayThingId: &th.GatewayThingId,
		})
		if err != nil {
			return Thing{}, err
		}
		if tags != nil {
			t.shadowSvc.SetTag(ctx, th.Id, shadow.TagsReq{Tags: tags})
		}
		n, err := t.repo.Get(ctx, th.Id)
		if err != nil {
			return Thing{}, err
		}
		res = *n
	} else {
		res, err = t.repo.Create(ctx, th, tags)
		if err != nil {
			return Thing{}, err
		}
		// notify shadow service
		t.shadowSvc.NotifyCreated(th.Id, shadow.ShadowWithEnable{Shadow: shadow.DefaultShadow(th.Id), Enabled: true})
	}

	return res, nil
}

func (t *thingSvc) Update(ctx context.Context, id string, tu ThingPatch) error {
	if ok, err := t.repo.Exist(ctx, id); err != nil {
		return err
	} else if !ok {
		return errors.WithMessagef(model.ErrNotFound, "thing %q", id)
	}
	patch := thingPatch{Enabled: tu.Enabled}

	if err := t.repo.Update(ctx, id, patch); err != nil {
		return err
	} else if tu.Enabled != nil && !*tu.Enabled {
		t.connector.Close(id)
	}
	return nil
}

func (t *thingSvc) Delete(ctx context.Context, id string) error {
	err := t.repo.Delete(ctx, id)
	if err != nil {
		return err
	}
	err = t.connector.Remove(id)
	if err != nil {
		slog.Error("Failed to close thing connector client", "id", id, "error", err)
	}
	// notify shadow service
	t.shadowSvc.NotifyDeleted(id)
	return nil
}

func (t *thingSvc) Query(ctx context.Context, pq PageQuery) (Page, error) {
	p, err := t.repo.Query(ctx, pq)
	if err != nil {
		return Page{}, err
	}
	return p, nil
}

func (t *thingSvc) Get(ctx context.Context, id string) (*Thing, error) {
	ch, err := t.repo.Get(ctx, id)
	if err != nil {
		return ch, err
	}
	if ch == nil {
		err = errors.Wrap(model.ErrNotFound, fmt.Sprintf("thingId %s", id))
		return nil, err
	} else {
		return ch, err
	}
}

func (t *thingSvc) Exist(ctx context.Context, id string) (bool, error) {
	e, err := t.repo.Exist(ctx, id)
	if err != nil {
		return false, errors.Wrapf(err, "check thing %q exist from db", id)
	}
	return e, err
}

func (t *thingSvc) BindToGateway(ctx context.Context, thingIds []string, gatewayThingId string) error {
	if len(thingIds) == 0 {
		return fmt.Errorf("thingId cannot be empty")
	}
	if slices.Contains(thingIds, gatewayThingId) {
		return fmt.Errorf("the binding gatewayThingId cannot be the current thingId")
	}
	if len(thingIds) == 0 || gatewayThingId == "" {
		return fmt.Errorf("thingId or gatewayThingId cannot be empty")
	}

	gw, err := t.repo.Get(ctx, gatewayThingId)
	if err != nil {
		return errors.Errorf("get thing %q", gatewayThingId)
	}
	if gw == nil {
		return errors.WithMessagef(model.ErrNotFound, "thing %q", gatewayThingId)
	}
	if !gw.IsGateway {
		return errors.WithMessage(model.ErrInvalidParams, "only gateway thing can be the target to bind")
	}

	if err := t.validBindThings(ctx, thingIds); err != nil {
		return err
	}

	err = t.repo.UpdateBatch(ctx, thingIds, thingPatch{GatewayThingId: &gatewayThingId})
	for _, id := range thingIds {
		t.delBoundCache(id)
	}
	return err
}

func (t *thingSvc) UnbindFromGateway(ctx context.Context, thingIds []string, gatewayThingId string) error {
	if err := t.validBindThings(ctx, thingIds); err != nil {
		return err
	}
	ids := thingIds
	// If gatewayThingId is empty, unbind all things
	if len(thingIds) == 0 {
		l, err := t.repo.Query(ctx, PageQuery{GatewayThingId: &gatewayThingId,
			PageQuery: model.PageQuery{PageIndex: 1, PageSize: 100}})
		if err != nil {
			return errors.WithMessage(err, "query gateway bound things")
		}
		if len(l.Content) == 0 {
			return nil
		}
		for _, th := range l.Content {
			ids = append(ids, th.Id)
		}
	}

	gw := ""
	err := t.repo.UpdateBatch(ctx, ids, thingPatch{GatewayThingId: &gw})
	for _, id := range ids {
		t.delBoundCache(id)
	}
	return err
}

func (t *thingSvc) validBindThings(ctx context.Context, thingIds []string) error {
	if len(thingIds) > MaxBindThings {
		return fmt.Errorf("things count cannot exceed %d", MaxBindThings)
	}
	ths, err := t.repo.GetBatch(ctx, thingIds)
	if err != nil {
		return fmt.Errorf("get things %q", thingIds)
	}
	for _, th := range ths {
		if th.IsGateway {
			return fmt.Errorf("gateway thing cannot unbind from or bind to another gateway")
		}
	}
	if len(ths) != len(thingIds) {
		var notfound []string
		for _, id := range thingIds {
			found := false
			for _, th := range ths {
				if id == th.Id {
					found = true
					break
				}
			}
			if !found {
				notfound = append(notfound, id)
			}
		}
		return errors.WithMessagef(model.ErrNotFound, "things %q", notfound)
	}

	return nil
}

// Memory cache for binding relation
var gatewayBindCache = cache.New(time.Minute*5, time.Minute*1)

func (t *thingSvc) IsBoundGateway(ctx context.Context, thingId, gatewayThingId string) (bool, error) {
	if gw, ok := gatewayBindCache.Get(thingId); ok {
		if gw == gatewayThingId {
			return true, nil
		}
		return false, nil
	}

	th, err := t.Get(ctx, thingId)
	boundGw := ""
	if err != nil {
		if !errors.Is(err, model.ErrNotFound) {
			return false, errors.WithMessagef(err, "get thing %s", thingId)
		}
	} else if th != nil {
		boundGw = th.GatewayThingId
	}
	res := false

	bound := boundGw == gatewayThingId

	if bound {
		boundGw = gatewayThingId
		res = true
	} else {
		res = false
	}
	gatewayBindCache.Set(thingId, boundGw, cache.DefaultExpiration)

	return res, nil
}

func (t *thingSvc) delBoundCache(thingId string) {
	gatewayBindCache.Delete(thingId)
}

var idRegexp = regexp.MustCompile("^[0-9a-zA-Z_-]+$")

func IdValid(id string) bool {
	return idRegexp.MatchString(id)
}
