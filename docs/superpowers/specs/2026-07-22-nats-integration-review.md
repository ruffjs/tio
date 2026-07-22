# NATS 集成实现 Review

**日期：** 2026-07-22
**审查范围：** `e56f273..HEAD`（14 个提交）
**对照文档：** [NATS 集成设计](./2026-07-21-nats-integration-design.md)

> **更新（2026-07-22）：** 问题 #1、#2、#4、#5、#16、#17 已被 [Presence 重构计划](./2026-07-22-presence-refactor-plan.md) 取代。重构采用 owner-only 写 + 回调通知，去掉了 eventbus、SubscribePresence 和 CAS。下文这些问题的"建议"部分仅作为历史记录保留，实际方案以重构计划为准。

## 总体评估

核心 NATS Connector 已实现，全部测试通过（223 单元 + 16 集成）。Connector 接口、Topic 转换、内嵌 Server、MQTT Publisher、认证器、消费者适配均正确。但 **多实例协调存在严重缺失**、**规则引擎集成完全缺失**、**配置清理不完整**。

---

## 已正确实现

| 设计章节 | 状态 | 说明 |
|---|---|---|
| §4 Connector 接口 | ✅ | `Publish`/`PublishReliable`/`PublishRetained`、`Subscribe`/`QueueSubscribe`、`SubscribePresence` 均符合设计 |
| §5.2 NATS Server 启动 | ✅ | APP/SYS 双 account、JetStream、cluster routes、MQTT gateway |
| §5.4 Topic 转换 | ✅ | `foo/#` 双 subscription 展开、校验、双向转换 |
| §6.1 两阶段初始化 | ✅ | `NewConnector` -> `ConfigureAuth` -> `Start`，含失败回滚 |
| §6.2 Publisher | ✅ | 私有 MQTT publisher，唯一 client ID，QoS 1/retained |
| §6.3 Subscriber | ✅ | 广播/竞争订阅，ctx 取消时 Drain/Unsubscribe，Flush 后返回 |
| §10 main.go 流程 | ✅ | JobCenter 禁用，初始化顺序正确 |
| §11 消费者适配 | ✅ | shadow 用 `QueueSubscribe("tio-shadow")`，ntp 用 `QueueSubscribe("tio-ntp")`，method 用 `Subscribe` 接收响应 |
| §14.3 验收测试 | ✅ | 覆盖密码认证、QoS 1 投递、retained、presence、wildcard、queue subscribe、Job API 缺失 |

---

## 严重问题

### 1. Presence CAS 未实现（§7.1）

> **已被 [Presence 重构计划](./2026-07-22-presence-refactor-plan.md) 取代。** 重构后采用 owner-only 写（按 `server.name` 过滤），每个实例只写自己设备的 KV，常规情况下无竞争，不需要 CAS。仅在 failover（owner 崩溃）时由 reconciliation 写其他 owner 的条目。

**文件：** `connector/nats/presence.go:135, 187`

`handleConnectEvent` 和 `handleDisconnectEvent` 使用 `c.kv.Put()` — 直接覆盖，非 compare-and-set。

**设计要求：**
> CAS 更新 `TIO_PRESENCE` KV；CAS 失败时重新读取并按 generation 决定是否重试。只有成功改变状态的 CAS winner 继续执行 2–4，其他实例不重复发布

**影响：** 跨实例并发 connect/disconnect 事件可能互相覆盖，导致 presence 状态错误。`PresenceRecord.Generation` 字段已定义但从未用于 CAS。

**建议：** 使用 `nats.KeyValue.Update(key, value, revision)` 或 `nats.KeyValue.Create(key, value)` 实现乐观锁；CAS 失败时重读并按 generation 决定是否重试。

---

### 2. 跨实例 Presence 分发缺失（§7.1 步骤 2 & 5）

> **已被 [Presence 重构计划](./2026-07-22-presence-refactor-plan.md) 取代。** 重构后去掉 eventbus 和 `$tio.events.presence.>`，跨实例通知由 KV replication + `IsConnected()` 读 KV 实现。`SubscribePresence` 被删除，替换为 `OnLocalPresence` 回调（仅通知本进程消费者）。

