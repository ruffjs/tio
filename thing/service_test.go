package thing_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"ruff.io/tio/db/mock"
	"ruff.io/tio/shadow"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"ruff.io/tio/pkg/model"
	"ruff.io/tio/pkg/testutil"
	"ruff.io/tio/pkg/uuid"
	shadowMock "ruff.io/tio/shadow/mock"
	shadowWire "ruff.io/tio/shadow/wire"
	"ruff.io/tio/thing"
	"ruff.io/tio/thing/wire"
)

var (
	ctxTest = context.Background()
)

var connector = shadowMock.NewConnectivity()

type failingShadowSvc struct {
	setTagErr error
}

func (s failingShadowSvc) Init(context.Context) {}
func (s failingShadowSvc) SetDesired(context.Context, string, shadow.StateReq) (shadow.Shadow, error) {
	return shadow.Shadow{}, nil
}
func (s failingShadowSvc) SetReported(context.Context, string, shadow.StateReq) (shadow.Shadow, error) {
	return shadow.Shadow{}, nil
}
func (s failingShadowSvc) SubscribeUpdate(shadow.StateUpdateSubscribe) {}
func (s failingShadowSvc) SubscribeDelta(shadow.StateDeltaSubscribe)   {}
func (s failingShadowSvc) SubAccepted(shadow.StateAcceptedSubscribe)   {}
func (s failingShadowSvc) SubRejected(shadow.StateRejectedSubscribe)   {}
func (s failingShadowSvc) Create(context.Context, string) (shadow.Shadow, error) {
	return shadow.Shadow{}, nil
}
func (s failingShadowSvc) Delete(context.Context, string) error { return nil }
func (s failingShadowSvc) Query(context.Context, model.PageQuery, string) (shadow.Page, error) {
	return shadow.Page{}, nil
}
func (s failingShadowSvc) Get(context.Context, string) (shadow.ShadowWithStatus, error) {
	return shadow.ShadowWithStatus{}, nil
}
func (s failingShadowSvc) SetTag(context.Context, string, shadow.TagsReq) error { return s.setTagErr }
func (s failingShadowSvc) GetFromCache(string) (shadow.ShadowWithStatus, bool) {
	return shadow.ShadowWithStatus{}, false
}
func (s failingShadowSvc) NotifyCreated(string, shadow.ShadowWithEnable) {}
func (s failingShadowSvc) NotifyDeleted(string)                          {}
func (s failingShadowSvc) NotifyUpdate(string, bool)                     {}

func NewTestSvc() (thing.Service, shadow.Service) {
	db := mock.NewSqliteConnTest()
	_ = db.AutoMigrate(thing.Entity{}, shadow.Entity{}, &shadow.ConnStatusEntity{})
	shadowSvc := shadowWire.InitSvc(db, connector, shadow.Config{})
	thingSvc := wire.InitSvc(context.Background(), db, shadowSvc, connector)
	return thingSvc, shadowSvc
}

func NewTestSvcWithDB() (thing.Service, shadow.Service, *gorm.DB) {
	db := mock.NewSqliteConnTest()
	_ = db.AutoMigrate(thing.Entity{}, shadow.Entity{}, &shadow.ConnStatusEntity{})
	shadowSvc := shadowWire.InitSvc(db, connector, shadow.Config{})
	thingSvc := wire.InitSvc(context.Background(), db, shadowSvc, connector)
	return thingSvc, shadowSvc, db
}

