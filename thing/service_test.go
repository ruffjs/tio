package thing_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"ruff.io/tio/config"
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
		resTh, err := svc.Create(ctxTest, th)
		require.NoError(t, err)
		require.NotEmpty(t, resTh.Id, "thing id is empty")
		require.NotEmpty(t, resTh.AuthValue, "thing auth value is empty")

		sd, err := sdSvc.Get(ctxTest, resTh.Id, shadow.GetOption{WithStatus: false})
		require.NoError(t, err)
		require.Equal(t, resTh.Id, sd.ThingId)
	})

	t.Run("create thing with duplicates", func(t *testing.T) {
		pre, err := svc.Create(ctxTest, th)
		require.NoError(t, err)
		th.Id = pre.Id
		_, err = svc.Create(ctxTest, th)
		require.ErrorIs(t, err, model.ErrDuplicated, "should have conflict error")
	})

	t.Run("create thing with same id after delete", func(t *testing.T) {
		th.Id = ""
		pre, err := svc.Create(ctxTest, th)
		require.NoError(t, err)
		connDelCall := connector.On("Close", pre.Id).Return(nil)
		removeCall := connector.On("Remove", pre.Id).Return(nil)
		defer connDelCall.Unset()
		defer removeCall.Unset()

		err = svc.Delete(ctxTest, pre.Id)
		require.NoError(t, err)

		th.Id = pre.Id
		_, err = svc.Create(ctxTest, th)
		require.NoError(t, err, "should have no error when creating duplicates with the same id as th deleted thing")
	})

	t.Run("create thing with password", func(t *testing.T) {
		th.Id = ""
		th.AuthValue = "password-xxx"
		re, err := svc.Create(ctxTest, th)
		require.NoError(t, err, "should have created with password")
		require.Equal(t, th.AuthValue, re.AuthValue, "should use password in request")
	})
}

func TestThingSvc_Update(t *testing.T) {
	svc, _ := NewTestSvc()
	th := thing.Thing{Id: "for-update-test"}
	_, err := svc.Create(ctxTest, th)
	require.NoError(t, err)
	t.Run("Disable thing", func(t *testing.T) {
		connDelCall := connector.On("Close", th.Id).Return(nil).Times(1)
		defer connDelCall.Unset()
		svc.Update(ctxTest, th.Id, thing.ThingUpdate{Enabled: model.Ref(false)})
		connDelCall.Parent.AssertExpectations(t)
		en, err := svc.Get(ctxTest, th.Id)
		require.NoError(t, err)
		require.Equal(t, en.Enabled, false)
	})
	t.Run("Enable thing", func(t *testing.T) {
		connDelCall := connector.On("Close", th.Id).Return(nil).Times(0)
		defer connDelCall.Unset()
		svc.Update(ctxTest, th.Id, thing.ThingUpdate{Enabled: model.Ref(true)})
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

	_, _ = svc.Create(ctxTest, thing.Thing{Id: randId})
	err = svc.Delete(ctxTest, randId)
	require.NoError(t, err)
	_, err = sdSvc.Get(ctxTest, randId, shadow.GetOption{WithStatus: false})
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

	_, _ = svc.Create(ctxTest, thing.Thing{Id: randId})

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

func TestTopicAcl(t *testing.T) {
	cases := []struct {
		supers []config.UserPassword
		user   string
		topic  string
		result bool
	}{
		{
			supers: []config.UserPassword{{Name: "a"}, {Name: "b"}},
			user:   "a",
			topic:  shadow.TopicUpdateOf("c"),
			result: true,
		},
		{
			supers: []config.UserPassword{{Name: "a"}, {Name: "b"}},
			user:   "b",
			topic:  shadow.TopicStateUpdatedOf("c"),
			result: true,
		},
		{
			supers: []config.UserPassword{{Name: "a"}, {Name: "b"}},
			user:   "d",
			topic:  shadow.TopicStateUpdatedOf("c"),
			result: false,
		},
		{
			supers: []config.UserPassword{{Name: "a"}, {Name: "b"}},
			user:   "c",
			topic:  shadow.TopicUpdateOf("c"),
			result: true,
		},
	}
	for _, c := range cases {
		r := thing.TopicAcl(c.supers, c.user, c.topic, true)
		require.Equal(t, c.result, r, fmt.Sprintf("user %s should access %s : %t", c.user, c.topic, c.result))
	}

}
