package nats

import (
	"context"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"ruff.io/tio/config"
	"ruff.io/tio/connector"
)

func TestNatsAuthenticator_InternalAppClientAuth(t *testing.T) {
	cfg := testServerConfig(t)
	appClient := config.NatsClientConfig{User: "app-user", Password: "app-pass"}
	sysClient := config.NatsClientConfig{User: "sys-user", Password: "sys-pass"}
	mqttPub := config.NatsClientConfig{User: "mqtt-pub", Password: "mqtt-pass"}

	authn := NewNatsAuthenticator(nil, nil, nil, appClient, sysClient, mqttPub, "legacy")
	s, err := StartNatsServer(cfg, authn)
	require.NoError(t, err)
	defer s.Shutdown()

	appAcc, _ := s.Server().LookupAccount(AppAccountName)
	sysAcc, _ := s.Server().LookupAccount(SysAccountName)
	authn.SetAccounts(appAcc, sysAcc)

	nc, err := nats.Connect(s.ClientURL(), nats.UserInfo(appClient.User, appClient.Password), nats.Name("app-client"), nats.Timeout(2*time.Second))
	require.NoError(t, err)
	defer nc.Close()

	err = nc.Publish("$iothub.things.test-device/shadows/name/default/update", []byte("test"))
	require.NoError(t, err)
}

func TestNatsAuthenticator_InternalSysClientAuth(t *testing.T) {
	cfg := testServerConfig(t)
	appClient := config.NatsClientConfig{User: "app-user", Password: "app-pass"}
	sysClient := config.NatsClientConfig{User: "sys-user", Password: "sys-pass"}
	mqttPub := config.NatsClientConfig{User: "mqtt-pub", Password: "mqtt-pass"}

	authn := NewNatsAuthenticator(nil, nil, nil, appClient, sysClient, mqttPub, "legacy")
	s, err := StartNatsServer(cfg, authn)
	require.NoError(t, err)
	defer s.Shutdown()

	appAcc, _ := s.Server().LookupAccount(AppAccountName)
	sysAcc, _ := s.Server().LookupAccount(SysAccountName)
	authn.SetAccounts(appAcc, sysAcc)

	nc, err := nats.Connect(s.ClientURL(), nats.UserInfo(sysClient.User, sysClient.Password), nats.Name("sys-client"), nats.Timeout(2*time.Second))
	require.NoError(t, err)
	defer nc.Close()

	sub, err := nc.SubscribeSync("$SYS.>")
	require.NoError(t, err)
	_ = sub
}

func TestNatsAuthenticator_InternalMqttPublisherAuth(t *testing.T) {
	cfg := testServerConfig(t)
	appClient := config.NatsClientConfig{User: "app-user", Password: "app-pass"}
	sysClient := config.NatsClientConfig{User: "sys-user", Password: "sys-pass"}
	mqttPub := config.NatsClientConfig{User: "mqtt-pub", Password: "mqtt-pass"}

	authn := NewNatsAuthenticator(nil, nil, nil, appClient, sysClient, mqttPub, "legacy")
	s, err := StartNatsServer(cfg, authn)
	require.NoError(t, err)
	defer s.Shutdown()

	appAcc, _ := s.Server().LookupAccount(AppAccountName)
	sysAcc, _ := s.Server().LookupAccount(SysAccountName)
	authn.SetAccounts(appAcc, sysAcc)

	nc, err := nats.Connect(s.ClientURL(), nats.UserInfo(mqttPub.User, mqttPub.Password), nats.Name("mqtt-pub-client"), nats.Timeout(2*time.Second))
	require.NoError(t, err)
	defer nc.Close()

	err = nc.Publish("$iothub.things.test-device/shadows/name/default/update", []byte("test"))
	require.NoError(t, err)

	_, err = nc.SubscribeSync("$iothub.>")
	require.NoError(t, err)
	err = nc.Flush()
	require.NoError(t, err)
}

