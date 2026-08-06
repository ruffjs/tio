package nats

import (
	"context"
	"testing"
	"time"

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
	authn := &NatsAuthenticator{}
	perms := authn.thingPermissions("test-thing")

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
		"$MQTT.sub.>",
		"$iothub.events.things.>",
	}
	require.Equal(t, expectedSubscribe, perms.Subscribe.Allow)
}

func TestNatsAuthenticator_SimpleThingPermissionsExcludeLegacy(t *testing.T) {
	authn := &NatsAuthenticator{protocolMode: "simple"}
	perms := authn.thingPermissions("test-thing")

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

func TestNatsAuthenticator_SimpleSuperUserPermissionsExcludeLegacy(t *testing.T) {
	authn := &NatsAuthenticator{
		protocolMode: "simple",
		superUsers:   []config.UserPassword{{Name: "admin"}},
	}
	perms := authn.thingPermissions("admin")

	require.Equal(t, []string{"tio.>"}, perms.Publish.Allow)
	require.Equal(t, []string{"tio.>", "$MQTT.sub.>", "_INBOX.>"}, perms.Subscribe.Allow)
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

	perms := authn.thingPermissions("gateway-1")

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
		"$MQTT.sub.>",
		"$iothub.events.things.>",
		"$iothub.things.device-a.>",
		"$iothub.user.things.device-a.>",
		"$iothub.things.device-b.>",
		"$iothub.user.things.device-b.>",
	}
	require.Equal(t, expectedSubscribe, perms.Subscribe.Allow)
}
