# Tio Simple 设备协议

Simple 协议是 legacy 设备协议的替代模式，也可以在 `dual` 模式下与 legacy 并存，由每个 Thing 的通信配置选择：

```yaml
protocol:
  mode: dual         # legacy、simple 或 dual，默认 legacy
  defaultEncoding: json     # json 或 cbor，默认 json
```

空值和未知值都会导致 Tio 拒绝启动。`mode: simple` 只注册 Simple MQTT Topic；
`mode: legacy` 只注册 Legacy 设备 Topic；`mode: dual` 同时注册两套 Topic，并按
`Thing.messageProtocol` 选择处理器。三种模式均优先使用 `Thing.payloadEncoding`，为空时使用 `protocol.defaultEncoding`；共享 Thing、Shadow 和 Method HTTP API。

可运行的设备交互测试见 [Simple 示例](../demos/simple/README.md)，覆盖服务端 Method、Shadow、NTP 和多 Topic 收发。

## 设备通信配置

通过 `POST /api/v1/things/` 创建、`PATCH /api/v1/things/{thingId}` 更新通信字段，配置持久化到 Thing，重连后沿用：

| 字段 | 可选值 | 空字符串含义 |
|---|---|---|
| `messageProtocol` | `legacy`、`simple` | Dual 模式使用 Legacy；单协议模式由部署 `mode` 决定 |
| `payloadEncoding` | `json`、`cbor` | 使用部署 `protocol.defaultEncoding` |

创建时可省略字段；upsert 省略或传空字符串保留已有值；PATCH 省略或传 `null` 保持原值，传空字符串恢复默认值。网关的每个子设备独立配置。

切换协议或编码时，暂停主动下发和设备业务消息，排空在途请求及会话旧消息，再更新配置、验证 Shadow get 和 Method 后恢复业务。配置在各节点缓存 2 秒，本节点写入立即失效；在途操作沿用原配置快照。集群应使用一致的部署配置及同一权威数据库，维护期间串行更新，并等待其他节点缓存过期。必要时使用 clean session，切换协议时清理旧 retained Presence。

## Topic 和权限

```text
tio/{thingId}/up/{type}     设备 -> Tio
tio/{thingId}/down/{type}   Tio -> 设备
tio/{thingId}/event          设备 -> 消费者
tio/{thingId}/data           设备 -> 消费者
```

| 方向 | MQTT Topic | QoS | NATS Subject |
|---|---|---:|---|
| 设备 -> Tio | `tio/{thingId}/up/{type}` | 1 | `tio.{thingId}.up.{type}` |
| Tio -> 设备 | `tio/{thingId}/down/{type}` | 1 | `tio.{thingId}.down.{type}` |
| 设备 -> 消费者 | `tio/{thingId}/event` | 1 | `tio.{thingId}.event` |
| 设备 -> 消费者 | `tio/{thingId}/data` | 0 | `tio.{thingId}.data` |

普通设备的权限范围：

```text
publish:
  tio/{thingId}/up/+
  tio/{thingId}/event
  tio/{thingId}/data

subscribe:
  tio/{thingId}/down/+
```

网关只能按相同方向访问已绑定设备的 Topic。

控制消息使用 QoS 1，不使用 Retain；`{type}` 使用小写下划线命名，每种类型对应固定的 Payload 格式。转发、存储和诊断时应同时保留 Topic 或 type 与 Payload。

## Presence

三种模式都会为所有 Thing 发布在线状态，供后端服务对接：

| MQTT Topic | retain | 说明 |
|---|---|---|
| `$iothub/things/{thingId}/presence` | 是 | 当前在线状态快照 |
| `$iothub/events/things/{thingId}/presence` | 否 | 上下线事件流 |

Payload 编码统一使用部署的 `protocol.defaultEncoding`，不随 Thing 配置变化。Simple 模式的普通设备没有这些 Topic 的权限；Legacy/Dual 模式的设备沿用原有 `$iothub` 权限，可以订阅自己的 Presence 和事件流。使用超级用户凭证的后端服务在所有模式下均可订阅。