func TestThingSvc_Create(t *testing.T) {
	svc, sdSvc := NewTestSvc()
	th := thing.Thing{}
	t.Run("create thing with no id", func(t *testing.T) {
		th.Id = ""
		resTh, err := svc.Create(ctxTest, th, nil, false)
		require.NoError(t, err)
		require.NotEmpty(t, resTh.Id, "thing id is empty")

		sd, err := sdSvc.Get(ctxTest, resTh.Id)
		require.NoError(t, err)
		require.Equal(t, resTh.Id, sd.ThingId)
	})

	t.Run("create thing with type isGateway=true and disabled", func(t *testing.T) {
		th.Id = ""
		th.IsGateway = true
		th.Enabled = false
		resTh, err := svc.Create(ctxTest, th, nil, false)
		require.NoError(t, err)
		require.Equal(t, th.IsGateway, resTh.IsGateway, "thing type isGateway")
		require.Equal(t, th.Enabled, resTh.Enabled, "thing enabled")

		sd, err := sdSvc.Get(ctxTest, resTh.Id)
		require.NoError(t, err)
		require.Equal(t, resTh.Id, sd.ThingId)
	})

	t.Run("create thing with type isGateway=false and enabled", func(t *testing.T) {
		th.Id = ""
		th.IsGateway = false
		th.Enabled = true
		resTh, err := svc.Create(ctxTest, th, nil, false)
		require.NoError(t, err)
		require.Equal(t, th.IsGateway, resTh.IsGateway, "thing type isGateway")
		require.Equal(t, th.Enabled, resTh.Enabled, "thing enabled")

		sd, err := sdSvc.Get(ctxTest, resTh.Id)
		require.NoError(t, err)
		require.Equal(t, resTh.Id, sd.ThingId)
	})

	t.Run("create thing with duplicates", func(t *testing.T) {
		pre, err := svc.Create(ctxTest, th, nil, false)
		require.NoError(t, err)
		th.Id = pre.Id
		_, err = svc.Create(ctxTest, th, nil, false)
		require.ErrorIs(t, err, model.ErrDuplicated, "should have conflict error")
	})

	t.Run("create thing with same id after delete", func(t *testing.T) {
		th.Id = ""
		pre, err := svc.Create(ctxTest, th, nil, false)
		require.NoError(t, err)
		connDelCall := connector.On("Close", pre.Id).Return(nil)
		removeCall := connector.On("Remove", pre.Id).Return(nil)
		defer connDelCall.Unset()
		defer removeCall.Unset()

		err = svc.Delete(ctxTest, pre.Id)
		require.NoError(t, err)

		th.Id = pre.Id
		_, err = svc.Create(ctxTest, th, nil, false)
		require.NoError(t, err, "should have no error when creating duplicates with the same id as th deleted thing")
	})

	t.Run("create thing with password", func(t *testing.T) {
		th.Id = ""
		th.AuthValue = "password-xxx"
		re, err := svc.Create(ctxTest, th, nil, false)
		require.NoError(t, err, "should have created with password")
		require.Equal(t, th.AuthValue, re.AuthValue, "should use password in request")
	})

	t.Run("create thing with upsert", func(t *testing.T) {
		th.Id = ""
		th.AuthValue = "password-xxx"
		res, err := svc.Create(ctxTest, th, nil, false)
		require.NoError(t, err, "should have created with password")

		th.Id = res.Id
		th.AuthValue = "password-yyy"
		re, err := svc.Create(ctxTest, th, nil, true)
		require.NoError(t, err, "should have created with password")
		require.Equal(t, th.AuthValue, re.AuthValue, "should use password in request")
	})
	t.Run("create thing with tags", func(t *testing.T) {
		th.Id = ""
		tags := map[string]any{"color": "red"}
		res, err := svc.Create(ctxTest, th, tags, false)
		require.NoError(t, err, "should have created with tags")

		sd, err := sdSvc.Get(ctxTest, res.Id)
		require.NoError(t, err)
		require.Equal(t, shadow.TagsValue(tags), sd.Tags, "should use tags in request")
	})
	t.Run("create thing with tags and upsert", func(t *testing.T) {
		th.Id = ""
		tags := map[string]any{"color": "red", "age": float64(10)}
		res, err := svc.Create(ctxTest, th, tags, false)
		require.NoError(t, err, "should have created with tags")

		th.Id = res.Id
		tags = map[string]any{"color": "blue", "size": "large"}
		_, err = svc.Create(ctxTest, th, tags, true)
		require.NoError(t, err, "should have created with tags")

		sd, err := sdSvc.Get(ctxTest, res.Id)
		require.NoError(t, err)
		resTags := shadow.TagsValue{
			"color": "blue",
			"size":  "large",
			"age":   float64(10),
		}
		require.Equal(t, resTags, sd.Tags, "should use tags in request")
	})
}