**文件：** `connector/nats/presence.go:204-221`

**设计要求：**
- 步骤 2：publish `$tio.events.presence.<thingId>`，供所有 tio 实例更新本地缓存
- 步骤 5：本实例收到 `$tio.events.presence.>` 后调用进程内 eventbus 的 `Publish`

**实现现状：** `publishPresence()` 只发布到 **本地 eventbus** 和 **MQTT topic**，完全没有通过 NATS subject `$tio.events.presence.>` 分发。也没有订阅该 subject 的代码。

**影响：** 多实例部署中，实例 B 的内部消费者（shadow service、method handler 通过 `SubscribePresence`）无法收到实例 A 上设备连接/断开事件。MQTT retained topic 对设备侧可见，但进程内 eventbus 分发完全断裂。

**建议：**
1. CAS 成功后通过 `c.natsConn.Publish("$tio.events.presence."+thingId, payload)` 发布
2. `initPresence()` 中订阅 `$tio.events.presence.>`，收到后调用 `c.presenceBus.Publish()`

---

### 3. 跨实例 `Close()` 未实现（§7.2）

**文件：** `connector/nats/connectivity.go:95-123`

`Close()` 只查询 **本地** `Connz` 并断开本地连接。

**设计要求：**
> 从 KV 查 owner server/client connection ID，向 owner 的控制 subject 发 request。owner 调用 embedded server DisconnectClientByID

**影响：** 如果 thing 连接在其他实例上，`Close()` 返回 `"no connection found"` 而不实际断开。`thing.Delete()` 在多实例下会失效。

**建议：**
1. 从 KV 读取 `PresenceRecord.ServerId`
2. 向 `$tio.control.<serverId>.disconnect` 发 request（携带 thingId、connection ID、request ID）
3. owner 实例订阅自己的控制 subject，校验后调用 `DisconnectClientByID`
4. 超时返回错误，禁止直接清 KV 冒充已断开

---

### 4. `SubscribePresence` 忽略 ctx（§4, §6.5）

> **已被 [Presence 重构计划](./2026-07-22-presence-refactor-plan.md) 取代。** `SubscribePresence` 整个方法被删除，eventbus 被移除，替换为 `OnLocalPresence(handler)` 回调，不存在 channel 泄漏问题。

**文件：** `connector/nats/connectivity.go:30-32`

```go
func (c *Connector) SubscribePresence(ctx context.Context) <-chan connector.PresenceEvent {
    return c.presenceBus.Subscribe(presenceEventBusKey)
}
```

`ctx` 参数被接受但从未使用。

**设计要求：**
> 每次调用返回独立的广播订阅；ctx 结束时关闭该 channel。

**影响：** 每次 `SubscribePresence` 调用都会在 eventbus 中永久新增一个 channel，永远不会被清理。导致订阅者泄漏和内存增长。`shadow/service.go:187` 和 `shadow/method.go:216` 均受影响。

**建议：**
```go
func (c *Connector) SubscribePresence(ctx context.Context) <-chan connector.PresenceEvent {
    ch := c.presenceBus.Subscribe(presenceEventBusKey)
    go func() {
        <-ctx.Done()
        c.presenceBus.Unsubscribe(presenceEventBusKey, ch)
    }()
    return ch
}
```

---

### 5. Reconciliation 仅限本地（§7.1）

> **已被 [Presence 重构计划](./2026-07-22-presence-refactor-plan.md) 取代。** 重构后 reconciliation 只处理 failover：通过 `$SYS.REQ.SERVER.PING` 查询存活 server，将已挂 owner 的设备标记为 disconnected。不再需要跨节点 CONNZ 聚合或两轮逻辑。

**文件：** `connector/nats/presence.go:236-302`

`reconcile()` 只调用 `c.natsSvr.Server().Connz()` — 仅查询 **本节点** 的连接。

