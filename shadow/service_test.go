package shadow_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"reflect"
	"ruff.io/tio/pkg/log"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"ruff.io/tio/db/mock"
	"ruff.io/tio/pkg/model"
	"ruff.io/tio/shadow"
	shadowMock "ruff.io/tio/shadow/mock"
	"ruff.io/tio/shadow/wire"
)

func newTestSvc() (shadow.Service, *gorm.DB) {
	db := mock.NewSqliteConnTest()
	err := db.AutoMigrate(&shadow.Entity{}, &shadow.ConnStatusEntity{})
	if err != nil {
		log.Fatalf("db AutoMigrate: %v", err)
	}
	time.Sleep(time.Millisecond * 100)
	return wire.InitSvc(db, shadowMock.NewConnectivity()), db
}

var ctx = context.Background()
var thingId = "test-thing-xx"
var stateVal = shadow.StateValue{
	"color": "red",
	"config": map[string]any{
		"period": 30,
	},
}
var svc, db = newTestSvc()

func TestShadowSvc_Create(t *testing.T) {
	id := fmt.Sprintf("for-create-%d", time.Now().UnixNano())
	s, err := svc.Create(ctx, id)
	require.NoError(t, err)
	require.Equal(t, id, s.ThingId)

	ss, err := svc.Get(ctx, id, shadow.GetOption{})
	require.NoError(t, err)
	require.Equal(t, id, ss.ThingId)
	require.Equal(t, int64(1), s.Version)

	// should also create conn status
	cs := shadow.ConnStatusEntity{ThingId: ss.ThingId}
	res := db.First(&cs)
	require.NoError(t, res.Error)
	require.Equal(t, id, cs.ThingId)

	err = svc.Delete(ctx, id)

	require.NoError(t, err)

	ss, err = svc.Get(ctx, id, shadow.GetOption{})
	require.Equal(t, model.ErrNotFound, err)

	// should also delete conn status
	res = db.First(&cs)
	require.Error(t, res.Error)
	require.True(t, errors.Is(res.Error, gorm.ErrRecordNotFound))
}

func TestShadowSVc_Query(t *testing.T) {
	id := fmt.Sprintf("for-query-%d", time.Now().UnixNano())
	s, err := svc.Create(ctx, id)
	require.NoError(t, err)
	require.Equal(t, id, s.ThingId)

	req := shadow.StateReq{ClientToken: "xxx", Version: 1, State: shadow.StateDR{
		Desired: shadow.StateValue{
			"color": "red",
			"config": map[string]any{
				"period": 30.0,
			},
		},
	}}
	s, err = svc.SetDesired(ctx, id, req)
	require.NoError(t, err)

	ss, err := svc.Query(ctx, model.PageQuery{PageIndex: 1, PageSize: 10},
		"select thingId, connected, `state.desired.config.period` as p, `state.desired.color` from shadow where thingId = '"+id+"'")
	require.NoError(t, err)
	require.Equal(t, 1, len(ss.Content))
	require.Equal(t, map[string]any{"thingId": id, "connected": false, "color": "red", "p": 30.0}, ss.Content[0])
}

func TestSvcImpl_Set(t *testing.T) {

	t.Run("should auto create when first set desired", func(t *testing.T) {
		thingId = "for-set-desired"
		s, err := svc.Create(ctx, thingId)
		require.NoError(t, err)
		req := shadow.StateReq{ClientToken: "xxx", Version: 1, State: shadow.StateDR{Desired: stateVal}}
		s, err = svc.SetDesired(ctx, thingId, req)
		require.NoError(t, err)
		o, _ := json.Marshal(stateVal)
		n, _ := json.Marshal(s.State.Desired)
		require.Equal(t, string(o), string(n))

		// TODO check shadow metadata
	})

	t.Run("should auto create when first set reported", func(t *testing.T) {
		thingId = "for-report"
		s, err := svc.Create(ctx, thingId)
		require.NoError(t, err)
		req := shadow.StateReq{ClientToken: "xxx", Version: 1, State: shadow.StateDR{Reported: stateVal}}
		s, err = svc.SetReported(ctx, thingId, req)
		require.NoError(t, err)
		o, _ := json.Marshal(stateVal)
		n, _ := json.Marshal(s.State.Reported)
		require.Equal(t, string(o), string(n))
	})

	t.Run("should update state", func(t *testing.T) {
		thingId = fmt.Sprintf("for-update-state-%d", time.Now().UnixNano())
		_, err := svc.Create(ctx, thingId)
		require.NoError(t, err)
		req := shadow.StateReq{ClientToken: "xxx", Version: 1, State: shadow.StateDR{Desired: stateVal}}
		s, err := svc.SetDesired(ctx, thingId, req)
		require.NoError(t, err)

		_, err = svc.Get(ctx, thingId, shadow.GetOption{})
		require.NoError(t, err)

		stateVal["color"] = "green"
		stateVal["config"] = map[string]any{"period": 44, "enabled": true}
		req.Version = 2
		s, err = svc.SetDesired(ctx, thingId, req)
		require.NoError(t, err)
		o, _ := json.Marshal(stateVal)
		n, _ := json.Marshal(s.State.Desired)
		require.Equal(t, string(o), string(n))
	})
}

