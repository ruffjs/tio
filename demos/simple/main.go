package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	_ "github.com/mattn/go-sqlite3"
	"ruff.io/tio/config"
	"ruff.io/tio/connector"
	"ruff.io/tio/pkg/codec"
	"ruff.io/tio/pkg/protocol"
	"ruff.io/tio/shadow"
)

type packet struct {
	topic string
	data  []byte
}

type probe struct {
	ctx       context.Context
	apiURL    string
	thingPath string
	prefix    string
	codec     codec.Codec
	http      *http.Client
	device    mqtt.Client
	inbox     chan packet
}

func pass(format string, args ...any) {
	fmt.Printf("PASS "+format+"\n", args...)
	time.Sleep(100 * time.Millisecond)
}

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func devicePassword(id string) (string, error) {
	if password := os.Getenv("TIO_DEVICE_PASSWORD"); password != "" {
		return password, nil
	}
	filename, err := filepath.Abs(env("TIO_SQLITE", "tio.sqlite"))
	if err != nil {
		return "", err
	}
	u := url.URL{Scheme: "file", Path: filename, RawQuery: "mode=ro&_busy_timeout=2000"}
	db, err := sql.Open("sqlite3", u.String())
	if err != nil {
		return "", err
	}
	defer db.Close()
	var password, authType string
	var enabled bool
	err = db.QueryRow("SELECT auth_value, auth_type, enabled FROM thing WHERE id = ?", id).Scan(&password, &authType, &enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("device %s not found in %s", id, filename)
	}
	if err != nil {
		return "", fmt.Errorf("read device credentials from %s: %w", filename, err)
	}
	if !enabled || (authType != "" && authType != "password") {
		return "", fmt.Errorf("device %s must be enabled and use password authentication", id)
	}
	return password, nil
}

func (p *probe) api(method, path string, body, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(method, p.apiURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.SetBasicAuth(env("TIO_API_USER", "admin"), env("TIO_API_PASSWORD", "public"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.http.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP %s %s failed", method, path)
	}
	defer resp.Body.Close()
	var envelope struct {
		Code int
		Data json.RawMessage
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("HTTP %s %s: invalid response (status %d)", method, path, resp.StatusCode)
	}
	if resp.StatusCode != 200 || envelope.Code != 200 {
		return fmt.Errorf("HTTP %s %s: status=%d code=%d", method, path, resp.StatusCode, envelope.Code)
	}
	if out != nil {
		return json.Unmarshal(envelope.Data, out)
	}
	return nil
}

func wait(token mqtt.Token) error {
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("MQTT operation timed out")
	}
	return token.Error()
}

func connect(name, user, password string, inbox chan packet) (mqtt.Client, error) {
	opts := mqtt.NewClientOptions().AddBroker(env("TIO_MQTT_URL", "tcp://127.0.0.1:1883")).
		SetClientID(name).SetUsername(user).SetPassword(password).
		SetAutoReconnect(false).SetCleanSession(true)
	opts.SetDefaultPublishHandler(func(_ mqtt.Client, msg mqtt.Message) {
		inbox <- packet{msg.Topic(), append([]byte(nil), msg.Payload()...)}
	})
	client := mqtt.NewClient(opts)
	if err := wait(client.Connect()); err != nil {
		client.Disconnect(0)
		return nil, fmt.Errorf("MQTT connect failed for %s", name)
	}
	return client, nil
}

func subscribe(client mqtt.Client, filter string, expected byte) error {
	token := client.Subscribe(filter, 1, nil)
	if err := wait(token); err != nil {
		return fmt.Errorf("subscribe %s: %w", filter, err)
	}
	code, ok := token.(*mqtt.SubscribeToken).Result()[filter]
	if !ok || code != expected {
		return fmt.Errorf("subscribe %s: SUBACK=%d, expected %d", filter, code, expected)
	}
	pass("subscribe %s (SUBACK=%d)", filter, code)
	return nil
}

func receive(ctx context.Context, inbox <-chan packet, topic string) ([]byte, error) {
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case msg := <-inbox:
			if msg.topic == topic {
				return msg.data, nil
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return nil, fmt.Errorf("timed out waiting for %s", topic)
		}
	}
}