**设计要求：**
> 每 10 秒通过 system service 请求所有节点的 account `CONNZ`，聚合当前 MQTT connections，并与 `TIO_PRESENCE` KV 比较

**缺失项：**
- 没有跨节点查询（应通过 NATS monitoring service 请求 `$SYS.REQ.SERVER.PING` 或逐节点 CONNZ）
- 没有"连续两轮均不存在才判定 disconnected"的两轮逻辑 — 当前实现立即标记为 disconnected
- 没有启动时的强制全量 reconciliation（设计要求："启动完成后必须先做一次全量 reconciliation，再向业务层报告 ready"）

**影响：** 单实例下功能正确；多实例下无法检测其他节点上的连接，reconciliation 失效。

---

### 6. 规则引擎 Source/Sink 完全缺失（§8, §15 步骤 5）

**缺失文件：** `rule/source/nats.go`、`rule/sink/nats.go`

旧的 `rule/source/embedmqtt.go`、`rule/source/mqtt.go`、`rule/sink/embedmqtt.go`、`rule/sink/mqtt.go` 已删除，但替代的 NATS source/sink 从未创建。`rule/source/` 目录现在只剩 `source.go`（注册表），无任何已注册的 source 类型。

**影响：** 规则引擎无法从设备 topic 摄取消息 — 核心功能断裂。

**设计要求：**
- Source：使用 `QueueSubscribe`，queue 为 `tio-rule-<ruleId>`，MQTT topic 内部转换为 NATS subject
- Sink：配置指定目标 MQTT topic 和 delivery（core/qos1/retained），分别调用 `Publish`/`PublishReliable`/`PublishRetained`

---

### 7. ACL 函数从未调用（§5.3）

**文件：** `connector/nats/authenticator.go:186-211`

`NatsAuthenticator.aclFn` 字段已存储但从未使用。`thingPermissions()` 生成静态权限：

```go
thingPrefix := "$iothub.things." + thingId + ".>"
userPrefix := "$iothub.user.things." + thingId + ".>"
```

**设计要求：**
> 根据 `TopicAcl` 的结果生成最小 NATS permissions

**影响：** Gateway 绑定功能失效 — gateway thing 无法访问其绑定 things 的 topic。`auth.TopicAcl()` 中通过 `bg.IsBoundGateway()` 判断的逻辑完全被绕过。设计还要求"绑定关系改变后强制 gateway 重连并重新生成 permissions"，当前实现无法做到。

**建议：** 在 `thingPermissions()` 中调用 `aclFn` 或查询绑定关系，将绑定的 things 的 subject 加入 permissions。

---

## 配置问题

### 8. 默认配置仍为 `type: embed`（§9.2）

**文件：** `config.default.yaml:27`

```yaml
connector:
  type: embed    # ← 应为 "nats"
```

embed connector 已删除，但默认配置仍为 embed，且保留了完整的 `mqttClient`/`mqttBroker`/`emqx` 配置块。虽然 `main.go` 忽略 type 字段直接创建 NATS connector，但配置具有误导性。

---

### 9. 旧配置类型未清理（§9.1）

**文件：** `config/config.go:36-39, 62-78, 129-161`

仍保留以下已废弃的类型和常量：
- `ConnectorEmqx`、`ConnectorMqttEmbed` 常量
- `InnerMqttBroker`、`InnerMqttStorage`、`MqttClientConfig`、`EmqxAdapterConfig`、`Redis` 类型
- `Connector` struct 仍有 `MqttClient`、`MqttBroker`、`Emqx` 字段

**设计要求（§9.1）：** `Connector` 只应包含 `Typ` 和 `Nats` 两个字段。

---

### 10. 死代码：`metrics/mqtthook.go`

**文件：** `metrics/mqtthook.go`

仍导入 `github.com/mochi-mqtt/server/v2`，实现了一个 `OpenMetricsHook`，但 mochi broker 已删除，此 hook 永远不会被注册。这是 `go.mod` 中保留 `mochi-mqtt v2.7.9` 依赖的唯一原因。

