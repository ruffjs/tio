# Tio Simple 设备协议

Simple 协议是 legacy 设备协议的启动期替代模式。两种模式不会并行运行：

```yaml
protocol:
  mode: simple       # legacy 或 simple，默认 legacy
  encoding: json     # json 或 cbor，默认 json
```

空值和未知值都会导致 Tio 拒绝启动。`mode: simple` 只注册 Simple MQTT Topic；
`mode: legacy` 只注册 Legacy 设备 Topic。两种模式共享 Thing、Shadow 和 Method HTTP API。

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

### Type 列表

| Type | 方向 | 说明 |
|---|---|---|
| `shadow_get` | 上行 | 获取完整 Shadow |
| `shadow_get_reply` | 下行 | Shadow 获取结果 |
| `shadow_update` | 上行 | 更新 reported |
| `shadow_update_reply` | 下行 | reported 更新结果 |
| `shadow_desired` | 下行 | 主动下发完整 desired |
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

### 下发 Desired

Topic：`tio/{thingId}/down/shadow_desired`

```json
{
  "version": 15,
  "state": {
    "light": true
  }
}
```

`state` 始终是完整 desired。desired 与 reported 一致时不发送；reported 更新不主动触发 `shadow_desired`；设备上线或重连后通过 `shadow_get` 获取当前完整状态。

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

## 编码范围

`protocol.encoding` 是全局设备 MQTT 应用消息编码：

- Shadow、Method 和 NTP 使用所选编码；
- Event 和 Data 由 Tio 原字节透传，不解码、不重新编码；
- CBOR 解码后的对象使用字符串键。

HTTP API、数据库 JSON、OpenAPI、connector 内部 NATS/KV 记录以及 Rule source/sink Payload 不会因该配置自动转换。