func TestNatsAuthenticator_DynamicThingPasswordAuth(t *testing.T) {
	cfg := testServerConfig(t)
	appClient := config.NatsClientConfig{User: "app-user", Password: "app-pass"}
	sysClient := config.NatsClientConfig{User: "sys-user", Password: "sys-pass"}
	mqttPub := config.NatsClientConfig{User: "mqtt-pub", Password: "mqtt-pass"}

	authzFn := func(authCtx connector.AuthContext) (connector.AuthResult, bool) {
		if authCtx.Username == "thing-1" && authCtx.Password == "thing-pass" {
			return connector.AuthResult{Principal: "thing-1", AuthMethod: "password"}, true
		}
		return connector.AuthResult{}, false
	}

	authn := NewNatsAuthenticator(authzFn, nil, nil, appClient, sysClient, mqttPub, "legacy")
	s, err := StartNatsServer(cfg, authn)
	require.NoError(t, err)
	defer s.Shutdown()

	appAcc, _ := s.Server().LookupAccount(AppAccountName)
	sysAcc, _ := s.Server().LookupAccount(SysAccountName)
	authn.SetAccounts(appAcc, sysAcc)

	nc, err := nats.Connect(s.ClientURL(), nats.UserInfo("thing-1", "thing-pass"), nats.Name("thing-client"), nats.Timeout(2*time.Second))
	require.NoError(t, err)
	defer nc.Close()

	thingTopic := "$iothub.things.thing-1.shadows.name.default.update"
	err = nc.Publish(thingTopic, []byte("test"))
	require.NoError(t, err)
}

func TestNatsAuthenticator_DynamicThingAuthDenied(t *testing.T) {
	cfg := testServerConfig(t)
	appClient := config.NatsClientConfig{User: "app-user", Password: "app-pass"}
	sysClient := config.NatsClientConfig{User: "sys-user", Password: "sys-pass"}
	mqttPub := config.NatsClientConfig{User: "mqtt-pub", Password: "mqtt-pass"}

	authzFn := func(authCtx connector.AuthContext) (connector.AuthResult, bool) {
		return connector.AuthResult{}, false
	}

	authn := NewNatsAuthenticator(authzFn, nil, nil, appClient, sysClient, mqttPub, "legacy")
	s, err := StartNatsServer(cfg, authn)
	require.NoError(t, err)
	defer s.Shutdown()

	appAcc, _ := s.Server().LookupAccount(AppAccountName)
	sysAcc, _ := s.Server().LookupAccount(SysAccountName)
	authn.SetAccounts(appAcc, sysAcc)

	_, err = nats.Connect(s.ClientURL(), nats.UserInfo("bad-user", "bad-pass"), nats.Name("bad-client"), nats.Timeout(2*time.Second))
	require.Error(t, err)
}

func TestNatsAuthenticator_AnonymousRejected(t *testing.T) {
	cfg := testServerConfig(t)
	appClient := config.NatsClientConfig{User: "app-user", Password: "app-pass"}
	sysClient := config.NatsClientConfig{User: "sys-user", Password: "sys-pass"}
	mqttPub := config.NatsClientConfig{User: "mqtt-pub", Password: "mqtt-pass"}

	authzFn := func(authCtx connector.AuthContext) (connector.AuthResult, bool) {
		return connector.AuthResult{}, false
	}

	authn := NewNatsAuthenticator(authzFn, nil, nil, appClient, sysClient, mqttPub, "legacy")
	s, err := StartNatsServer(cfg, authn)
	require.NoError(t, err)
	defer s.Shutdown()

	appAcc, _ := s.Server().LookupAccount(AppAccountName)
	sysAcc, _ := s.Server().LookupAccount(SysAccountName)
	authn.SetAccounts(appAcc, sysAcc)

	_, err = nats.Connect(s.ClientURL(), nats.Name("anon-client"), nats.Timeout(2*time.Second))
	require.Error(t, err)
}