func (p *probe) send(kind string, value any) error {
	data, err := p.codec.Marshal(value)
	if err != nil {
		return err
	}
	return wait(p.device.Publish(p.prefix+"/up/"+kind, 1, false, data))
}

func (p *probe) reply(ctx context.Context, kind string, out any) error {
	data, err := receive(ctx, p.inbox, p.prefix+"/down/"+kind)
	if err != nil {
		return err
	}
	return p.codec.Unmarshal(data, out)
}

func (p *probe) reported(ctx context.Context, state map[string]any, version int64, code int) (int64, error) {
	if err := p.send("shadow_update", protocol.ShadowUpdateReq{State: state, Version: version}); err != nil {
		return 0, err
	}
	var reply protocol.ShadowUpdateReply
	if err := p.reply(ctx, "shadow_update_reply", &reply); err != nil {
		return 0, err
	}
	if reply.Code != code {
		return 0, fmt.Errorf("shadow_update_reply code=%d, expected %d", reply.Code, code)
	}
	return reply.Version, nil
}

func (p *probe) desired(key string, value any) error {
	return p.api(http.MethodPut, p.thingPath+"/shadows/default/state/desired",
		map[string]any{"clientToken": key, "state": map[string]any{"desired": map[string]any{key: value}}}, nil)
}

func nestedState(value, label string) map[string]any {
	return map[string]any{
		"control": map[string]any{
			"target":  map[string]any{"value": value, "unit": "C"},
			"enabled": true,
		},
		"label": label,
	}
}

