# Presence 系统重构计划

**日期：** 2026-07-22
**状态：** Draft
**前置：** [NATS 集成设计](./2026-07-21-nats-integration-design.md)、[实现 Review](./2026-07-22-nats-integration-review.md)

## 1. 背景

当前 Presence 系统路径过长：

```
系统事件(所有实例) -> KV Put -> eventbus -> SubscribePresence channel -> 消费者
                                    └── 多实例断裂（eventbus 是进程内的）
```

问题：
- 状态管理扩散：所有实例都处理所有设备的系统事件、都写 KV、都写 DB
- `eventbus` 是多余中转层：系统事件已跨实例分发，eventbus 又做一次本地 fan-out但跨实例断裂
- `SubscribePresence` 忽略 ctx，channel 永不清理，订阅者泄漏
- `shadow/method.go` 的等待上线逻辑（waiting map + channel）比 method 请求本身还复杂

## 2. 方案概述

核心原则：**owner-only 写，回调通知，去掉 eventbus 和 SubscribePresence。**

```
设备连接到实例 A
  │
  ▼
实例 A 处理自己的系统事件 (server.name 过滤):
  ├── KV Put("presence.<thingId>")         ← 全局状态，唯一写入者
  ├── DB UpdateConnStatus (timestamp 保护)  ← 本地 DB
  ├── MQTT Publish(retained + event)        ← 设备面向
  └── callback(handler)                     ← 通知本进程消费者

其他实例:
  ├── 不处理（不是自己的设备）
  ├── IsConnected() -> 读 KV
  └── AllClientInfo() -> 遍历 KV
```

消除：eventbus、SubscribePresence、`$tio.events.presence.>`、CAS（常规）、waiting map。

## 3. 接口变更

### 3.1 删除

```go
// 从 ConnectChecker 中删除
SubscribePresence(ctx context.Context) <-chan PresenceEvent
```

### 3.2 新增

```go
// connector/connector.go

// PresenceHandler 处理本实例设备的上线/下线事件。
// 在 NATS 系统事件回调中被同步调用；如需异步，handler 内部自行 spawn goroutine。
type PresenceHandler func(ci ClientInfo)

type Connectivity interface {
    ConnectChecker
    Start(ctx context.Context) error
    Close(thingId string) error
    Remove(thingId string) error
    OnLocalPresence(handler PresenceHandler)
}

type ConnectChecker interface {
    IsConnected(thingId string) (bool, error)
    ClientInfo(thingId string) (ClientInfo, error)
    AllClientInfo() ([]ClientInfo, error)
}
```

### 3.3 保留不变

- `PresenceEvent` struct 和 `EventConnected`/`EventDisconnected` 常量 — 仍用于 MQTT topic payload
- `TopicPresence`/`TopicPresenceEvent` 常量 — 设备面向的 MQTT topic
- `ClientInfo` struct — DB 更新和 API 查询用

### 3.4 变更对照

| 旧 | 新 | 说明 |
|---|---|---|
| `SubscribePresence(ctx) <-chan PresenceEvent` | `OnLocalPresence(handler)` | 回调替代 channel |
| `eventbus.EventBus[PresenceEvent]` | 删除 | 不再需要 |
| `PresenceEvent` 用于内部通知 | `ClientInfo` 用于回调 | 回调直接传 `ClientInfo` |
| `PresenceEvent` 用于 MQTT payload | 保留 | 设备面向 |
| `toClientInfo(PresenceEvent)` | 删除 | 回调直接传 `ClientInfo`，无需转换 |

## 4. 实现细节

### 4.1 `connector/connector.go`

```go
type PresenceHandler func(ci ClientInfo)

type ConnectChecker interface {
    IsConnected(thingId string) (bool, error)
    ClientInfo(thingId string) (ClientInfo, error)
    AllClientInfo() ([]ClientInfo, error)
}

type Connectivity interface {
    ConnectChecker
    Start(ctx context.Context) error
    Close(thingId string) error
    Remove(thingId string) error
    OnLocalPresence(handler PresenceHandler)
}
```

删除 `SubscribePresence`。保留 `PresenceEvent`、`EventConnected`、`EventDisconnected` 用于 MQTT payload。

### 4.2 `connector/nats/connector.go`

```go
type Connector struct {
    // ...existing fields...
    presenceHandler connector.PresenceHandler
}

func (c *Connector) OnLocalPresence(handler connector.PresenceHandler) {
    c.presenceHandler = handler
}
```