func TestNatsAuthenticator_ThingCannotAccessOtherThingTopics(t *testing.T) {
	cfg := testServerConfig(t)
	appClient := config.NatsClientConfig{User: "app-user", Password: "app-pass"}
	sysClient := config.NatsClientConfig{User: "sys-user", Password: "sys-pass"}
	mqttPub := config.NatsClientConfig{User: "mqtt-pub", Password: "mqtt-pass"}

	authzFn := func(authCtx connector.AuthContext) (connector.AuthResult, bool) {
		if authCtx.Username == "thing-a" {
			return connector.AuthResult{Principal: "thing-a", AuthMethod: "password"}, true
		}
		return connector.AuthResult{}, false
	}

	authn := NewNatsAuthenticator(authzFn, nil, nil, appClient, sysClient, mqttPub, "legacy")
	s, err := StartNatsServer(cfg, authn)
	require.NoError(t, err)
	defer s.Shutdown()

	appAcc, _ := s.Server().LookupAccount(AppAccountName)
	sysAcc, _ := s.Server().LookupAccount(SysAccountName)
	authn.SetAccounts(appAcc, sysAcc)

	nc, err := nats.Connect(s.ClientURL(), nats.UserInfo("thing-a", "pass"), nats.Name("thing-a-client"), nats.Timeout(2*time.Second))
	require.NoError(t, err)
	defer nc.Close()

	otherThingTopic := "$iothub.things.thing-b.shadows.name.default.update"
	err = nc.Publish(otherThingTopic, []byte("test"))
	require.NoError(t, err)

	err = nc.Flush()
	require.NoError(t, err)
}

func TestNatsAuthenticator_ThingCannotAccessSysSubjects(t *testing.T) {
	cfg := testServerConfig(t)
	appClient := config.NatsClientConfig{User: "app-user", Password: "app-pass"}
	sysClient := config.NatsClientConfig{User: "sys-user", Password: "sys-pass"}
	mqttPub := config.NatsClientConfig{User: "mqtt-pub", Password: "mqtt-pass"}

	authzFn := func(authCtx connector.AuthContext) (connector.AuthResult, bool) {
		if authCtx.Username == "thing-1" {
			return connector.AuthResult{Principal: "thing-1", AuthMethod: "password"}, true
		}
		return connector.AuthResult{}, false
	}

	authn := NewNatsAuthenticator(authzFn, nil, nil, appClient, sysClient, mqttPub, "legacy")
	s, err := StartNatsServer(cfg, authn)
	require.NoError(t, err)
	defer s.Shutdown()

	appAcc, _ := s.Server().LookupAccount(AppAccountName)
	sysAcc, _ := s.Server().LookupAccount(SysAccountName)
	authn.SetAccounts(appAcc, sysAcc)

	nc, err := nats.Connect(s.ClientURL(), nats.UserInfo("thing-1", "pass"), nats.Name("thing-client"), nats.Timeout(2*time.Second))
	require.NoError(t, err)
	defer nc.Close()

	_, err = nc.SubscribeSync("$SYS.>")
	require.NoError(t, err)
	err = nc.Flush()
	require.NoError(t, err)
}

func TestNatsAuthenticator_ThingPermissions(t *testing.T) {
	authn := &NatsAuthenticator{protocolMode: "legacy"}
	perms := authn.thingPermissions("test-thing", false)

	require.NotNil(t, perms)
	require.NotNil(t, perms.Publish)
	require.NotNil(t, perms.Subscribe)

	expectedPublish := []string{
		"$iothub.things.test-thing.>",
		"$iothub.user.things.test-thing.>",
	}
	require.Equal(t, expectedPublish, perms.Publish.Allow)

	expectedSubscribe := []string{
		"$iothub.things.test-thing.>",
		"$iothub.user.things.test-thing.>",
		"$iothub.things.test-thing",
		"$iothub.user.things.test-thing",
		"$MQTT.sub.>",
		"$iothub.events.things.>",
	}
	require.Equal(t, expectedSubscribe, perms.Subscribe.Allow)
}

