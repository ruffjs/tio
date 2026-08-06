# Tio Simple 设备协议设计

## 1. 目标

Simple 协议用于降低设备接入复杂度，同时保留以下核心能力：

- Shadow 的 `desired`、`reported` 和文档级 version；
- Shadow 增量更新与乐观锁；
- 设备直接方法；
- NTP 时间同步；
- Event 和 Data 原字节透传；
- JSON、CBOR 全局编码配置。

Simple 与 Legacy 不并行运行：

```yaml
protocol:
  mode: simple       # legacy 或 simple，默认 legacy
  encoding: json     # json 或 cbor，默认 json
```

空值或未知值导致 Tio 拒绝启动。

`mode: simple` 注册 Simple MQTT Topic；`mode: legacy` 注册 Legacy 设备 Topic。两种模式共享 Thing、Shadow 和 Method HTTP API。

---

## 2. 设计原则

1. Tio 管理设备连接和通用设备能力，不承载具体业务语义。
2. 控制消息通过 `/up/{type}` 和 `/down/{type}` 传输。
3. `{type}` 决定唯一、固定的 Payload 格式。
4. 回复消息使用 `code` 区分成功和失败：
   - `code == 200`：成功；
   - `code != 200`：失败，并包含 `message`。
5. Event 和 Data 独立传输，不进入控制消息通道。

---

## 3. Topic

### 3.1 Topic 模型

```text
tio/{thingId}/up/{type}
tio/{thingId}/down/{type}
tio/{thingId}/event
tio/{thingId}/data
```

| 方向 | MQTT Topic | QoS | NATS Subject |
|---|---|---:|---|
| 设备 → Tio | `tio/{thingId}/up/{type}` | 1 | `tio.{thingId}.up.{type}` |
| Tio → 设备 | `tio/{thingId}/down/{type}` | 1 | `tio.{thingId}.down.{type}` |
| 设备 → 消费者 | `tio/{thingId}/event` | 1 | `tio.{thingId}.event` |
| 设备 → 消费者 | `tio/{thingId}/data` | 0 | `tio.{thingId}.data` |

普通设备的权限范围：

```text
publish:
  tio/{thingId}/#  （除 down 外）

subscribe:
  tio/{thingId}/#  （除 up 外）
```

ACL 采用 deny-list 策略：设备不可发布到 `down/{type}`，不可订阅 `up/{type}`；`tio/{thingId}/#` 下其余所有 Topic 均可自由发布和订阅，包括 `event`、`data` 以及未定义的自定义 Topic。自定义 Topic 不经由 Tio 解析，设备可将 NATS 作为自由消息中间件使用。

网关只能按相同方向访问已绑定设备的 Topic。

### 3.2 Type 命名

`{type}` 使用小写下划线命名：

```text
{domain}_{action}
```

当前类型：

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

### 3.3 Topic 规则

- 每个 `{type}` 对应唯一、固定的 Payload 格式；
- 控制消息统一使用 QoS 1，不使用 Retain；
- 新增控制能力时增加新的 `{type}`；
- 只有通用、稳定、低频的设备控制能力可以增加 `{type}`；
- 业务事件继续使用 `event`，高频数据继续使用 `data`；
- 收到未知 `{type}` 时不执行，记录限频日志和指标；
- Topic 名称发布后视为协议的一部分，不应随意修改。

由于 Payload 不再自描述，Tio 在日志、NATS 转发、持久化、死信和诊断信息中必须同时保留完整 Topic 或解析后的 `type`，不得只保留 Payload。

---

## 4. 编码

消息编码由 Tio 全局配置：

```yaml
protocol:
  encoding: json
```

或：

```yaml
protocol:
  encoding: cbor
```

规则：

- 同一个 Tio 实例使用一种编码；
- Shadow、Method 和 NTP 使用所选编码；
- JSON 和 CBOR 使用相同消息模型；
- CBOR 对象只允许字符串键；
- Event 和 Data 原字节透传，Tio 不解码、不重新编码。

以下内容不受该配置影响：

- HTTP API；
- 数据库 JSON；
- OpenAPI；
- Connector 内部 NATS/KV 数据；
- Rule source/sink Payload。

---

## 5. 回复格式

以下回复消息必须包含 `code`：

- `shadow_get_reply`
- `shadow_update_reply`
- `method_resp`
- `ntp_resp`

统一规则：