func TestThingSvc_Update(t *testing.T) {
	svc, _ := NewTestSvc()
	th := thing.Thing{Id: "for-update-test"}
	_, err := svc.Create(ctxTest, th, nil, false)
	require.NoError(t, err)
	t.Run("Disable thing", func(t *testing.T) {
		connDelCall := connector.On("Close", th.Id).Return(nil).Times(1)
		defer connDelCall.Unset()
		svc.Update(ctxTest, th.Id, thing.ThingPatch{Enabled: model.Ref(false)})
		connDelCall.Parent.AssertExpectations(t)
		en, err := svc.Get(ctxTest, th.Id)
		require.NoError(t, err)
		require.Equal(t, en.Enabled, false)
	})
	t.Run("Enable thing", func(t *testing.T) {
		connDelCall := connector.On("Close", th.Id).Return(nil).Times(0)
		defer connDelCall.Unset()
		svc.Update(ctxTest, th.Id, thing.ThingPatch{Enabled: model.Ref(true)})
		connDelCall.Parent.AssertExpectations(t)
		en, err := svc.Get(ctxTest, th.Id)
		require.NoError(t, err)
		require.Equal(t, en.Enabled, true)
	})
}

func TestThingSvc_Delete(t *testing.T) {
	svc, sdSvc := NewTestSvc()
	randId, _ := uuid.New().ID()

	connector.On("Close", randId).Return(nil).Twice()
	connector.On("Remove", randId).Return(nil).Twice()

	err := svc.Delete(ctxTest, randId)
	require.NoError(t, err, "should no error when not found")

	_, _ = svc.Create(ctxTest, thing.Thing{Id: randId}, nil, false)
	err = svc.Delete(ctxTest, randId)
	require.NoError(t, err)
	_, err = sdSvc.Get(ctxTest, randId)
	require.Error(t, err, "shadow should get not found error when thing is deleted")
	if herr, ok := err.(model.HttpErr); ok {
		require.Equal(t, herr.HttpCode, 404)
	} else {
		t.Fatalf("error should by model.HttpErr : %v", err)
	}

	connector.AssertExpectations(t)
}

func TestThingSvc_Get(t *testing.T) {
	svc, _ := NewTestSvc()
	randId, _ := uuid.New().ID()
	_, err := svc.Get(ctxTest, randId)
	isNotFound := errors.Is(err, model.ErrNotFound)
	require.True(t, isNotFound, "error should be thing.NotFoundErr")

	_, _ = svc.Create(ctxTest, thing.Thing{Id: randId}, nil, false)

	pq := thing.PageQuery{
		WithAuthValue: true,
		PageQuery:     model.PageQuery{PageIndex: 0, PageSize: 10},
	}
	page, err := svc.Query(ctxTest, pq)
	require.NoError(t, err)
	require.LessOrEqual(t, int64(1), page.Total, "query page total count")
	require.LessOrEqual(t, 1, len(page.Content), "query page content count")
}