### 4.3 `connector/nats/presence.go`

**核心变更：owner-only 过滤 + 回调通知 + 去掉 eventbus。**

```go
func (c *Connector) initPresence() error {
    kv, err := c.js.CreateKeyValue(&nats.KeyValueConfig{
        Bucket:   presenceKVBucket,
        History:  5,
        Replicas: c.cfg.Server.PresenceReplicas,
    })
    if err != nil {
        return fmt.Errorf("create presence KV bucket: %w", err)
    }
    c.kv = kv

    if _, err := c.sysConn.Subscribe(sysConnectSubj, c.handleConnectEvent); err != nil {
        return fmt.Errorf("subscribe connect events: %w", err)
    }
    if _, err := c.sysConn.Subscribe(sysDisconnectSubj, c.handleDisconnectEvent); err != nil {
        return fmt.Errorf("subscribe disconnect events: %w", err)
    }
    if err := c.sysConn.Flush(); err != nil {
        return fmt.Errorf("flush presence subscriptions: %w", err)
    }

    go c.startReconciliation(c.ctx)
    return nil
}

func (c *Connector) handleConnectEvent(msg *nats.Msg) {
    var evt sysConnectEvent
    if err := json.Unmarshal(msg.Data, &evt); err != nil {
        slog.Error("parse connect event", "error", err)
        return
    }

    // 只处理连接到本实例的设备
    if evt.Server.Name != c.cfg.Server.ServerName {
        return
    }

    thingId := evt.Client.User
    if thingId == "" || strings.HasPrefix(thingId, "$") {
        return
    }

    now := time.Now()
    rec := PresenceRecord{
        ThingId:    thingId,
        Connected:  true,
        ServerId:   evt.Server.Name,
        ClientId:   evt.Client.MQTTClient,
        RemoteAddr: evt.Client.Host,
        Timestamp:  now.UnixMilli(),
    }

    // 更新 KV（全局状态，owner 唯一写入者，无需 CAS）
    data, err := json.Marshal(rec)
    if err != nil {
        slog.Error("marshal presence record", "error", err)
        return
    }
    if _, err := c.kv.Put("presence."+thingId, data); err != nil {
        slog.Error("put presence record", "key", "presence."+thingId, "error", err)
        return
    }

    // 发布设备面向的 MQTT topic
    c.publishMqttPresence(thingId, now, connector.EventConnected, rec)

    // 通知本进程消费者（shadow service 更新 DB）
    if c.presenceHandler != nil {
        c.presenceHandler(connector.ClientInfo{
            ClientId:    thingId,
            Username:    thingId,
            Connected:   true,
            ConnectedAt:  &now,
            RemoteAddr:  evt.Client.Host,
        })
    }
}

func (c *Connector) handleDisconnectEvent(msg *nats.Msg) {
    var evt sysDisconnectEvent
    if err := json.Unmarshal(msg.Data, &evt); err != nil {
        slog.Error("parse disconnect event", "error", err)
        return
    }

    // 只处理连接到本实例的设备
    if evt.Server.Name != c.cfg.Server.ServerName {
        return
    }

    thingId := evt.Client.User
    if thingId == "" || strings.HasPrefix(thingId, "$") {
        return
    }

    now := time.Now()
    rec := PresenceRecord{
        ThingId:    thingId,
        Connected:  false,
        ServerId:   evt.Server.Name,
        ClientId:   evt.Client.MQTTClient,
        RemoteAddr: evt.Client.Host,
        Timestamp:  now.UnixMilli(),
    }

    data, err := json.Marshal(rec)
    if err != nil {
        slog.Error("marshal presence record", "error", err)
        return
    }
    if _, err := c.kv.Put("presence."+thingId, data); err != nil {
        slog.Error("put presence record on disconnect", "key", "presence."+thingId, "error", err)
        return
    }

    c.publishMqttPresence(thingId, now, connector.EventDisconnected, rec)

    if c.presenceHandler != nil {
        c.presenceHandler(connector.ClientInfo{
            ClientId:         thingId,
            Username:         thingId,
            Connected:        false,
            DisconnectedAt:   &now,
            DisconnectReason: evt.Reason,
            RemoteAddr:       evt.Client.Host,
        })
    }
}

// publishMqttPresence 发布设备面向的 retained + event MQTT topic。
func (c *Connector) publishMqttPresence(thingId string, ts time.Time, eventType string, rec PresenceRecord) {
    payload, _ := json.Marshal(connector.PresenceEvent{
        Timestamp:  ts.UnixMilli(),
        EventType:  eventType,
        ThingId:    thingId,
        ClientId:   rec.ClientId,
        RemoteAddr: rec.RemoteAddr,
    })

    if c.mqttPub != nil {
        if err := c.mqttPub.Publish(connector.TopicPresenceEvent(thingId), 1, false, payload); err != nil {
            slog.Error("publish presence event via MQTT", "thingId", thingId, "error", err)
        }
        if err := c.mqttPub.Publish(connector.TopicPresence(thingId), 1, true, payload); err != nil {
            slog.Error("publish retained presence via MQTT", "thingId", thingId, "error", err)
        }
    }
}
```