- `code == 200` 表示成功；
- `code != 200` 表示失败；
- 失败时必须包含 `message`；
- 成功时不发送 `"message": "OK"`；
- 成功结果保持扁平，不强制放入统一的 `data`；
- Method 自身的业务返回值仍放在 `data` 中。

每个 Topic 分别定义自己的成功和失败格式，不根据字段是否存在推测消息类型。

---

## 6. Shadow 模型

Shadow 保存一份文档：

```json
{
  "version": 13,
  "state": {
    "desired": {},
    "reported": {}
  }
}
```

### 6.1 Version

`version` 表示整份 Shadow 文档版本：

- 由 Tio 生成；
- desired 或 reported 实际变化时递增一次；
- no-op 更新不递增；
- desired 和 reported 共用一个 version；
- 请求携带的 version 表示期望的当前版本；
- version 不匹配时拒绝更新；
- Shadow `get` 不改变 version。

### 6.2 状态合并

desired 和 reported 使用相同的增量合并规则：

- 未出现的字段保持不变；
- 嵌套对象递归合并；
- 标量和数组整体替换；
- 值为 `null` 时删除字段。

旧状态：

```json
{
  "network": {
    "ssid": "old",
    "retry": 3
  },
  "labels": ["a"],
  "obsolete": true
}
```

Patch：

```json
{
  "network": {
    "ssid": "new"
  },
  "labels": ["b"],
  "obsolete": null
}
```

结果：

```json
{
  "network": {
    "ssid": "new",
    "retry": 3
  },
  "labels": ["b"]
}
```

### 6.3 Delta

Tio 根据 desired 和 reported 动态计算 delta，用于：

- 判断是否需要主动下发 desired；
- Shadow HTTP API；
- 内部事件和规则处理。

Simple MQTT 消息不返回 delta，也不返回字段级 metadata 或 timestamp。

---

## 7. Shadow 请求规则

设备同一时间最多只能存在一个未完成的 Shadow 请求：

```text
发送 shadow_get 或 shadow_update
→ 等待对应回复或超时
→ 再发送下一条 Shadow 请求
```

`shadow_desired` 是主动通知，不受该限制。

---

## 8. 获取 Shadow

### 8.1 请求

Topic：

```text
tio/{thingId}/up/shadow_get
```

Payload：

```json
{}
```

设备上线、重连或发生版本冲突后发送该请求。

### 8.2 成功响应

Topic：

```text
tio/{thingId}/down/shadow_get_reply
```

Payload：

```json
{
  "code": 200,
  "version": 13,
  "state": {
    "desired": {
      "light": true
    },
    "reported": {
      "light": false,
      "temperature": 4.2
    }
  }
}
```

成功时：

- `version` 必须存在；
- `state.desired` 和 `state.reported` 必须存在；
- 空状态使用空对象 `{}`。

### 8.3 失败响应

```json
{
  "code": 500,
  "message": "Failed to read shadow"
}
```

`shadow_get` 不修改 Shadow，也不改变 version。

---

## 9. 更新 Reported

### 9.1 请求

Topic：

```text
tio/{thingId}/up/shadow_update
```

Payload：

```json
{
  "version": 13,
  "state": {
    "light": true,
    "temperature": 4.1
  }
}
```

规则：

- `state` 必须是对象；
- `state` 表示 reported Patch；
- 设备只能更新 reported；
- `version` 可选。

Version 行为：

- 未提供：无条件合并；
- 已提供：仅在当前 Shadow version 相同时合并。

### 9.2 更新成功

Topic：

```text
tio/{thingId}/down/shadow_update_reply
```

Payload：

```json
{
  "code": 200,
  "version": 14
}
```

reported 实际变化时 version 递增。

Patch 为 no-op 时：

- 返回 `200`；
- 返回当前 version；
- version 不递增。

### 9.3 版本冲突

```json
{
  "code": 409,
  "version": 14,
  "message": "Shadow version conflict"
}
```

设备收到 `409` 后执行 `shadow_get`，根据最新 Shadow 决定是否重试。

### 9.4 非法请求

```json
{
  "code": 400,
  "message": "Invalid shadow update"
}
```

### 9.5 QoS 重复消息

Shadow 不使用请求 ID，因此不保存请求级去重结果。

条件更新成功但响应丢失时，重复请求可能返回 `409`，设备通过重新获取 Shadow 恢复。

无条件重复 Patch 在状态已一致时为 no-op。

---