func TestThingSvc_QueryWithStatus(t *testing.T) {
	svc, _, db := NewTestSvcWithDB()
	randId, _ := uuid.New().ID()

	// Create a thing
	_, err := svc.Create(ctxTest, thing.Thing{Id: randId}, nil, false)
	require.NoError(t, err)

	now := time.Now()
	connectedAt := now.Add(-1 * time.Hour)
	disconnectedAt := now.Add(-30 * time.Minute)

	t.Run("query with WithStatus=true should return connection status", func(t *testing.T) {
		// Update connection status to connected
		err = db.Model(&shadow.ConnStatusEntity{}).
			Where("thing_id = ?", randId).
			Updates(map[string]any{
				"connected":       true,
				"connected_at":    &connectedAt,
				"disconnected_at": nil,
			}).Error
		require.NoError(t, err)

		pq := thing.PageQuery{
			WithStatus:    true,
			WithAuthValue: false,
			PageQuery:     model.PageQuery{PageIndex: 1, PageSize: 10},
		}
		page, err := svc.Query(ctxTest, pq)
		require.NoError(t, err)
		require.GreaterOrEqual(t, page.Total, int64(1))

		// Find the thing we created
		var foundThing *thing.ThingWithConnStatus
		for i := range page.Content {
			if page.Content[i].Id == randId {
				foundThing = &page.Content[i]
				break
			}
		}
		require.NotNil(t, foundThing, "should find the created thing")
		require.NotNil(t, foundThing.Connected, "Connected field should not be nil")
		require.True(t, *foundThing.Connected, "thing should be connected")
		require.NotNil(t, foundThing.ConnectedAt, "ConnectedAt should not be nil")
	})

	t.Run("query with WithStatus=false should not return connection status", func(t *testing.T) {
		pq := thing.PageQuery{
			WithStatus:    false,
			WithAuthValue: false,
			PageQuery:     model.PageQuery{PageIndex: 1, PageSize: 10},
		}
		page, err := svc.Query(ctxTest, pq)
		require.NoError(t, err)
		require.GreaterOrEqual(t, page.Total, int64(1))

		// Find the thing we created
		var foundThing *thing.ThingWithConnStatus
		for i := range page.Content {
			if page.Content[i].Id == randId {
				foundThing = &page.Content[i]
				break
			}
		}
		require.NotNil(t, foundThing, "should find the created thing")
		// When WithStatus=false, connection status fields should be zero values
		require.Nil(t, foundThing.Connected, "Connected should be nil when WithStatus=false")
		require.Nil(t, foundThing.ConnectedAt, "ConnectedAt should be nil when WithStatus=false")
		require.Nil(t, foundThing.DisconnectedAt, "DisconnectedAt should be nil when WithStatus=false")
	})

	t.Run("query with WithStatus=true for disconnected thing", func(t *testing.T) {
		// Update connection status to disconnected
		err = db.Model(&shadow.ConnStatusEntity{}).
			Where("thing_id = ?", randId).
			Updates(map[string]any{
				"connected":       false,
				"disconnected_at": &disconnectedAt,
			}).Error
		require.NoError(t, err)

		pq := thing.PageQuery{
			WithStatus:    true,
			WithAuthValue: false,
			PageQuery:     model.PageQuery{PageIndex: 1, PageSize: 10},
		}
		page, err := svc.Query(ctxTest, pq)
		require.NoError(t, err)

		// Find the thing we created
		var foundThing *thing.ThingWithConnStatus
		for i := range page.Content {
			if page.Content[i].Id == randId {
				foundThing = &page.Content[i]
				break
			}
		}
		require.NotNil(t, foundThing, "should find the created thing")
		require.NotNil(t, foundThing.Connected, "Connected field should not be nil")
		require.False(t, *foundThing.Connected, "thing should be disconnected")
		require.NotNil(t, foundThing.DisconnectedAt, "DisconnectedAt should not be nil when disconnected")
	})
}

func TestThingSvc_QueryWithEnabledFilter(t *testing.T) {
	svc, _, _ := NewTestSvcWithDB()
	enabledId, _ := uuid.New().ID()
	disabledId, _ := uuid.New().ID()

	_, err := svc.Create(ctxTest, thing.Thing{Id: enabledId, Enabled: true}, nil, false)
	require.NoError(t, err)
	_, err = svc.Create(ctxTest, thing.Thing{Id: disabledId, Enabled: false}, nil, false)
	require.NoError(t, err)

	pq := thing.PageQuery{
		Enabled:       model.Ref(false),
		WithAuthValue: true,
		PageQuery:     model.PageQuery{PageIndex: 1, PageSize: 10},
	}
	page, err := svc.Query(ctxTest, pq)
	require.NoError(t, err)
	require.Equal(t, 1, len(page.Content))
	require.Equal(t, disabledId, page.Content[0].Id)
	require.False(t, page.Content[0].Enabled)
}

func TestThingSvc_CreateUpsertPropagatesTagError(t *testing.T) {
	_, _, db := NewTestSvcWithDB()
	repo := thing.NewThingRepo(db)
	svc := thing.NewSvc(repo, uuid.New(), failingShadowSvc{setTagErr: fmt.Errorf("set tag failed")}, connector)

	id, _ := uuid.New().ID()
	_, err := svc.Create(ctxTest, thing.Thing{Id: id, Enabled: true}, nil, false)
	require.NoError(t, err)

	_, err = svc.Create(ctxTest, thing.Thing{Id: id, Enabled: true}, map[string]any{"color": "red"}, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "set tag failed")
}

func TestIdValid(t *testing.T) {
	for i := 0; i < 1000; i++ {
		idLen := rand.Intn(32)
		if idLen == 0 {
			continue
		}
		want := rand.Intn(2) == 0
		id := testutil.RandStr(idLen, testutil.LettersForId, testutil.LettersInvalidForId, want)
		got := thing.IdValid(id)
		require.Equal(t, want, got, "id=%q", id)
	}
}