**删除：** `presenceBus` 字段、`eventbus` import、`publishPresence` 方法中的 `presenceBus.Publish` 调用。

### 4.4 `connector/nats/connectivity.go`

```go
// SubscribePresence 方法删除。
// IsConnected / ClientInfo / AllClientInfo 保持不变（读 KV）。
```

### 4.5 `connector/nats/presence.go` — Reconciliation

Reconciliation 只处理 failover：检测已挂的 owner server，将其设备标记为 disconnected。

```go
func (c *Connector) reconcile() {
    if c.natsSvr == nil || c.kv == nil {
        return
    }

    // 查询集群中所有存活 server
    aliveServers := c.queryAliveServers()

    keys, err := c.kv.Keys()
    if err != nil && err != nats.ErrNoKeysFound {
        slog.Error("reconcile: get KV keys", "error", err)
        return
    }

    for _, key := range keys {
        entry, err := c.kv.Get(key)
        if err != nil {
            continue
        }
        var rec PresenceRecord
        if json.Unmarshal(entry.Value(), &rec) != nil {
            continue
        }

        // 只处理 owner server 已挂的 stale 条目
        if rec.Connected && !aliveServers[rec.ServerId] {
            now := time.Now()
            rec.Connected = false
            rec.Timestamp = now.UnixMilli()
            data, _ := json.Marshal(rec)
            if _, err := c.kv.Put(key, data); err != nil {
                slog.Error("reconcile: put stale presence", "key", key, "error", err)
                continue
            }

            // 更新 MQTT retained
            c.publishMqttPresence(rec.ThingId, now, connector.EventDisconnected, rec)

            // 通知本进程消费者
            if c.presenceHandler != nil {
                c.presenceHandler(connector.ClientInfo{
                    ClientId:         rec.ThingId,
                    Username:         rec.ThingId,
                    Connected:        false,
                    DisconnectedAt:   &now,
                    DisconnectReason: "owner server unavailable",
                    RemoteAddr:       rec.RemoteAddr,
                })
            }
        }
    }
}

// queryAliveServers 通过 NATS system request 查询集群中所有存活 server。
func (c *Connector) queryAliveServers() map[string]bool {
    result := make(map[string]bool)

    // 方式 1：通过本地 server 的 Varz 获取集群信息
    if c.natsSvr != nil {
        v, err := c.natsSvr.Server().Varz(nil)
        if err == nil {
            result[v.Name] = true
        }
    }

    // 方式 2：通过 system request 查询所有节点
    // 向 $SYS.REQ.SERVER.PING 发 request，收集响应
    reply, err := c.sysConn.Request("$SYS.REQ.SERVER.PING", nil, 2*time.Second)
    if err == nil {
        var servers struct {
            Servers []struct {
                Name string `json:"name"`
            } `json:"servers"`
        }
        if json.Unmarshal(reply.Data, &servers) == nil {
            for _, s := range servers.Servers {
                result[s.Name] = true
            }
        }
    }

    // 降级：如果 system request 失败，至少包含本节点
    if len(result) == 0 && c.natsSvr != nil {
        v, _ := c.natsSvr.Server().Varz(nil)
        if v != nil {
            result[v.Name] = true
        }
    }

    return result
}
```

### 4.6 `shadow/service.go`

**变更：** 删除 `SubscribePresence`，改用 callback；保留启动时全量同步。