## 10. 下发 Desired

Topic：

```text
tio/{thingId}/down/shadow_desired
```

云端通过 Shadow HTTP API 更新 desired。desired 实际变化后：

1. 合并 desired；
2. Shadow version 递增；
3. 重新计算 delta；
4. delta 非空时向在线设备发送完整 desired。

Payload：

```json
{
  "version": 15,
  "state": {
    "light": true,
    "target_temperature": 4
  }
}
```

规则：

- `state` 始终是完整 desired；
- 重复写入相同 desired 不发送；
- desired 与 reported 一致时不发送；
- reported 更新不主动触发新的 `shadow_desired`；
- 设备上线或重连后通过 `shadow_get` 获取当前完整状态。

reported 更新不主动触发 desired，可避免：

```text
desired → update reported → desired → update reported
```

形成循环下发。

---

## 11. Method

### 11.1 方法请求

Topic：

```text
tio/{thingId}/down/method_req
```

Payload：

```json
{
  "id": "1001",
  "method": "unlock",
  "data": {
    "door": 1,
    "sessionId": "s1001",
    "expireAt": 1785542410000,
    "uploadUrl": "https://example.com/upload/video",
    "uploadUrlExpiredAt": 1785543410000
  }
}
```

字段：

| 字段 | 必填 | 说明 |
|---|---:|---|
| `id` | 是 | 调用唯一 ID |
| `method` | 是 | 方法名 |
| `data` | 否 | 不透明业务参数 |

`id` 由 Tio 生成：

- 同一个 Thing 的方法 ID 不重复；
- ID 是不透明值，不具有排序语义；
- 设备不得根据 ID 大小判断消息新旧。

### 11.2 方法响应

Topic：

```text
tio/{thingId}/up/method_resp
```

成功且无业务返回数据：

```json
{
  "id": "1001",
  "code": 200
}
```

成功且有业务返回数据：

```json
{
  "id": "1001",
  "code": 200,
  "data": {
    "result": "unlocked"
  }
}
```

失败：

```json
{
  "id": "1001",
  "code": 409,
  "message": "Door is already unlocked"
}
```

规则：

- `id` 与请求相同；
- `code` 必须存在；
- 失败时必须包含 `message`；
- `data` 仅在存在业务返回值时提供；
- Tio 不解析或裁剪 `data`。

建议响应码：

| Code | 含义 |
|---:|---|
| `200` | 执行成功 |
| `400` | 参数错误 |
| `404` | 方法不存在 |
| `408` | 设备执行超时 |
| `409` | 当前状态不允许执行或请求已过期 |
| `429` | 设备繁忙 |
| `500` | 设备内部错误 |
| `503` | 暂时无法执行 |

Tio 根据 `id` 关联请求和响应。

HTTP 取消、等待超时或 MQTT 发布失败时，Tio 必须清理调用关联。

HTTP 方法调用响应始终返回 HTTP 200，表示成功从设备收到回复。响应 body 结构为 `{"code":200,"message":"OK","data":{"code":<设备code>,"message":"<设备message>","data":<设备data>}}`，设备执行结果嵌套在 `data` 中，与 HTTP 状态码分开。设备离线或超时等 Tio 侧错误同样返回 HTTP 200，顶层 `code` 为对应错误码（如 504）。

---

## 12. Method 重复执行保护

MQTT QoS 1 和调用重试可能造成重复请求。

对于开锁等关键 Method，设备应按 Method 持久化最近 10 个调用记录：

```text
method → recent [id, response]
```

收到请求时：

```text
id 已存在
→ 不重复执行
→ 返回原响应

id 不存在
→ 执行
→ 保存 id 和响应
→ 返回响应
```

建议：

- 使用固定长度环形记录；
- 至少保存 `id` 和 `code`；
- 响应包含重要 `data` 时，应保存完整响应或可重建结果；
- 关键记录在设备重启后仍应保留。

最近 10 条主要用于覆盖：

- MQTT QoS 1 重复投递；
- 短时间调用重试；
- 网络中断后的再次投递。

高风险 Method 可在业务 `data` 中携带：

```json
{
  "expireAt": 1785542410000
}
```

设备在执行前校验有效期。Tio 不解析该业务字段。

---

## 13. NTP 时间同步

NTP 保留当前实现的三个时间字段，仅通过 Topic 标识请求和响应类型。

### 13.1 请求

Topic：

```text
tio/{thingId}/up/ntp_req
```