超级用户可通过 MQTT 或 NATS 发布、订阅 `TIO_APP` 账号内的全部 Topic/Subject，不受部署 `mode` 限制；MQTT 支持 `#` 订阅，NATS 支持 `>` 订阅。

### Type 列表

| Type | 方向 | 说明 |
|---|---|---|
| `shadow_get` | 上行 | 获取完整 Shadow |
| `shadow_get_reply` | 下行 | Shadow 获取结果 |
| `shadow_update` | 上行 | 更新 reported |
| `shadow_update_reply` | 下行 | reported 更新结果 |
| `shadow_delta` | 下行 | 主动下发 delta |
| `method_req` | 下行 | 调用设备方法 |
| `method_resp` | 上行 | 方法执行结果 |
| `ntp_req` | 上行 | 时间同步请求 |
| `ntp_resp` | 下行 | 时间同步响应 |

## 回复格式

以下回复消息必须包含 `code`：

- `shadow_get_reply`、`shadow_update_reply`、`method_resp`、`ntp_resp`

统一规则：

- `code == 200` 表示成功；
- `code != 200` 表示失败，并包含 `message`；
- 成功时不发送 `"message": "OK"`。

## Shadow 消息

desired 和 reported 共用一个由 Tio 生成的文档级 `version`，使用相同的增量合并规则：未出现的字段保持不变，嵌套对象递归合并，标量和数组整体替换，`null` 删除字段。设备通过 `shadow_update` 更新 reported，`state` 必须为对象。

### 获取 Shadow

Topic：`tio/{thingId}/up/shadow_get`

Payload：`{}`

成功响应（`tio/{thingId}/down/shadow_get_reply`）：

```json
{
  "code": 200,
  "version": 13,
  "state": {
    "desired": {},
    "reported": {}
  }
}
```

失败响应：

```json
{"code": 500, "message": "Failed to read shadow"}
```

### 更新 Reported

Topic：`tio/{thingId}/up/shadow_update`

```json
{
  "version": 13,
  "state": {
    "light": true
  }
}
```

`version` 可选：未提供时无条件合并；已提供时作为乐观锁。

成功响应（`tio/{thingId}/down/shadow_update_reply`）：

```json
{"code": 200, "version": 14}
```

版本冲突：

```json
{"code": 409, "version": 14, "message": "Shadow version conflict"}
```

desired 和 reported 实际变化均递增 version，no-op 不递增。设备同一时间最多只能存在一个未完成的 Shadow 请求。

Shadow 请求通过对应的回复 Topic 关联。收到 `409` 后，设备应执行 `shadow_get` 获取最新状态，再决定是否重试；条件更新成功但回复丢失时，QoS 1 重复请求也可能返回 `409`。`shadow_get` 不改变 version，主动通知 `shadow_delta` 不受请求串行限制。

### 下发 Delta

Topic：`tio/{thingId}/down/shadow_delta`

```json
{
  "version": 15,
  "state": {
    "light": true
  }
}
```

`state` 是 desired 与 reported 的 delta。默认只包含变化的叶子字段；配置 `shadow.fullFieldDelta: true` 时，第一级字段下任意变化都会携带该字段的完整值。

本次更新前后 delta 内容相同则不发送；reported 更新使 delta 变化时仍会发送；delta 变空（desired 与 reported 一致）时不发送。设备上线、重连或需要全量状态时通过 `shadow_get` 获取完整 desired 和 reported。

## Method

### 方法请求

Topic：`tio/{thingId}/down/method_req`

```json
{
  "id": "1001",
  "method": "unlock",
  "data": {"door": 1}
}
```

### 方法响应

Topic：`tio/{thingId}/up/method_resp`

成功：

```json
{"id": "1001", "code": 200, "data": {"result": "unlocked"}}
```

失败：

```json
{"id": "1001", "code": 409, "message": "Door is already unlocked"}
```

`id` 由 Tio 生成，设备视为不透明值。Tio 根据 `id` 关联请求和响应。

响应沿用请求 `id`，业务 `data` 可省略。MQTT QoS 1 和调用重试可能造成重复请求；关键 Method 应在设备侧持久化最近的调用 ID 和响应，重复 ID 直接返回原响应。业务可在 `data` 中约定 `expireAt`，由设备校验有效期。