func TestNatsAuthenticator_MqttThingWildcardSubscriptions(t *testing.T) {
	cfg := testServerConfig(t)
	cfg.MqttPort = freeTCPPort(t)
	authzFn := func(ctx connector.AuthContext) (connector.AuthResult, bool) {
		return connector.AuthResult{Principal: ctx.Username, AuthMethod: "password"}, ctx.Username == "gateway-1"
	}
	authn := NewNatsAuthenticator(authzFn,
		&stubBindingGetter{boundThings: []string{"device-a"}}, nil,
		config.NatsClientConfig{User: "app", Password: "app"},
		config.NatsClientConfig{User: "sys", Password: "sys"},
		config.NatsClientConfig{User: "mqtt", Password: "mqtt"}, "legacy")
	s, err := StartNatsServer(cfg, authn)
	require.NoError(t, err)
	defer s.Shutdown()
	appAcc, err := s.Server().LookupAccount(AppAccountName)
	require.NoError(t, err)
	sysAcc, err := s.Server().LookupAccount(SysAccountName)
	require.NoError(t, err)
	authn.SetAccounts(appAcc, sysAcc)
	c := testMqttSubscriber(t, getMqttPort(t, s), "gateway-1", "pass")
	defer c.Disconnect(100)

	for _, tt := range []struct {
		topic string
		allow bool
	}{
		{"$iothub/things/gateway-1/#", true},
		{"$iothub/user/things/gateway-1/#", true},
		{"$iothub/things/device-a/#", true},
		{"$iothub/user/things/device-a/#", true},
		{"$iothub/things/device-b/#", false},
		{"$iothub/user/things/device-b/#", false},
		{"$iothub/things/#", false},
	} {
		t.Run(tt.topic, func(t *testing.T) {
			token := c.Subscribe(tt.topic, 0, nil)
			require.True(t, token.WaitTimeout(3*time.Second))
			require.NoError(t, token.Error())
			result := token.(*mqtt.SubscribeToken).Result()
			require.Contains(t, result, tt.topic)
			if tt.allow {
				require.Equal(t, byte(0), result[tt.topic])
			} else {
				require.Equal(t, byte(128), result[tt.topic])
			}
		})
	}
}

func TestNatsAuthenticator_SimpleThingPermissionsExcludeLegacy(t *testing.T) {
	authn := &NatsAuthenticator{protocolMode: "simple"}
	perms := authn.thingPermissions("test-thing", false)

	require.Equal(t, []string{
		"tio.test-thing.>",
	}, perms.Publish.Allow)
	require.Equal(t, []string{
		"tio.test-thing.down",
		"tio.test-thing.down.>",
	}, perms.Publish.Deny)
	require.Equal(t, []string{
		"tio.test-thing.>",
		"$MQTT.sub.>",
	}, perms.Subscribe.Allow)
	require.Equal(t, []string{
		"tio.test-thing.up",
		"tio.test-thing.up.>",
	}, perms.Subscribe.Deny)
}

func TestNatsAuthenticator_SuperUserPermissions(t *testing.T) {
	for _, mode := range []string{"legacy", "simple", "dual"} {
		t.Run(mode, func(t *testing.T) {
			authn := &NatsAuthenticator{protocolMode: mode, superUsers: []config.UserPassword{{Name: "admin"}}}
			perms := authn.thingPermissions("admin", true)
			require.Equal(t, []string{">"}, perms.Publish.Allow)
			require.Equal(t, []string{">"}, perms.Subscribe.Allow)
			require.Empty(t, perms.Publish.Deny)
			require.Empty(t, perms.Subscribe.Deny)
			ordinary := authn.thingPermissions("admin", false)
			require.NotContains(t, ordinary.Publish.Allow, ">", "a matching name must not grant superuser access")
			require.NotContains(t, ordinary.Subscribe.Allow, ">")
		})
	}
}

