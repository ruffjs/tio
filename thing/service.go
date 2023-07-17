package thing

import (
	"context"
	"fmt"
	"regexp"
	"ruff.io/tio/connector"
	"strings"

	"github.com/pkg/errors"
	"ruff.io/tio"
	"ruff.io/tio/config"
	"ruff.io/tio/pkg/log"
	"ruff.io/tio/pkg/model"
	"ruff.io/tio/shadow"
)

type Service interface {
	Create(ctx context.Context, th Thing) (Thing, error)
	Delete(ctx context.Context, id string) error
	Query(ctx context.Context, pq PageQuery) (Page, error)
	Get(ctx context.Context, id string) (*Thing, error)
	Exist(ctx context.Context, id string) (bool, error)
}

type Page = model.PageData[ThingWithStatus]

type PageQuery struct {
	WithAuthValue bool `json:"withAuthValue"`
	WithStatus    bool `json:"withStatus"`
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

func TopicAcl(superUsers []config.UserPassword, thingId string, topic string, write bool) bool {
	for _, u := range superUsers {
		if u.Name == thingId {
			return true
		}
	}
	thingTopicPrefix := shadow.TopicThingsPrefix + thingId + "/"
	userThingTopicPrefix := shadow.TopicUserThingsPrefix + thingId + "/"
	if strings.HasPrefix(topic, thingTopicPrefix) || strings.HasPrefix(topic, userThingTopicPrefix) {
		return true
	}
	return false
}

var idRegexp = regexp.MustCompile("^[0-9a-zA-Z_-]+$")

func IdValid(id string) bool {
	return idRegexp.MatchString(id)
}