**建议：** 删除此文件，用 NATS server 的 `Varz`/`Connz` 或 JetStream metrics 替代。

---

### 11. 旧 docker-compose 未更新

**文件：** `build/docker/docker-compose.yaml`

仍引用 EMQX 作为独立服务。应更新为单节点 NATS 配置或删除。

---

## 次要问题

### 12. KV bucket 校验缺失（§5.2）

`presence.go:68-74` 使用 `CreateKeyValue` 且 `History: 5`，但未设置 replicas 或 TTL。

**设计要求：**
> 已有 bucket 的 replicas、history 或 TTL 不匹配时启动失败，不能静默沿用

`PresenceReplicas` 配置字段存在但从未传入 KV config。应使用 `nats.KeyValueConfig` 的 `Replicas` 和 `TTL` 字段，并在 bucket 已存在时校验配置是否匹配。

---

### 13. `Remove()` 忽略所有 Close 错误（§6.5）

**文件：** `connector/nats/connectivity.go:125-138`

```go
_ = c.Close(thingId)
```

**设计要求：**
```go
if err := c.Close(thingId); err != nil && !errors.Is(err, ErrNotConnected) { return err }
```

当前实现忽略所有错误，且 `ErrNotConnected` 从未定义。

---

### 14. 集群配置缺少 `clusterAdvertise`（§7.3）

**文件：** `deployments/nats-cluster/configs/node-*.yaml`

容器/NAT 环境必须配置 `clusterAdvertise`，否则 cluster route 无法正确建立。当前三个节点配置均未设置。

---

### 15. 集群配置缺少 route mTLS（§7.3）

**设计要求：**
> route 连接必须配置独立凭据和 mTLS；不能复用设备或 system client 凭据

当前集群配置使用明文 `nats://` routes，`routeTls` 块为空。

---

### 16. 启动时无全量 reconciliation（§7.1）

> **已被 [Presence 重构计划](./2026-07-22-presence-refactor-plan.md) 部分取代。** 重构后 reconciliation 的职责从"全量比对 KV 与实际连接"变为"检测已挂 owner server 并清理 stale 条目"。启动时仍应执行一次 reconciliation（检测此前崩溃的 owner），但不需要全量 CONNZ 聚合。

**设计要求：**
> 启动完成后必须先做一次全量 reconciliation，再向业务层报告 ready

`initPresence()` 只启动定时 ticker，未在 ready 前执行一次全量 reconciliation。

---

### 17. EventBus 慢消费者策略不完整（§7.1）

> **已被 [Presence 重构计划](./2026-07-22-presence-refactor-plan.md) 取代。** eventbus 从 connector 中完全移除，不再有慢消费者问题。`OnLocalPresence` 回调是同步单点通知，不存在 fan-out。

**文件：** `pkg/eventbus/eventbus.go:48-60`

**设计要求：**
> 每个订阅者使用有界 buffer；buffer 满时记录 subscriber 名称和指标，并触发该订阅者下一轮从 KV 全量同步

当前实现：buffer 大小 10000，满时 1 秒超时后只记录日志，未记录 subscriber 名称和指标，未触发 KV 全量同步。

---

## 问题优先级汇总

> 标记 **[重构]** 的问题已被 [Presence 重构计划](./2026-07-22-presence-refactor-plan.md) 取代，不需要在当前实现中修复。