```go
func (s *shadowSvc) Init(ctx context.Context) {
    s.ctx = ctx
    s.doFirstSyncStatus(ctx)
}

// HandleLocalPresence 由 connector 回调调用，处理本实例设备的上线/下线。
func (s *shadowSvc) HandleLocalPresence(ci connector.ClientInfo) {
    if err := s.repo.UpdateConnStatus(s.ctx, []connector.ClientInfo{ci}); err != nil {
        slog.Error("update conn status from callback", "thingId", ci.ClientId, "error", err)
        return
    }
    s.cache.UpdateConnStatus(ci.ClientId, ci)
}

// 删除 syncConnStatus 方法（不再需要 SubscribePresence channel + goroutine）。
// 删除 toClientInfo 方法（回调直接传 ClientInfo，无需转换）。
// doFirstSyncStatus 保持不变。
```

`Repo.UpdateConnStatus` 已有 timestamp 保护（`shadow_repo.go:96-99`），无需修改：

```sql
WHERE (connected_at IS NULL OR connected_at <= ?)
  AND (disconnected_at IS NULL OR disconnected_at <= ?)
```

### 4.7 `shadow/method.go`

**变更：** 离线设备直接返回错误，删除全部等待逻辑。

```go
func (h *mqttMethod) InvokeMethod(
    ctx context.Context,
    msg MethodReqMsg,
) (MethodResp, error) {
    online, err := h.connector.IsConnected(msg.ThingId)
    if err != nil {
        return MethodResp{}, errors.WithMessage(err, "could not get online status")
    }
    if !online {
        return MethodResp{}, model.ErrDirectMethodThingOffline
    }
    return h.doInvokeMethod(ctx, msg)
}
```

**删除：**
- `waiting` map 字段
- `addWaiting` / `removeWaiting` 方法
- `subscribeThingOnline` 方法
- `InitMethodHandler` 中的 `subscribeThingOnline` 调用
- `ConnTimeout` 字段的使用（保留 struct 字段但忽略，或标记 deprecated）

**保留：**
- `pending` map 和 `subscribeMethodResp`（等待设备响应仍需要）
- `doInvokeMethod`（发送请求 + 等待响应）

### 4.8 `job/runner.go`

**变更：** 删除 `SubscribePresence` 调用，改为 dispatch 时检查 `IsConnected`。

```go
func (r *runnerImpl) sysOpTaskLoop(addCh <-chan []Task, delCh <-chan deleteTaskMsg) {
    // 删除: onConn := r.conn.SubscribePresence(r.ctx)
    // 删除: case e := <-onConn: ...

    // 在 dispatch 时检查在线状态（已有逻辑，见 lines 323-330）
    // 离线设备的 task 放入 offlineThingTasks
    // 保留 offlineThingTasks map，但在 dispatch 循环中定期重试 IsConnected
    // 而非等待事件通知

    tick := time.NewTicker(time.Millisecond * 50)
    retryTick := time.NewTicker(time.Second * 2) // 定期重试离线设备
    for {
        select {
        case <-r.ctx.Done():
            return
        // ... existing cases ...
        case <-retryTick.C:
            // 重试离线设备：检查是否有已重新上线的
            for thingId, tasks := range offlineThingTasks {
                if online, _ := r.conn.IsConnected(thingId); online {
                    delete(offlineThingTasks, thingId)
                    for _, t := range tasks {
                        curQ.Push(&t)
                    }
                }
            }
        case <-tick.C:
            // ... existing dispatch logic ...
        }
    }
}
```

> 注：JobCenter 当前已禁用。此变更在 JobCenter 重新启用前不是阻塞性的，但一并修改以消除对 `SubscribePresence` 的依赖。

### 4.9 `connector/mock/connector.go`

**变更：** 删除 eventbus，`SimulatePresence` 改为调用回调。

```go
type MockConnector struct {
    // ...existing fields...
    // 删除: presenceBus *eventbus.EventBus[connector.PresenceEvent]
    presenceHandler connector.PresenceHandler
}

func NewMockConnector() *MockConnector {
    return &MockConnector{
        connectedThings: make(map[string]bool),
        queueCounters:   make(map[string]*atomic.Uint64),
    }
}

// 删除 SubscribePresence 方法。
// 新增 OnLocalPresence:
func (m *MockConnector) OnLocalPresence(handler connector.PresenceHandler) {
    m.presenceHandler = handler
}

// SimulatePresence 改为直接调用回调:
func (m *MockConnector) SimulatePresence(ci connector.ClientInfo) {
    if m.presenceHandler != nil {
        m.presenceHandler(ci)
    }
}

// SetConnected 保持不变，用于 IsConnected 测试。
```

### 4.10 `cmd/tio/main.go`