func TestThingSvc_GatewayBind(t *testing.T) {
	exceedIds := make([]string, 0, 101)
	for i := 0; i < 101; i++ {
		id, _ := uuid.New().ID()
		exceedIds = append(exceedIds, id)
	}

	cases := []struct {
		name      string
		gwId      string
		thIds     []string
		unbindIds []string
		remainIds []string
		unbindAll bool
	}{
		{
			name:  "bind 1 thing to gateway",
			gwId:  "gateway-1",
			thIds: []string{"subthing-1"},
		},
		{
			name:  "bind multiple things to gateway",
			gwId:  "gateway-2",
			thIds: []string{"subthing-2", "subthing-3", "subthing-4"},
		},
		{
			name:      "unbind multiple things from gateway",
			gwId:      "gateway-3",
			thIds:     []string{"subthing-5", "subthing-6", "subthing-7"},
			unbindIds: []string{"subthing-7", "subthing-6"},
			remainIds: []string{"subthing-5"},
		},
		{
			name:      "unbind all things from gateway",
			gwId:      "gateway-4",
			thIds:     []string{"subthing-8", "subthing-9", "subthing-10"},
			unbindAll: true,
		},
	}

	svc, _ := NewTestSvc()

	for _, cas := range cases {
		t.Run(cas.name, func(t *testing.T) {
			gw, err := svc.Create(ctxTest, thing.Thing{Id: cas.gwId, IsGateway: true}, nil, false)
			require.NoError(t, err)
			for _, thId := range cas.thIds {
				_, err = svc.Create(ctxTest, thing.Thing{Id: thId, IsGateway: false}, nil, false)
				require.NoError(t, err, "create thing %q", thId)
			}
			err = svc.BindToGateway(ctxTest, cas.thIds, gw.Id)
			require.NoError(t, err)

			// checke bound
			l, err := svc.Query(ctxTest, thing.PageQuery{
				GatewayThingId: &gw.Id, PageQuery: model.PageQuery{PageIndex: 1, PageSize: 10}})
			require.NoError(t, err)
			require.Equal(t, len(cas.thIds), len(l.Content))
			for _, th := range l.Content {
				require.Contains(t, cas.thIds, th.Id)
			}

			if cas.unbindAll {
				err = svc.UnbindFromGateway(ctxTest, []string{}, gw.Id)
				require.NoError(t, err)
				// check
				l, err := svc.Query(ctxTest, thing.PageQuery{
					GatewayThingId: &gw.Id, PageQuery: model.PageQuery{PageIndex: 1, PageSize: 10}})
				require.NoError(t, err)
				require.Equal(t, 0, len(l.Content))
			} else if len(cas.unbindIds) > 0 {
				err = svc.UnbindFromGateway(ctxTest, cas.unbindIds, gw.Id)
				// check
				require.NoError(t, err, "unbind things %v from gateway %s", cas.unbindIds, gw.Id)
				l, err := svc.Query(ctxTest, thing.PageQuery{
					GatewayThingId: &gw.Id, PageQuery: model.PageQuery{PageIndex: 1, PageSize: 10}})
				require.NoError(t, err)
				require.Equal(t, len(cas.remainIds), len(l.Content))
				for _, thId := range l.Content {
					require.Contains(t, cas.remainIds, thId.Id)
				}
			}
		})
	}

	t.Run("bind exceed max things to gateway", func(t *testing.T) {
		gw, err := svc.Create(ctxTest, thing.Thing{Id: "gateway-exceed", IsGateway: true}, nil, false)
		require.NoError(t, err)
		for _, thId := range exceedIds {
			_, err := svc.Create(ctxTest, thing.Thing{Id: thId, IsGateway: false}, nil, false)
			require.NoError(t, err, "create thing %q", thId)
		}
		err = svc.BindToGateway(ctxTest, exceedIds, gw.Id)
		require.Error(t, err)
	})

	t.Run("bind gateway not exist", func(t *testing.T) {
		th, err := svc.Create(ctxTest, thing.Thing{Id: "th-for-nogw-1", IsGateway: false}, nil, false)
		require.NoError(t, err)
		err = svc.BindToGateway(ctxTest, []string{th.Id}, "not-exist-gateway")
		require.ErrorAs(t, err, &model.ErrNotFound, "should have found error")
	})
}
