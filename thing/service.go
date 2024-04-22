package thing

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"ruff.io/tio/connector"

	"github.com/pkg/errors"
	"ruff.io/tio"
	"ruff.io/tio/pkg/cache"
	"ruff.io/tio/pkg/log"
	"ruff.io/tio/pkg/model"
	"ruff.io/tio/shadow"
)

type Service interface {
	Create(ctx context.Context, th Thing) (Thing, error)
	Update(ctx context.Context, id string, tu ThingPatch) error
	Delete(ctx context.Context, id string) error
	Query(ctx context.Context, pq PageQuery) (Page, error)
	Get(ctx context.Context, id string) (*Thing, error)
	Exist(ctx context.Context, id string) (bool, error)

	// Binding a thing to a gateway thing means that the gateway has full authority
	// to communicate with the tio on behalf of the device
	// One thing can only be bound to one gateway
	// One gateway can bind multiple things
	BindToGateway(ctx context.Context, id, GatewayThingId string) error
	UnbindFromGateway(ctx context.Context, id string) error
	IsBoundGateway(ctx context.Context, thingId, gatewayThingId string) (bool, error)
}

type Page = model.PageData[ThingWithStatus]

type PageQuery struct {
	Enabled       *bool `json:"enabled"`
	WithAuthValue bool  `json:"withAuthValue"`
	WithStatus    bool  `json:"withStatus"`
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

func (t *thingSvc) Create(ctx context.Context, th Thing) (Thing, error) {
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
			return Thing{}, model.ErrDuplicated
		}
	}
	if th.AuthType == "" {
		th.AuthType = AuthTypePassword
	}
	if th.AuthType == AuthTypePassword && th.AuthValue == "" {
		s, err := t.idProvider.ID()
		if err != nil {
			return Thing{}, errors.Wrap(err, "secret generate")
		}
		th.AuthValue = s
	}
	res, err := t.repo.Create(ctx, th)
	if err != nil {
		return Thing{}, err
	}

	return res, err
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
		log.Errorf("Failed to close thing connector client, thingId=%q : %v", id, err)
	}
	return nil
}

func (t *thingSvc) Query(ctx context.Context, pq PageQuery) (Page, error) {
	p, err := t.repo.Query(ctx, pq)
	if err != nil {
		return Page{}, err
	}
	rp := t.toPage(p, pq.WithStatus)
	return rp, nil
}

func (t *thingSvc) toPage(p model.PageData[Thing], withStatus bool) Page {
	rp := Page{
		Total:   p.Total,
		Content: make([]ThingWithStatus, len(p.Content)),
	}
	for i, pi := range p.Content {
		rpi := &rp.Content[i]
		rpi.Thing = pi
		if !withStatus {
			continue
		}
		c, err := t.connector.ClientInfo(pi.Id)
		if err == nil {
			rpi.Connected = &c.Connected
			rpi.ConnectedAt = c.ConnectedAt
			rpi.DisconnectedAt = c.DisconnectedAt
			rpi.RemoteAddr = c.RemoteAddr
		}
	}
	return rp
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

func (t *thingSvc) BindToGateway(ctx context.Context, id string, gatewayThingId string) error {
	if id == gatewayThingId {
		return fmt.Errorf("the binding gatewayThingId cannot be the current thingId")
	}
	if id == "" || gatewayThingId == "" {
		return fmt.Errorf("thingId or gatewayThingId cannot be empty")
	}

	th, err := t.repo.Get(ctx, id)
	if err != nil {
		return errors.Errorf("get thing %q", id)
	}
	if th == nil {
		return errors.WithMessagef(model.ErrNotFound, "thing %q", id)
	}
	if th.IsGateway {
		return errors.WithMessage(model.ErrInvalidParams, "gateway thing cannot bind to another gateway")
	}

	gw, err := t.repo.Get(ctx, gatewayThingId)
	if err != nil {
		return errors.Errorf("get thing %q", id)
	}
	if gw == nil {
		return errors.WithMessagef(model.ErrNotFound, "thing %q", id)
	}
	if !gw.IsGateway {
		return errors.WithMessage(model.ErrInvalidParams, "only gateway thing can be the target to bind")
	}

	err = t.repo.Update(ctx, id, thingPatch{GatewayThingId: &gatewayThingId})
	t.delBoundCache(id)
	return err
}

func (t *thingSvc) UnbindFromGateway(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("thingId cannot be empty")
	}

	th, err := t.repo.Get(ctx, id)
	if err != nil {
		return errors.Errorf("get thing %q", id)
	}
	if th == nil {
		return errors.WithMessagef(model.ErrNotFound, "thing %q", id)
	}
	if th.IsGateway {
		return errors.WithMessage(model.ErrInvalidParams, "gateway thing cannot unbind")
	}

	gw := ""
	err = t.repo.Update(ctx, id, thingPatch{GatewayThingId: &gw})
	t.delBoundCache(id)
	return err
}

var gatewayBindCache = cache.New(time.Minute*5, time.Minute*1)

type gatewayBindCacheItem struct {
	thingId        string
	gatewayThingId string
}

func (t *thingSvc) IsBoundGateway(ctx context.Context, thingId, gatewayThingId string) (bool, error) {
	th, err := t.Get(ctx, thingId)
	if err != nil {
		return false, errors.WithMessagef(err, "get thing %s", thingId)
	}
	res := false

	bound := th.GatewayThingId == gatewayThingId

	bindedGw := ""
	if bound {
		bindedGw = gatewayThingId
		res = true
	} else {
		res = false
	}
	gatewayBindCache.Set(thingId, gatewayBindCacheItem{thingId: thingId, gatewayThingId: bindedGw}, cache.DefaultExpiration)

	return res, nil
}

func (t *thingSvc) delBoundCache(thingId string) {
	gatewayBindCache.Delete(thingId)
}

var idRegexp = regexp.MustCompile("^[0-9a-zA-Z_-]+$")

func IdValid(id string) bool {
	return idRegexp.MatchString(id)
}