```go
// 在 conn.Start(ctx) 之后、shadowSvc.Init(ctx) 之前注册回调:
natsConnector.OnLocalPresence(func(ci connector.ClientInfo) {
    go shadowSvc.HandleLocalPresence(ci)
})
```

### 4.11 `shadow/mock/mock_connector.go`

更新 mock 以匹配新接口（删除 `SubscribePresence`，新增 `OnLocalPresence`）。

## 5. 测试变更

### 5.1 `shadow/method_test.go`

**当前：** 测试 `ConnTimeout > 0` 时设备上线后唤醒等待请求。

**改为：** 测试离线设备直接返回错误。

```go
func TestInvokeMethodOfflineReturnsError(t *testing.T) {
    mc := mock.NewMockConnector()
    mc.SetConnected(thingId, false)
    h := shadow.NewMethodHandler(mc)

    _, err := h.InvokeMethod(ctx, MethodReqMsg{
        ThingId: thingId,
        Method:  "test",
        Req:     MethodReq{ClientToken: "token"},
    })

    require.ErrorIs(t, err, model.ErrDirectMethodThingOffline)
}
```

删除所有 `SimulatePresence(EventConnected)` 相关测试。

### 5.2 `shadow/service_test.go`

**当前：** 通过 `SubscribePresence` channel 驱动 DB 更新。

**改为：** 直接调用 `HandleLocalPresence`。

```go
func TestHandleLocalPresenceUpdatesDB(t *testing.T) {
    svc := newTestSvc(db)
    now := time.Now()
    svc.HandleLocalPresence(connector.ClientInfo{
        ClientId:   thingId,
        Connected:  true,
        ConnectedAt: &now,
    })

    // 验证 DB 已更新
    sw, _ := svc.Get(ctx, thingId)
    require.True(t, sw.ConnStatus.Connected)
}
```

### 5.3 `connector/nats/presence_test.go`

**新增：** 验证 owner-only 过滤。

```go
func TestOnlyOwnServerEventsProcessed(t *testing.T) {
    // 模拟来自其他 server 的 connect event
    // 验证 KV 未被更新、回调未被调用
}

func TestOwnServerEventUpdatesKVAndCallback(t *testing.T) {
    // 模拟来自本 server 的 connect event
    // 验证 KV 已更新、回调被调用
}
```

### 5.4 `connector/mock/connector_test.go`

删除 `SubscribePresence` 相关测试。新增 `OnLocalPresence` + `SimulatePresence` 测试。

### 5.5 `integration_tests/nats_single_node_test.go`

`TestPresenceConnectDisconnect` 和 `TestMultiplePresenceSubscribers` 不需要修改 — 它们测试的是 MQTT topic 级别的 presence，不依赖 `SubscribePresence`。

### 5.6 集成测试：DB 竞争保护

```go
func TestDBTimestampProtection(t *testing.T) {
    // 1. 设备连接到本实例，验证 DB 显示 connected
    // 2. 设备断开，验证 DB 显示 disconnected
    // 3. 手动插入一条更旧的 connected 记录
    // 4. 验证 DB 仍显示 disconnected（旧写入被拒绝）
}
```

## 6. 删除清单

| 文件 | 删除内容 |
|---|---|
| `connector/connector.go` | `SubscribePresence` from `ConnectChecker` |
| `connector/nats/connector.go` | `presenceBus` 字段 |
| `connector/nats/presence.go` | `presenceBus` 初始化、`publishPresence` 中的 eventbus 调用 |
| `connector/nats/connectivity.go` | `SubscribePresence` 方法 |
| `connector/mock/connector.go` | `presenceBus` 字段、`SubscribePresence` 方法 |
| `shadow/service.go` | `syncConnStatus`、`toClientInfo`、`SubscribePresence` 调用 |
| `shadow/method.go` | `waiting` map、`addWaiting`/`removeWaiting`、`subscribeThingOnline` |
| `job/runner.go` | `SubscribePresence` 调用和 `case e := <-onConn` 分支 |
| `shadow/mock/mock_connector.go` | `SubscribePresence` stub |
| `integration_tests/mtls_helpers_test.go` | `SubscribePresence` wrapper（改为直接用 `IsConnected`） |

## 7. 验收标准

### 7.1 功能验收

