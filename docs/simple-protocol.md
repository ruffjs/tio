# Tio Simple 设备协议

Simple 协议是 legacy 设备协议的启动期替代模式。两种模式不会并行运行：

```yaml
protocol:
  mode: simple       # legacy 或 simple，默认 legacy
  encoding: json     # json 或 cbor，默认 json
```

空值和未知值都会导致 Tio 拒绝启动。`mode: simple` 只注册 Simple MQTT Topic 和
`POST /api/v1/things/{id}/invoke`；`mode: legacy` 只注册 Legacy Shadow、Method、NTP
设备路径和 Legacy Method HTTP 路由。两种模式共享 Thing 和 Shadow HTTP API。

## Topic 和权限

| 方向 | MQTT Topic | QoS | 用途 | NATS Subject |
| --- | --- | --- | --- | --- |
| 设备 → Tio | `tio/{thingId}/up` | 1 | `report`、`get`、`reply` | `tio.{thingId}.up` |
| Tio → 设备 | `tio/{thingId}/down` | 1 | `set`、`call` | `tio.{thingId}.down` |
| 设备 → 消费者 | `tio/{thingId}/event` | 1 | 业务事件原字节透传 | `tio.{thingId}.event` |
| 设备 → 消费者 | `tio/{thingId}/data` | 0 | 遥测数据原字节透传 | `tio.{thingId}.data` |

普通设备只能发布自己的 `up/event/data` 并订阅自己的 `down`。网关只能以同样方向访问
已绑定设备的 Topic。其他 Thing、方向、额外层级和通配符均被拒绝。

## Control 消息

Control 消息使用统一信封：

```json
{"t":"report","id":"optional-id","d":{}}
```

- `t`：`report`、`get`、`set`、`call`、`reply` 之一。
- `id`：Method 调用关联 ID；`get` 携带 ID 时，返回的 `set` 会原样带回。
- `d`：状态或不透明的 Method 业务内容。

### Report 和增量合并

设备通过 `report` 增量更新 reported：

```json
{
  "t": "report",
  "d": {
    "version": 12,
    "state": {
      "environment": {"temperature": 4.2},
      "obsolete": null
    }
  }
}
```

`version` 是设备已经应用的 desired 版本，必须是非负整数；`state` 必须是对象。
desired 和 reported 使用同一套合并规则：

- 未出现的字段保持不变；
- 嵌套对象递归合并；
- 标量和数组整体替换；
- 值为 `null` 时删除该字段。

例如旧状态为：

```json
{"network":{"ssid":"old","retry":3},"labels":["a"],"obsolete":true}
```

应用 Patch：

```json
{"network":{"ssid":"new"},"labels":["b"],"obsolete":null}
```

结果为：

```json
{"network":{"ssid":"new","retry":3},"labels":["b"]}
```

desired 合并后实际变化时 version 增加一次；no-op desired 和所有 report 都不增加
desired version。

### Set 和 Get

主动 `set` 仅在 desired 实际变化且 desired/reported delta 非空时发送，并始终携带完整
desired，而不是 Patch。重复 desired 不发送；新的 desired 恰好等于 reported 时也不发送，
但实际变化仍会持久化并增加 version。

设备 `report` 不触发 `set`。设备上线或重连后应发送 `{"t":"get"}`；Tio 无论是否存在
delta，都会返回当前完整状态：

```json
{"t":"set","d":{"version":13,"state":{"light":true}}}
```

## Method

调用方使用 `POST /api/v1/things/{id}/invoke`：

```json
{"method":"open","params":{"door":1},"timeout":30}
```

Tio 下发：

```json
{"t":"call","id":"generated-id","d":{"m":"open","door":1}}
```

设备用相同 ID 回复：

```json
{"t":"reply","id":"generated-id","d":{"ok":true,"nested":{"code":"done"}}}
```

`params` 和完整 reply `d` 都是不透明业务值，Tio 不解析或裁剪。timeout 单位为秒，默认
30，最大 300；超时、HTTP 取消和发布失败都会清理调用关联。

## 编码范围

`protocol.encoding` 是全局设备 MQTT 应用消息编码：

- Simple Control 的 `report/get/set/call/reply` 使用所选编码；
- legacy 模式下 Shadow、Method、NTP 和设备可见 Presence 使用所选编码；
- Event/Data 由 Tio 原字节透传，不解码、不重新编码；设备与消费者仍须共同遵循全局编码；
- CBOR 解码后的对象使用字符串键。

HTTP API、数据库 JSON、OpenAPI、connector 内部 NATS/KV 记录以及任意 Rule source/sink
Payload 不会因该配置自动转换。Event/Data 不进入 Shadow 或 Method，也不会修改 Shadow。