## NTP 时间同步

### 请求

Topic：`tio/{thingId}/up/ntp_req`

```json
{"clientSendTime": 1685428658000}
```

### 响应

Topic：`tio/{thingId}/down/ntp_resp`

```json
{
  "code": 200,
  "clientSendTime": 1685428658000,
  "serverRecvTime": 1685428881508,
  "serverSendTime": 1685428881508
}
```

`clientSendTime` 同时用于关联请求，NTP 不额外使用 `id`。

三个时间字段均为 Unix 毫秒时间戳，分别表示设备发送、服务端接收和服务端发送时间。

## 编码范围

设备控制消息始终优先使用 `Thing.payloadEncoding`，为空时使用 `protocol.defaultEncoding`（默认 JSON），与 `mode` 无关：

- Shadow、Method 和 NTP 使用所选编码；
- Event 和 Data 由 Tio 原字节透传，不解码、不重新编码；
- CBOR 解码后的对象使用字符串键。

HTTP API、数据库 JSON、OpenAPI、connector 内部 NATS/KV 记录以及 Rule source/sink Payload 不会因该配置自动转换。

## Event 和 Data 约定

Event 推荐使用 `{"id":"evt-001","name":"door_closed","ts":1685428658000,"data":{}}`：`id` 由设备生成，同一设备内唯一，同一事件重试沿用原 ID；`name` 为稳定事件名，`ts` 为 Unix 毫秒时间戳，`data` 为可选业务数据。消费者按 `thingId + id` 去重。Tio 原字节透传，业务双方也可约定其他格式。

Data 适用于高频、允许少量丢失的数据，使用 QoS 0，格式由业务双方约定。

## 协议扩展与兼容

通用控制能力通过新增 `{type}` 扩展，业务事件和高频数据使用 Event/Data。已发布的类型名称和方向保持稳定；Payload 可增加向后兼容的可选字段，不兼容变化使用新类型或协议版本。

## 业务服务订阅 Data/Event

data 和 event 消息不被 Tio 处理，直接透传到 NATS subject。业务服务可通过 JetStream durable consumer 可靠消费，支持离线消息保留和多消费者独立消费。

### 连接

使用 super-user 凭证通过 **NATS 协议**连接（非 MQTT），默认端口为 `4222`，可在主配置引用的 NATS YAML 中设置 `server.port`：

```go
nc, _ := nats.Connect("nats://host:port", nats.UserInfo("$biz", "password"))
js, _ := nc.JetStream()
```

### 创建 Stream

业务服务自行创建 JetStream stream，按需选择 subjects、retention 和 MaxAge：

```go
js.AddStream(&nats.StreamConfig{
    Name:      "BIZ_DATA",
    Subjects:  []string{"tio.*.data", "tio.*.event"},
    Retention: nats.LimitsPolicy,
    MaxAge:    7 * 24 * time.Hour,
    Storage:   nats.FileStorage,
    Replicas:  3, // 集群节点数
})
```

Stream 必须在设备开始发布消息**之前**创建，否则之前的消息不会被捕获。`AddStream` 是幂等的，多节点同时创建不会冲突。

### Durable Consumer

Pull 模式（主动拉取）：

```go
sub, _ := js.PullSubscribe("tio.*.data", "biz-data", nats.ManualAck())
msgs, _ := sub.Fetch(10, 5*time.Second)
for _, msg := range msgs {
    fmt.Printf("%s: %s\n", msg.Subject, msg.Data)
    msg.Ack()
}
```

Push 模式（自动推送）：

```go
js.Subscribe("tio.*.event", func(msg *nats.Msg) {
    fmt.Printf("%s: %s\n", msg.Subject, msg.Data)
    msg.Ack()
}, nats.Durable("biz-event"), nats.ManualAck())
```

相同 durable name 的多实例**负载均衡**（每条消息只投递给一个实例）；不同 durable name 的多服务**独立消费**（各收全量消息）。

业务服务离线时消息在 stream 中保留（受 `MaxAge` 限制），重连后从上次 ack 位置继续消费。