| 编号 | 验收项 | 验证方法 |
|---|---|---|
| A1 | 设备连接到本实例后，KV 中 `presence.<thingId>` 显示 connected | 连接设备后读 KV |
| A2 | 设备连接到本实例后，DB `conn_status` 表显示 connected | 连接设备后查 DB |
| A3 | 设备连接到本实例后，MQTT retained topic 有最新 presence payload | MQTT 订阅后收到 retained |
| A4 | 设备断开后，KV、DB、MQTT retained 均更新为 disconnected | 断开后检查三个状态源 |
| A5 | 非本实例的系统事件不被处理 | 模拟其他 server 的事件，验证 KV/DB/回调无变化 |
| A6 | `IsConnected()` 返回与 KV 一致的状态 | 连接/断开后调用 |
| A7 | `AllClientInfo()` 返回 KV 中所有 presence 条目 | 多设备连接后调用 |
| A8 | `ClientInfo(thingId)` 返回 KV 中对应条目 | 连接后调用 |

### 7.2 Method 验收

| 编号 | 验收项 | 验证方法 |
|---|---|---|
| B1 | 离线设备调用 direct method 返回 `ErrDirectMethodThingOffline` | `IsConnected=false` 时调用 |
| B2 | 在线设备调用 direct method 正常发送请求并收到响应 | 设备连接后调用 |
| B3 | 无 `waiting` map 相关代码 | grep 确认删除 |
| B4 | 无 `subscribeThingOnline` 方法 | grep 确认删除 |

### 7.3 DB 竞争保护验收

| 编号 | 验收项 | 验证方法 |
|---|---|---|
| C1 | 旧 timestamp 的 DB 写入被 WHERE 子句拒绝 | 插入旧 timestamp 记录，验证行未更新 |
| C2 | 新 timestamp 的 DB 写入正常生效 | 插入新 timestamp 记录，验证行已更新 |
| C3 | 设备快速断开-重连到另一实例，DB 最终显示 connected | 模拟跨实例场景 |

### 7.4 接口清理验收

| 编号 | 验收项 | 验证方法 |
|---|---|---|
| D1 | `ConnectChecker` 接口无 `SubscribePresence` | 编译检查 |
| D2 | 无 `eventbus` import 在 `connector/nats/` 和 `connector/mock/` | grep 确认 |
| D3 | 无 `SubscribePresence` 调用残留 | grep 确认 |
| D4 | mock connector 实现 `OnLocalPresence` | 编译检查 |
| D5 | NATS connector 实现 `OnLocalPresence` | 编译检查 |

### 7.5 Reconciliation 验收

| 编号 | 验收项 | 验证方法 |
|---|---|---|
| E1 | owner server 挂掉后，reconciliation 将其设备标记为 disconnected | 模拟 owner 不可用 |
| E2 | 存活 server 的设备不被误判为 disconnected | 模拟 owner 可用 |
| E3 | reconciliation 标记 disconnected 后触发 MQTT retained 更新和回调 | 验证 MQTT + 回调 |

### 7.6 测试验收

| 编号 | 验收项 |
|---|---|
| F1 | `go build ./...` 通过 |
| F2 | `go test ./...` 全部通过 |
| F3 | `go test ./integration_tests/` 全部通过 |
| F4 | 新增 owner-only 过滤测试通过 |
| F5 | 新增 DB timestamp 保护测试通过 |
| F6 | 新增 method 离线拒绝测试通过 |

## 8. 实施顺序

1. **`connector/connector.go`** — 修改接口（删除 `SubscribePresence`，新增 `OnLocalPresence`、`PresenceHandler`）
2. **`connector/nats/connector.go`** — 添加 `presenceHandler` 字段和 `OnLocalPresence` 方法
3. **`connector/nats/presence.go`** — owner-only 过滤、回调通知、删除 eventbus、重构 reconciliation
4. **`connector/nats/connectivity.go`** — 删除 `SubscribePresence` 方法
5. **`connector/mock/connector.go`** — 删除 eventbus、更新 `SimulatePresence`、实现 `OnLocalPresence`
6. **`shadow/service.go`** — 删除 `syncConnStatus`、新增 `HandleLocalPresence`、删除 `toClientInfo`
7. **`shadow/method.go`** — 简化 `InvokeMethod`、删除 waiting 逻辑
8. **`job/runner.go`** — 删除 `SubscribePresence`、改为轮询重试
9. **`shadow/mock/mock_connector.go`** — 更新 mock 接口
10. **`cmd/tio/main.go`** — 注册 `OnLocalPresence` 回调
11. **测试更新** — 按第 5 节修改所有受影响测试
12. **运行全部测试** — 确认 `go build ./...` 和 `go test ./...` 通过