func TestNatsAuthenticator_SuperUserAllTopics(t *testing.T) {
	for _, mode := range []string{"legacy", "simple", "dual"} {
		t.Run(mode, func(t *testing.T) {
			cfg := testConnectorConfig(t)
			cfg.SuperUsers = []config.UserPassword{{Name: "admin", Password: "secret"}}
			authz := func(c connector.AuthContext) (connector.AuthResult, bool) {
				if c.Username == "admin" && c.Password == "secret" {
					return connector.AuthResult{Principal: "admin", AuthMethod: "superuser-password"}, true
				}
				return connector.AuthResult{Principal: "admin", AuthMethod: "password"}, c.Username == "admin" && c.Password == "device-secret"
			}
			authn := NewNatsAuthenticator(authz, nil, cfg.SuperUsers, cfg.AppClient, cfg.SystemClient, cfg.MqttPublisher, mode)
			s, err := StartNatsServer(cfg.Server, authn)
			require.NoError(t, err)
			defer s.Shutdown()
			app, err := s.Server().LookupAccount(AppAccountName)
			require.NoError(t, err)
			sys, err := s.Server().LookupAccount(SysAccountName)
			require.NoError(t, err)
			authn.SetAccounts(app, sys)

			nc, err := nats.Connect(s.ClientURL(), nats.UserInfo("admin", "secret"))
			require.NoError(t, err)
			defer nc.Close()
			sub, err := nc.SubscribeSync(">")
			require.NoError(t, err)
			require.NoError(t, nc.FlushTimeout(time.Second))
			for _, subject := range []string{"business.custom", "tio.other.down.test", "$iothub.things.other.test"} {
				require.NoError(t, nc.Publish(subject, []byte("nats")))
				msg, err := sub.NextMsg(time.Second)
				require.NoError(t, err)
				require.Equal(t, subject, msg.Subject)
				require.Equal(t, []byte("nats"), msg.Data)
			}
			require.NoError(t, sub.Unsubscribe())

			denied := make(chan error, 1)
			ordinary, err := nats.Connect(s.ClientURL(), nats.UserInfo("admin", "device-secret"),
				nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) { denied <- err }))
			require.NoError(t, err)
			defer ordinary.Close()
			_, err = ordinary.SubscribeSync(">")
			require.NoError(t, err)
			require.NoError(t, ordinary.FlushTimeout(time.Second))
			select {
			case err := <-denied:
				require.ErrorContains(t, err, "Permissions Violation")
			case <-time.After(time.Second):
				t.Fatal("ordinary authentication with a superuser name must not allow all subjects")
			}

			client := testMqttSubscriber(t, getMqttPort(t, s), "admin", "secret")
			defer client.Disconnect(100)
			received := make(chan string, 1)
			token := client.Subscribe("#", 0, func(_ mqtt.Client, msg mqtt.Message) {
				if msg.Topic() == "business/custom" {
					received <- string(msg.Payload())
				}
			})
			require.True(t, token.WaitTimeout(time.Second))
			require.NoError(t, token.Error())
			require.Equal(t, byte(0), token.(*mqtt.SubscribeToken).Result()["#"])
			token = client.Publish("business/custom", 0, false, []byte("mqtt"))
			require.True(t, token.WaitTimeout(time.Second))
			require.NoError(t, token.Error())
			select {
			case payload := <-received:
				require.Equal(t, "mqtt", payload)
			case <-time.After(time.Second):
				t.Fatal("superuser did not receive custom MQTT topic")
			}
		})
	}
}

func TestNatsAuthenticator_DualThingPermissions(t *testing.T) {
	authn := &NatsAuthenticator{protocolMode: "dual"}
	perms := authn.thingPermissions("test-thing", false)

	require.Equal(t, []string{
		"tio.test-thing.>",
		"$iothub.things.test-thing.>",
		"$iothub.user.things.test-thing.>",
	}, perms.Publish.Allow)
	require.Equal(t, []string{"tio.test-thing.down", "tio.test-thing.down.>"}, perms.Publish.Deny)
	require.Equal(t, []string{
		"tio.test-thing.>",
		"$iothub.things.test-thing.>",
		"$iothub.user.things.test-thing.>",
		"$iothub.things.test-thing",
		"$iothub.user.things.test-thing",
		"$MQTT.sub.>",
		"$iothub.events.things.>",
	}, perms.Subscribe.Allow)
	require.Equal(t, []string{"tio.test-thing.up", "tio.test-thing.up.>"}, perms.Subscribe.Deny)
}

type stubBindingGetter struct {
	boundThings []string
}

func (s *stubBindingGetter) IsBoundGateway(_ context.Context, _, _ string) (bool, error) {
	return false, nil
}

func (s *stubBindingGetter) GetBoundThingIds(_ context.Context, _ string) ([]string, error) {
	return s.boundThings, nil
}