func run() (err error) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	id := env("TIO_THING_ID", "test")
	password, err := devicePassword(id)
	if err != nil {
		return err
	}
	p := &probe{ctx: ctx, apiURL: strings.TrimRight(env("TIO_API_URL", "http://127.0.0.1:9000"), "/"),
		thingPath: "/api/v1/things/" + url.PathEscape(id), prefix: "tio/" + id,
		http: &http.Client{Timeout: 8 * time.Second}, inbox: make(chan packet, 128)}
	var settings struct {
		Protocol config.Protocol
		Shadow   shadow.Config
	}
	if err := p.api(http.MethodGet, "/private/api/config", nil, &settings); err != nil {
		return err
	}
	var thing struct {
		Enabled                          bool
		MessageProtocol, PayloadEncoding string
	}
	if err := p.api(http.MethodGet, p.thingPath, nil, &thing); err != nil {
		return err
	}
	if !thing.Enabled || settings.Protocol.Mode == "legacy" || (settings.Protocol.Mode == "dual" && thing.MessageProtocol != "simple") {
		return fmt.Errorf("device must be enabled and use the Simple protocol")
	}
	encoding := thing.PayloadEncoding
	if encoding == "" {
		encoding = settings.Protocol.DefaultEncoding
	}
	p.codec, err = codec.New(encoding)
	if err != nil {
		return err
	}
	fmt.Printf("Testing thing=%s mode=%s encoding=%s fullFieldDelta=%t\n", id, settings.Protocol.Mode, encoding, settings.Shadow.FullFieldDelta)
	runID := fmt.Sprintf("%x", time.Now().UnixNano())
	key := "__tio_probe_" + runID
	backendInbox := make(chan packet, 128)
	backend, err := connect("probe-backend-"+runID, env("TIO_SUPER_USER", "$biz"), env("TIO_SUPER_PASSWORD", "public"), backendInbox)
	if err != nil {
		return err
	}
	defer backend.Disconnect(250)
	presenceTopic := "$iothub/things/" + id + "/presence"
	custom := p.prefix + "/custom/" + runID
	for _, topic := range []string{presenceTopic, custom, p.prefix + "/data", p.prefix + "/event"} {
		if err := subscribe(backend, topic, 1); err != nil {
			return err
		}
	}
	p.device, err = connect("probe-device-"+runID, id, password, p.inbox)
	if err != nil {
		return err
	}
	defer p.device.Disconnect(250)
	for _, filter := range []string{p.prefix + "/down/#", custom} {
		if err := subscribe(p.device, filter, 1); err != nil {
			return err
		}
	}
	for _, filter := range []string{p.prefix + "/#", p.prefix + "/up/#", "tio/probe-other-" + runID + "/down/#"} {
		if err := subscribe(p.device, filter, 128); err != nil {
			return err
		}
	}
	defaultCodec, err := codec.New(settings.Protocol.DefaultEncoding)
	if err != nil {
		return err
	}
	for {
		data, err := receive(ctx, backendInbox, presenceTopic)
		if err != nil {
			return err
		}
		var event connector.PresenceEvent
		if err := defaultCodec.Unmarshal(data, &event); err != nil {
			return err
		}
		if event.ThingId == id && event.EventType == connector.EventConnected {
			break
		}
	}
	pass("backend Presence: device connected")
	stamp := time.Now().UnixMilli()
	if err := p.send("ntp_req", protocol.NtpReq{ClientSendTime: stamp}); err != nil {
		return err
	}
	var ntp protocol.NtpResp
	if err := p.reply(ctx, "ntp_resp", &ntp); err != nil {
		return err
	}
	if ntp.Code != 200 || ntp.ClientSendTime != stamp || ntp.ServerRecvTime <= 0 || ntp.ServerSendTime < ntp.ServerRecvTime {
		return fmt.Errorf("invalid NTP response")
	}
	pass("up/ntp_req -> down/ntp_resp")
	if err := p.send("shadow_get", map[string]any{}); err != nil {
		return err
	}
	var initial protocol.ShadowGetReply
	if err := p.reply(ctx, "shadow_get_reply", &initial); err != nil {
		return err
	}
	if initial.Code != 200 || initial.State == nil {
		return fmt.Errorf("invalid shadow_get_reply")
	}
	if _, exists := initial.State.Desired[key]; exists {
		return fmt.Errorf("test field already exists")
	}
	if _, exists := initial.State.Reported[key]; exists {
		return fmt.Errorf("test field already exists")
	}
	pass("up/shadow_get -> down/shadow_get_reply")
	defer func() {
		cleanupCtx, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		cleanupErr := p.desired(key, nil)
		_, reportErr := p.reported(cleanupCtx, map[string]any{key: nil}, 0, 200)
		var current shadow.Shadow
		verifyErr := p.api(http.MethodGet, p.thingPath+"/shadows/default", nil, &current)
		_, desiredExists := current.State.Desired[key]
		_, reportedExists := current.State.Reported[key]
		if verifyErr == nil && (desiredExists || reportedExists) {
			verifyErr = fmt.Errorf("temporary field %s remains in shadow", key)
		}
		if cleanupErr = errors.Join(cleanupErr, reportErr, verifyErr); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("cleanup field %s: %w", key, cleanupErr))
		} else {
			pass("cleanup: temporary desired/reported fields removed")
		}
	}()
	baseline := nestedState("reported", runID)
	version, err := p.reported(ctx, map[string]any{key: baseline}, 0, 200)
	if err != nil {
		return err
	}
	var stored shadow.Shadow
	if err := p.api(http.MethodGet, p.thingPath+"/shadows/default", nil, &stored); err != nil {
		return err
	}
	if !reflect.DeepEqual(stored.State.Reported[key], baseline) || version <= initial.Version {
		return fmt.Errorf("reported update was not persisted")
	}
	pass("up/shadow_update -> down/shadow_update_reply -> HTTP persisted state")
	if _, err := p.reported(ctx, map[string]any{key: "stale"}, version-1, 409); err != nil {
		return err
	}
	pass("shadow_update: stale version 409")
	if _, err := p.reported(ctx, nil, 0, 400); err != nil {
		return err
	}
	pass("shadow_update: invalid state 400")
	if err := p.desired(key, baseline); err != nil {
		return err
	}
	leafPatch := map[string]any{"control": map[string]any{"target": map[string]any{"value": "desired"}}}
	if err := p.desired(key, leafPatch); err != nil {
		return err
	}
	var delta protocol.ShadowDelta
	if err := p.reply(ctx, "shadow_delta", &delta); err != nil {
		return err
	}
	wanted := leafPatch
	if settings.Shadow.FullFieldDelta {
		wanted = nestedState("desired", runID)
	}
	if !reflect.DeepEqual(delta.State[key], wanted) || delta.Version <= version {
		return fmt.Errorf("nested desired update produced an incorrect delta (fullFieldDelta=%t); unchanged siblings must match the configured delta shape", settings.Shadow.FullFieldDelta)
	}
	if settings.Shadow.FullFieldDelta {
		pass("HTTP patches %s.control.target.value -> delta contains complete %s (unit, enabled, label preserved)", key, key)
	} else {
		pass("nested leaf delta (fullFieldDelta=false; enable it to verify complete top-level fields)")
	}
	convergedState := nestedState("desired", runID)
	if _, err := p.reported(ctx, map[string]any{key: convergedState}, 0, 200); err != nil {
		return err
	}
	if err := p.send("shadow_get", map[string]any{}); err != nil {
		return err
	}
	var converged protocol.ShadowGetReply
	if err := p.reply(ctx, "shadow_get_reply", &converged); err != nil {
		return err
	}
	if converged.State == nil || !reflect.DeepEqual(converged.State.Desired[key], convergedState) || !reflect.DeepEqual(converged.State.Reported[key], convergedState) {
		return fmt.Errorf("desired/reported test field did not converge")
	}
	pass("device reports desired value -> shadow converged")
	method := "probe_" + runID
	methodDone := make(chan error, 1)
	go func() {
		var result struct {
			Code int
			Data string
		}
		callErr := p.api(http.MethodPost, p.thingPath+"/invoke", map[string]any{"method": method, "params": runID, "timeout": 4}, &result)
		if callErr == nil && (result.Code != 200 || result.Data != runID) {
			callErr = fmt.Errorf("HTTP method result differs from the device response")
		}
		methodDone <- callErr
	}()
	var request protocol.MethodReq
	if err := p.reply(ctx, "method_req", &request); err != nil {
		return err
	}
	if request.Method != method || request.Data != runID || request.ID == "" {
		return fmt.Errorf("unexpected method request; use a dedicated test device")
	}
	if err := p.send("method_resp", protocol.MethodResp{ID: request.ID, Code: 200, Data: runID}); err != nil {
		return err
	}
	if err := <-methodDone; err != nil {
		return err
	}
	pass("server HTTP invoke -> down/method_req -> up/method_resp -> HTTP result")
	for _, topic := range []string{p.prefix + "/data", p.prefix + "/event", custom} {
		payload := append([]byte{0, 255, 1}, []byte(runID)...)
		if err := wait(p.device.Publish(topic, 1, false, payload)); err != nil {
			return err
		}
		got, err := receive(ctx, backendInbox, topic)
		if err != nil {
			return err
		}
		if !bytes.Equal(got, payload) {
			return fmt.Errorf("payload changed on %s", topic)
		}
		if topic == custom {
			echo, err := receive(ctx, p.inbox, topic)
			if err != nil || !bytes.Equal(echo, payload) {
				return fmt.Errorf("device custom loopback failed")
			}
		}
		pass("device -> backend %s (raw bytes preserved)", topic)
	}
	if err := wait(backend.Publish(custom, 1, false, []byte(runID))); err != nil {
		return err
	}
	got, err := receive(ctx, p.inbox, custom)
	if err != nil {
		return err
	}
	if string(got) != runID {
		return fmt.Errorf("backend custom message differs from device payload")
	}
	pass("backend -> device custom topic")
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	fmt.Println("All device topic checks passed.")
}