Payload：

```json
{
  "clientSendTime": 1685428658000
}
```

字段：

| 字段 | 说明 |
|---|---|
| `clientSendTime` | 设备发送请求时的本地 Unix 毫秒时间戳 |

### 13.2 成功响应

Topic：

```text
tio/{thingId}/down/ntp_resp
```

Payload：

```json
{
  "code": 200,
  "clientSendTime": 1685428658000,
  "serverRecvTime": 1685428881508,
  "serverSendTime": 1685428881508
}
```

字段：

| 字段 | 说明 |
|---|---|
| `clientSendTime` | 原样返回设备发送时间 |
| `serverRecvTime` | Tio 收到请求时的 Unix 毫秒时间戳 |
| `serverSendTime` | Tio 发送响应时的 Unix 毫秒时间戳 |

`clientSendTime` 同时用于关联本次请求，NTP 不额外使用 `id`。

### 13.3 失败响应

```json
{
  "code": 400,
  "message": "Invalid clientSendTime"
}
```

---

## 14. Event

Topic：

```text
tio/{thingId}/event
```

推荐格式：

```json
{
  "id": "evt-01K1C8J0Z93XK6J23P8A6Y4QSC",
  "name": "door_closed",
  "ts": 1785542400123,
  "data": {
    "sessionId": "s1001"
  }
}
```

字段：

| 字段 | 必填 | 说明 |
|---|---:|---|
| `id` | 是 | 事件唯一 ID，用于业务消费者去重 |
| `name` | 是 | 稳定的事件名称 |
| `ts` | 是 | 事件发生时间，Unix 毫秒时间戳 |
| `data` | 否 | 不透明业务数据 |

规则：

- `id` 由设备生成，在同一设备内不得重复；
- 同一业务事件重试发送时必须使用相同 `id`；
- 不同业务事件不得复用相同 `id`；
- MQTT QoS 1 可能重复投递，消费者按 `thingId + id` 去重；
- `name` 使用稳定名称，例如 `door_opened`、`door_closed`、`media_uploaded`；
- `data` 由设备与消费者约定；
- Tio 原字节透传，不校验格式、不修改内容；
- Event 不进入 Shadow、Method 或 NTP。

Event 格式是推荐约定。未采用该格式的 Payload 仍可正常透传，但消费者需自行解决事件识别和去重。

---

## 15. Data

Topic：

```text
tio/{thingId}/data
```

规则：

- QoS 0；
- 原字节透传；
- Tio 不解析、不修改；
- 不进入 Shadow、Method 或 NTP；
- 适用于高频、允许少量丢失的数据。

---

## 16. 协议扩展与兼容

- 新增通用控制能力时增加新的 `{type}`；
- 已发布的 `{type}` 不修改名称和方向；
- 已有 Payload 可以增加向后兼容的可选字段；
- 不兼容的结构变化应增加新的 `{type}` 或协议版本；
- 不应为具体业务事件不断增加控制 Topic；
- 需要不同 QoS、Retain、权限、优先级或大 Payload 的能力，应使用独立 Topic 或独立传输通道。

---

## 17. 实现要求

1. Simple 和 Legacy 的设备 Topic 互斥注册。
2. 控制消息使用 `/up/{type}` 和 `/down/{type}`，ACL 仅禁止设备发布 `down` 和订阅 `up`，`tio/{thingId}/#` 下其余 Topic 自由放行。
3. `{type}` 使用小写下划线命名，每个 Type 对应固定 Payload。
4. Tio 内部转发和存储必须同时保留 Type 与 Payload。
5. Shadow 使用单一文档级 version。
6. desired 和 reported 实际变化均递增 version。
7. desired 和 reported 使用统一增量合并规则。
8. 设备 reported 更新支持可选乐观锁。
9. `shadow_get_reply` 返回完整 desired、reported 和 version。
10. Shadow 请求不使用 ID，设备串行发送请求。
11. Shadow、Method 和 NTP 回复通过 `code` 区分成功与失败格式。
12. reported 更新不主动下发 desired。
13. Method ID 为不透明唯一值，不具有排序语义。
14. 关键 Method 通过设备侧最近调用记录防止重复执行。
15. NTP 保留 `clientSendTime`、`serverRecvTime`、`serverSendTime`。
16. Event 推荐携带唯一 ID，消费者按 `thingId + id` 去重。
17. Event 和 Data 保持原字节透传。