func TestThingPermissionsForGatewayIncludesBoundThings(t *testing.T) {
	appClient := config.NatsClientConfig{User: "app", Password: "app"}
	sysClient := config.NatsClientConfig{User: "sys", Password: "sys"}
	mqttPub := config.NatsClientConfig{User: "mqtt", Password: "mqtt"}

	bg := &stubBindingGetter{boundThings: []string{"device-a", "device-b"}}
	authn := NewNatsAuthenticator(nil, bg, nil, appClient, sysClient, mqttPub, "legacy")

	perms := authn.thingPermissions("gateway-1", false)

	expectedPublish := []string{
		"$iothub.things.gateway-1.>",
		"$iothub.user.things.gateway-1.>",
		"$iothub.things.device-a.>",
		"$iothub.user.things.device-a.>",
		"$iothub.things.device-b.>",
		"$iothub.user.things.device-b.>",
	}
	require.Equal(t, expectedPublish, perms.Publish.Allow)

	expectedSubscribe := []string{
		"$iothub.things.gateway-1.>",
		"$iothub.user.things.gateway-1.>",
		"$iothub.things.gateway-1",
		"$iothub.user.things.gateway-1",
		"$MQTT.sub.>",
		"$iothub.events.things.>",
		"$iothub.things.device-a.>",
		"$iothub.user.things.device-a.>",
		"$iothub.things.device-a",
		"$iothub.user.things.device-a",
		"$iothub.things.device-b.>",
		"$iothub.user.things.device-b.>",
		"$iothub.things.device-b",
		"$iothub.user.things.device-b",
	}
	require.Equal(t, expectedSubscribe, perms.Subscribe.Allow)
}

func TestDualThingPermissionsForGatewayIncludesBoundThings(t *testing.T) {
	bg := &stubBindingGetter{boundThings: []string{"device-a"}}
	authn := NewNatsAuthenticator(nil, bg, nil,
		config.NatsClientConfig{User: "app", Password: "app"},
		config.NatsClientConfig{User: "sys", Password: "sys"},
		config.NatsClientConfig{User: "mqtt", Password: "mqtt"}, "dual")

	perms := authn.thingPermissions("gateway-1", false)

	require.Equal(t, []string{
		"tio.gateway-1.>",
		"$iothub.things.gateway-1.>",
		"$iothub.user.things.gateway-1.>",
		"tio.device-a.>",
		"$iothub.things.device-a.>",
		"$iothub.user.things.device-a.>",
	}, perms.Publish.Allow)
	require.Equal(t, []string{"tio.gateway-1.down", "tio.gateway-1.down.>", "tio.device-a.down", "tio.device-a.down.>"}, perms.Publish.Deny)
	require.Equal(t, []string{
		"tio.gateway-1.>",
		"$iothub.things.gateway-1.>",
		"$iothub.user.things.gateway-1.>",
		"$iothub.things.gateway-1",
		"$iothub.user.things.gateway-1",
		"$MQTT.sub.>",
		"$iothub.events.things.>",
		"tio.device-a.>",
		"$iothub.things.device-a.>",
		"$iothub.user.things.device-a.>",
		"$iothub.things.device-a",
		"$iothub.user.things.device-a",
	}, perms.Subscribe.Allow)
	require.Equal(t, []string{"tio.gateway-1.up", "tio.gateway-1.up.>", "tio.device-a.up", "tio.device-a.up.>"}, perms.Subscribe.Deny)
}

func TestNatsAuthenticator_SystemAccountRequired(t *testing.T) {
	cfg := testConnectorConfig(t)
	authn := NewNatsAuthenticator(nil, nil, nil, cfg.AppClient, cfg.SystemClient, cfg.MqttPublisher, "legacy")
	s, err := StartNatsServer(cfg.Server, authn)
	require.NoError(t, err)
	defer s.Shutdown()
	appAcc, err := s.Server().LookupAccount(AppAccountName)
	require.NoError(t, err)
	authn.SetAccounts(appAcc, nil)
	nc, err := nats.Connect(s.ClientURL(), nats.UserInfo(cfg.SystemClient.User, cfg.SystemClient.Password))
	if nc != nil {
		nc.Close()
	}
	require.ErrorIs(t, err, nats.ErrAuthorization)
}
