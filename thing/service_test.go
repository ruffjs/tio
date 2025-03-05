package thing_test

import (
	"context"
	"math/rand"
	"testing"

	"ruff.io/tio/db/mock"
	"ruff.io/tio/shadow"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
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

func NewTestSvc() (thing.Service, shadow.Service) {
	db := mock.NewSqliteConnTest()
	_ = db.AutoMigrate(thing.Entity{}, shadow.Entity{}, &shadow.ConnStatusEntity{})
	shadowSvc := shadowWire.InitSvc(db, connector)
	thingSvc := wire.InitSvc(context.Background(), db, shadowSvc, connector)
	return thingSvc, shadowSvc
}

func TestThingSvc_Create(t *testing.T) {
	svc, sdSvc := NewTestSvc()
	th := thing.Thing{}
	t.Run("create thing with no id", func(t *testing.T) {
		th.Id = ""
		resTh, err := svc.Create(ctxTest, th, false)
		require.NoError(t, err)
		require.NotEmpty(t, resTh.Id, "thing id is empty")
		require.NotEmpty(t, resTh.AuthValue, "thing auth value is empty")

		sd, err := sdSvc.Get(ctxTest, resTh.Id)
		require.NoError(t, err)
		require.Equal(t, resTh.Id, sd.ThingId)
	})

	t.Run("create thing with type isGateway=true and disabled", func(t *testing.T) {
		th.Id = ""
		th.IsGateway = true
		th.Enabled = false
		resTh, err := svc.Create(ctxTest, th, false)
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
		resTh, err := svc.Create(ctxTest, th, false)
		require.NoError(t, err)
		require.Equal(t, th.IsGateway, resTh.IsGateway, "thing type isGateway")
		require.Equal(t, th.Enabled, resTh.Enabled, "thing enabled")

		sd, err := sdSvc.Get(ctxTest, resTh.Id)
		require.NoError(t, err)
		require.Equal(t, resTh.Id, sd.ThingId)
	})

	t.Run("create thing with duplicates", func(t *testing.T) {
		pre, err := svc.Create(ctxTest, th, false)
		require.NoError(t, err)
		th.Id = pre.Id
		_, err = svc.Create(ctxTest, th, false)
		require.ErrorIs(t, err, model.ErrDuplicated, "should have conflict error")
	})

	t.Run("create thing with same id after delete", func(t *testing.T) {
		th.Id = ""
		pre, err := svc.Create(ctxTest, th, false)
		require.NoError(t, err)
		connDelCall := connector.On("Close", pre.Id).Return(nil)
		removeCall := connector.On("Remove", pre.Id).Return(nil)
		defer connDelCall.Unset()
		defer removeCall.Unset()

		err = svc.Delete(ctxTest, pre.Id)
		require.NoError(t, err)

		th.Id = pre.Id
		_, err = svc.Create(ctxTest, th, false)
		require.NoError(t, err, "should have no error when creating duplicates with the same id as th deleted thing")
	})

	t.Run("create thing with password", func(t *testing.T) {
		th.Id = ""
		th.AuthValue = "password-xxx"
		re, err := svc.Create(ctxTest, th, false)
		require.NoError(t, err, "should have created with password")
		require.Equal(t, th.AuthValue, re.AuthValue, "should use password in request")
	})

	t.Run("create thing with upsert", func(t *testing.T) {
		th.Id = ""
		th.AuthValue = "password-xxx"
		res, err := svc.Create(ctxTest, th, false)
		require.NoError(t, err, "should have created with password")

		th.Id = res.Id
		th.AuthValue = "password-yyy"
		re, err := svc.Create(ctxTest, th, true)
		require.NoError(t, err, "should have created with password")
		require.Equal(t, th.AuthValue, re.AuthValue, "should use password in request")
	})
}

func TestThingSvc_Update(t *testing.T) {
	svc, _ := NewTestSvc()
	th := thing.Thing{Id: "for-update-test"}
	_, err := svc.Create(ctxTest, th, false)
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

	_, _ = svc.Create(ctxTest, thing.Thing{Id: randId}, false)
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

	_, _ = svc.Create(ctxTest, thing.Thing{Id: randId}, false)

	pq := thing.PageQuery{
		WithAuthValue: true,
		PageQuery:     model.PageQuery{PageIndex: 0, PageSize: 10},
	}
	page, err := svc.Query(ctxTest, pq)
	require.NoError(t, err)
	require.LessOrEqual(t, int64(1), page.Total, "query page total count")
	require.LessOrEqual(t, 1, len(page.Content), "query page content count")
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
			gw, err := svc.Create(ctxTest, thing.Thing{Id: cas.gwId, IsGateway: true}, false)
			require.NoError(t, err)
			for _, thId := range cas.thIds {
				_, err = svc.Create(ctxTest, thing.Thing{Id: thId, IsGateway: false}, false)
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
		gw, err := svc.Create(ctxTest, thing.Thing{Id: "gateway-exceed", IsGateway: true}, false)
		require.NoError(t, err)
		for _, thId := range exceedIds {
			_, err := svc.Create(ctxTest, thing.Thing{Id: thId, IsGateway: false}, false)
			require.NoError(t, err, "create thing %q", thId)
		}
		err = svc.BindToGateway(ctxTest, exceedIds, gw.Id)
		require.Error(t, err)
	})

	t.Run("bind gateway not exist", func(t *testing.T) {
		th, err := svc.Create(ctxTest, thing.Thing{Id: "th-for-nogw-1", IsGateway: false}, false)
		require.NoError(t, err)
		err = svc.BindToGateway(ctxTest, []string{th.Id}, "not-exist-gateway")
		require.ErrorAs(t, err, &model.ErrNotFound, "should have found error")
	})
}