func TestShadowSvc_SubscribeUpdate(t *testing.T) {
	svc, _ := newTestSvc()
	upd := struct {
		ThingId     string
		StateNotice shadow.StateUpdatedNotice
	}{}
	svc.SubscribeUpdate(func(thingId string, state shadow.StateUpdatedNotice) {
		upd.ThingId = thingId
		upd.StateNotice = state
	})

	thingId = fmt.Sprintf("for-sub-%d", time.Now().UnixNano())
	_, err := svc.Create(ctx, thingId)
	require.NoError(t, err)

	cases := []struct {
		color  string
		config map[string]any
		token  string
	}{
		{
			color: "red",
			config: map[string]any{
				"period": "10",
			},
			token: "1111",
		},
		{
			color: "white",
			config: map[string]any{
				"period": "20",
			},
			token: "2222",
		},
		{
			color: "black",
			config: map[string]any{
				// change period to map
				"period": map[string]any{
					"seconds": "30",
				},
			},
			token: "3333",
		},
	}

	var preColor any
	var preConfig map[string]any
	req := shadow.StateReq{Version: 1}
	for _, c := range cases {
		req.ClientToken = c.token
		stateVal["color"] = c.color
		stateVal["config"] = c.config
		req.State = shadow.StateDR{Desired: stateVal}
		_, err = svc.SetDesired(ctx, thingId, req)
		require.NoError(t, err)

		require.Equal(t, thingId, upd.ThingId, "state notice thingId should equal origin")
		require.Equal(t, req.ClientToken, upd.StateNotice.ClientToken, "state notice clientToken should equal origin")

		require.Equal(t, stateVal["color"], upd.StateNotice.Current.State.Desired["color"])
		require.Equal(t, preColor, upd.StateNotice.Previous.State.Desired["color"])

		require.Equal(t, stateVal["config"], upd.StateNotice.Current.State.Desired["config"])
		preNotifyConf := upd.StateNotice.Previous.State.Desired["config"]
		require.Truef(t, preConfig == nil && preNotifyConf == nil || reflect.DeepEqual(preConfig, preNotifyConf),
			"previous not equal to notify: %#v != %#v", preConfig, preNotifyConf)

		preColor = c.color
		preConfig = c.config
		req.Version++
	}
}

func TestShadowSvc_SubscribeDelta(t *testing.T) {
	svc, _ := newTestSvc()
	thingId = fmt.Sprintf("for-delta-sub-%d", time.Now().UnixNano())
	_, err := svc.Create(ctx, thingId)
	require.NoError(t, err)

	cases := []struct {
		color       string
		clientToken string
	}{
		{
			color:       "red",
			clientToken: "1111",
		},
		{
			color:       "white",
			clientToken: "2222",
		},
	}

	lastDelta := struct {
		ThingId     string
		StateNotice shadow.DeltaStateNotice
	}{}
	svc.SubscribeDelta(func(thingId string, state shadow.DeltaStateNotice) {
		lastDelta.ThingId = thingId
		lastDelta.StateNotice = state
	})
	resetDelta := func() {
		lastDelta = struct {
			ThingId     string
			StateNotice shadow.DeltaStateNotice
		}{}
	}

	version := int64(1)
	for _, c := range cases {
		t.Run("set desired to notify delta state", func(t *testing.T) {
			resetDelta()
			stateVal["color"] = c.color
			req := shadow.StateReq{ClientToken: c.clientToken, State: shadow.StateDR{Desired: stateVal}, Version: version}
			_, err = svc.SetDesired(ctx, thingId, req)
			require.NoError(t, err)

			assertDelta(t, lastDelta, req.ClientToken, c.color)
			version++
		})
		t.Run("set wrong version discard update", func(t *testing.T) {
			req := shadow.StateReq{ClientToken: c.clientToken, State: shadow.StateDR{Desired: stateVal}, Version: version + 1}
			_, err = svc.SetDesired(ctx, thingId, req)
			require.Error(t, err)
		})
		t.Run("set reported equal to notify nothing", func(t *testing.T) {
			resetDelta()
			reqRep := shadow.StateReq{ClientToken: c.clientToken, State: shadow.StateDR{Reported: stateVal}}
			_, err := svc.SetReported(ctx, thingId, reqRep)
			require.NoError(t, err)
			require.Equal(t, "", lastDelta.ThingId)
			version++
		})
		t.Run("set reported different to notify", func(t *testing.T) {
			resetDelta()
			desiredClr := stateVal["color"].(string)
			stateVal["color"] = fmt.Sprintf("rpt-%d", time.Now().Nanosecond())
			reqRep := shadow.StateReq{ClientToken: c.clientToken + "rpt", State: shadow.StateDR{Reported: stateVal}}
			_, err := svc.SetReported(ctx, thingId, reqRep)
			require.NoError(t, err)

			assertDelta(t, lastDelta, reqRep.ClientToken, desiredClr)
			version++
		})
	}
}

type lastDelta = struct {
	ThingId     string
	StateNotice shadow.DeltaStateNotice
}

func assertDelta(t *testing.T, lastDelta lastDelta, token, color string) {
	require.Equal(t, thingId, lastDelta.ThingId, "state notice thingId should equal origin")
	require.Equal(t, token, lastDelta.StateNotice.ClientToken, "state notice clientToken should equal origin")
	require.Equal(t, color, lastDelta.StateNotice.State["color"])
	m := lastDelta.StateNotice.Metadata["color"]
	var ts int64
	if tsm, ok := m.(map[string]any); ok {
		ts = int64(tsm["timestamp"].(float64))
	} else if tsm, ok := m.(shadow.MetaTimestamp); ok {
		ts = tsm.Timestamp
	}
	dTs := time.Now().UnixMilli() - ts
	require.Truef(t, dTs >= 0 && dTs < 100, "state metadata should be closer to current time")
}