| 优先级 | 编号 | 问题 | 影响范围 | 状态 |
|---|---|---|---|---|
| **P0** | #6 | 规则引擎 source/sink 缺失 | 规则引擎无法摄取设备消息 | 待修复 |
| **P0** | #1 | Presence CAS 未实现 | 多实例 presence 状态损坏 | **[重构]** 取代 |
| **P0** | #2 | 跨实例 presence 分发缺失 | 多实例下内部消费者收不到跨节点事件 | **[重构]** 取代 |
| **P0** | #3 | 跨实例 Close 未实现 | 多实例下 thing.Delete 失效 | 待修复 |
| **P1** | #4 | SubscribePresence 忽略 ctx | 订阅者泄漏 | **[重构]** 取代 |
| **P1** | #5 | Reconciliation 仅限本地 | 多实例下状态修正失效 | **[重构]** 取代 |
| **P1** | #7 | ACL 函数未调用 | Gateway 绑定功能失效 | 待修复 |
| **P2** | #8 | 默认配置 type 错误 | 配置误导 | 待修复 |
| **P2** | #9 | 旧配置类型未清理 | 代码冗余 | 待修复 |
| **P2** | #10 | metrics/mqtthook.go 死代码 | 不必要的依赖保留 | 待修复 |
| **P2** | #11 | 旧 docker-compose 未更新 | 部署文档误导 | 待修复 |
| **P3** | #12 | KV bucket 校验缺失 | 生产就绪性 | 待修复 |
| **P3** | #13 | Remove() 忽略 Close 错误 | 错误处理不完整 | 待修复 |
| **P3** | #14 | 集群配置缺少 clusterAdvertise | 集群路由无法建立 | 待修复 |
| **P3** | #15 | 集群配置缺少 route mTLS | 安全性 | 待修复 |
| **P3** | #16 | 启动时无全量 reconciliation | 状态修正延迟 | **[重构]** 部分取代 |
| **P3** | #17 | EventBus 慢消费者策略 | 生产就绪性 | **[重构]** 取代 |

---

## 附录：嵌入式 NATS Server 监控能力评估

### 嵌入 vs. 外置的监控差异

嵌入式 NATS server 与 standalone 使用**同一份代码**，监控能力完全相同。唯一区别是谁管理进程生命周期。server 内部的 monitoring HTTP server、system request handler、JetStream management API 全部正常工作。

### 各监控工具兼容性

| 工具 | 协议 | 嵌入式可用性 | 说明 |
|---|---|---|---|
| **nats-top** | HTTP (`/varz`, `/connz`, `/subsz`...) | ✅ 可用 | 需设置 `opts.HTTPPort`（如 8222），当前未配置 |
| **Prometheus exporter** | HTTP (scrape `/varz` 等) | ✅ 可用 | 同上，需开启 HTTP monitor port |
| **survey** | NATS 协议 (`$SYS.REQ.SERVER.*`) | ✅ 可用 | SystemAccount 已配置为 `TIO_SYS`，用 sysConn 凭据连接即可，不依赖 HTTP 端口 |

### 当前实现状态

`connector/nats/server.go:86-95` 中未设置 `HTTPPort`/`MonitorPort`，HTTP 监控端点当前关闭。survey 走 NATS 协议内的 `$SYS.REQ.*` system request，不依赖 HTTP 端口，SystemAccount 已存在，当前可用。

### 建议

在 `NatsServerConfig` 中新增 `MonitorPort` 字段并在 `buildServerOptions` 中设置，以便运维团队可选用 nats-top 和 Prometheus exporter：

```go
// config/config.go - NatsServerConfig
MonitorPort int `json:"monitorPort"` // NATS HTTP monitoring port (默认 8222)

// connector/nats/server.go - buildServerOptions
opts.HTTPPort = cfg.MonitorPort
```

默认配置中可设为 8222 或 0（关闭）。多实例部署时每个节点使用不同端口映射。

### 嵌入式 vs. 外置 NATS 监控总结

以下监控能力**不因嵌入/外置而改变**：

- HTTP monitoring endpoints（`/varz`, `/connz`, `/subsz`, `/routez`, `/accountz` 等）
- System request 响应（`$SYS.REQ.*` subjects）
- JetStream management API（`$JS.API.*`）
- Server profiling（`opts.ProfPort`）
- System events 发布（`$SYS.ACCOUNT.*.CONNECT/DISCONNECT`）

嵌入式模式下唯一需要做的是显式开启 HTTP monitor port。不存在"嵌入模式砍掉了监控"的问题。
