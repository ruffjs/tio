# Tio 设备通信模型与协议设计

## 1. 目标

调整 Tio 的设备通信模型，提供统一、轻量的设备接入能力：

* 使用 Shadow 管理持续状态；
* 使用 Method 执行一次性操作；
* 透传关键事件和遥测数据；
* 支持 JSON 或 CBOR 编码；
* 不引入具体业务语义。

核心模型：

```text
Thing
Shadow
Method
```

---

## 2. 系统边界

Tio 负责：

* 设备连接与认证；
* Thing 管理；
* Shadow 状态存储与同步；
* Method 调用与响应；
* MQTT 消息投递；
* Event 和 Data 透传。

Tio 不负责：

* 解析业务事件；
* 解析 Method 业务参数；
* 维护业务流程或状态机；
* 生成文件上传地址；
* 传输图片、视频等大文件。

Method 参数以及 Event、Data 内容均视为不透明 Payload。

---

## 3. Topic

| 通道         | Topic                 | QoS | Tio 行为                    |
| ---------- | --------------------- | --- | ------------------------- |
| Control 上行 | `tio/{thingId}/up`    | 1   | 处理 `report`、`get`、`reply` |
| Control 下行 | `tio/{thingId}/down`  | 1   | 下发 `set`、`call`           |
| Event      | `tio/{thingId}/event` | 1   | 原样透传给业务消费者                |
| Data       | `tio/{thingId}/data`  | 0   | 原样透传给数据消费者                |

设备发布：

```text
tio/{thingId}/up
tio/{thingId}/event
tio/{thingId}/data
```

设备订阅：

```text
tio/{thingId}/down
```

设备身份由 Topic 中的 `thingId` 确定，Payload 中不重复携带。

---

## 4. 编码

消息编码由 Tio 全局配置：

```yaml
protocol:
  encoding: json
  mode: legacy
```

或：

```yaml
protocol:
  encoding: cbor
  mode: legacy
```

要求：

* 一个 Tio 实例使用一种编码；
* `protocol.encoding` 在进程启动时确定，运行期间不可切换；
* `protocol.mode` 只能是 `legacy` 或 `simple`，分别选择现有兼容协议或本设计定义的简化协议；
* 为保持向后兼容，默认值为 `legacy`；
* 协议模式在进程启动时确定，运行期间不可切换；
* 所有设备侧 MQTT 应用消息使用相同编码，包括 Control、Event 和 Data；
* 凡是 Tio 解析或生成的设备 MQTT Payload，均必须通过同一个全局 Codec；
* 如果实现中保留兼容协议，该兼容协议的设备 MQTT Payload 也必须受同一个编码配置控制，不得固定为 JSON；
* JSON 和 CBOR 使用相同消息模型；
* 不进行逐设备或逐消息协商。

Event 和 Data 仍是不透明 Payload。Tio 不为透传而解码、修改或重新编码它们，
但设备与对应消费者必须按照 `protocol.encoding` 编解码；Tio 投递给消费者的字节必须与设备发布的字节完全一致。

连接认证、MQTT 协议报文和 Broker 内部消息不属于这里的应用消息编码范围。

### 4.1 协议模式边界

协议模式必须完整选择一套协议，不能混合两种协议的部分消息路径：

* `mode: legacy` 时，只注册兼容协议的设备 Topic、下行消息和专属调用入口，不注册简化协议路径；
* `mode: simple` 时，只注册 `tio/{thingId}/up|event|data` 的处理或转发、`tio/{thingId}/down` 下行和简化协议调用入口，不注册兼容协议路径；
* Tio 每次启动只注册被选中协议的完整消息路径，未选中协议不得留下订阅、下行或专属调用入口；
* 两种协议使用同一套 Thing、连接状态和 Shadow 持久化模型，但不会在同一进程实例中并行运行；
* `protocol.encoding` 作用于当前模式，不为不同模式分别配置编码；
* 未知或空的 `protocol.mode` 必须导致 Tio 拒绝启动。

---

## 5. Control Plane

控制面支持五种消息：

```text
report
get
set
call
reply
```

Shadow 消息结构：

```json
{
  "t": "report",
  "d": {}
}
```

Method 消息结构：

```json
{
  "t": "call",
  "id": "cmd_1001",
  "d": {}
}
```

字段定义：

| 字段   | 说明                               |
| ---- | -------------------------------- |
| `t`  | 消息类型                             |
| `id` | Method 调用 ID，仅 `call`、`reply` 使用 |
| `d`  | 状态、方法参数或响应内容                     |

Tio 只解析控制消息的通用字段，不解析 `d` 中的业务内容。

---

## 6. Shadow

Shadow 包含：

```text
desired   期望状态
reported  实际状态
```

Shadow 用于持续、可覆盖的状态。一次性操作使用 Method。

### 6.1 report

设备可以增量上报 reported state：

```json
{
  "t": "report",
  "d": {
    "version": 12,
    "state": {
      "light": true,
      "temperature": 4.2
    }
  }
}
```

Tio 将 `d.state` 按 6.4 节的规则合并到：

```text
Shadow.reported
```

`version` 表示设备已经应用的 desired 版本。

设备不需要在每次 `report` 中发送完整 reported state。未出现在 `d.state` 中的字段保持不变；
需要删除的字段必须显式上报为 `null`。

---

### 6.2 set

desired state 更新后，Tio 下发完整状态：

```json
{
  "t": "set",
  "d": {
    "version": 13,
    "state": {
      "light": true,
      "target_temperature": 4
    }
  }
}
```

`set` 始终携带完整 desired state，不使用增量 Patch。

调用方更新 desired 时可以提交增量状态；Tio 先按 6.4 节的规则合并并持久化。

主动下发 `set` 必须同时满足：

1. 合并后的 desired 与更新前的 desired 不同；
2. 合并后的 desired 与当前 reported 存在 delta。

满足条件时，Tio 下发合并后的完整 desired state，而不是只下发 delta。
相同 desired 的重复写入不下发；新的 desired 恰好等于 reported 时也不下发。
desired 实际变化时 version 仍正常递增，即使本次没有主动下发 `set`。

设备发送 `report` 不触发 `set`；设备上线或重连后的同步只由 `get` 触发，并且 `get` 始终返回完整 `set`。

---

### 6.3 get

设备上线或重连后发送：

```json
{
  "t": "get"
}
```

Tio 始终返回当前完整 `set`：

```json
{
  "t": "set",
  "d": {
    "version": 13,
    "state": {
      "light": true,
      "target_temperature": 4
    }
  }
}
```

设备版本是否与服务端一致，不影响返回内容。

---

### 6.4 状态合并规则

desired 更新和设备 `report` 使用同一套确定性的递归合并规则：

1. 更新内容的根节点必须是对象；根节点为 `null`、数组或标量时拒绝消息。
2. 更新中未出现的字段保持原值不变。
3. 当新值是对象时逐字段递归合并；如果旧值不存在或不是对象，先将旧值视为空对象再合并。
4. 当新值是标量或数组时，完整替换旧值；数组不按元素合并。
5. 当字段的新值为 `null` 时，删除该字段；`null` 不作为 Shadow 中可持久化的业务值。
6. 删除一个不存在的字段属于无变化。
7. 空对象 `{}` 不删除已有对象，也不改变其已有字段；当旧值不存在或不是对象时，结果为新的空对象。
8. 是否发生变化，以完成合并后的完整状态与合并前状态进行深度比较为准。

示例，已有状态：

```json
{
  "light": true,
  "network": {
    "ssid": "office",
    "rssi": -60
  },
  "alarms": ["door", "temp"]
}
```

增量更新：

```json
{
  "network": {
    "rssi": -55,
    "ssid": null
  },
  "alarms": ["door"]
}
```

合并结果：

```json
{
  "light": true,
  "network": {
    "rssi": -55
  },
  "alarms": ["door"]
}
```

该规则必须由一个共享的纯函数实现，desired 和 reported 不得各自维护一套合并逻辑。

### 6.5 版本规则

* version 表示整个 desired state 的版本；
* desired state 按 6.4 节合并后实际发生变化时 version 递增；
* 合并结果与原状态相同时 version 不变；
* `get` 不改变 version；
* `report` 不自动修改 desired，也不递增 desired version；
* Tio 保存当前 desired state；
* Tio 不维护每一条 `set` 的未送达队列；
* 设备通过 `get` 完成上线和重连后的状态同步。

---

## 7. Method

Method 用于一次性设备操作。

### 7.1 call

Tio 向设备下发调用：

```json
{
  "t": "call",
  "id": "cmd_1001",
  "d": {
    "m": "open",
    "expire_at": 1785542410000
  }
}
```

其中：

* `id` 唯一标识一次调用；
* `d.m` 表示方法名；
* 其他参数由调用方定义；
* Tio 不解析方法名和参数。

### 7.2 reply

设备返回调用结果：

```json
{
  "t": "reply",
  "id": "cmd_1001",
  "d": {
    "ok": true,
    "code": "ok"
  }
}
```

失败示例：

```json
{
  "t": "reply",
  "id": "cmd_1001",
  "d": {
    "ok": false,
    "code": "command_expired",
    "message": "command expired"
  }
}
```

Tio 根据 `id` 将 `reply` 与原 `call` 关联，并将结果返回给调用方。

Tio 不根据 `reply` 内容修改 Shadow。

---

## 8. Event Channel

Event Topic：

```text
tio/{thingId}/event
```

Event Payload 由设备和业务平台约定，例如：

```json
{
  "id": "evt_1001",
  "name": "door_closed",
  "ts": 1785542400123,
  "d": {
    "session_id": "ss_1001"
  }
}
```

Tio 对 Event：

* 不解析字段；
* 不识别事件类型；
* 不写入 Shadow；
* 不执行事件去重；
* 将原始字节透传给业务消费者；
* 设备和业务消费者使用全局 `protocol.encoding` 编解码。

Event 的 ID、业务关联和幂等处理由业务平台负责。

---

## 9. Data Channel

Data Topic：

```text
tio/{thingId}/data
```

Data Payload 由设备和数据消费者约定，例如：

```json
{
  "name": "temperature",
  "ts": 1785542400123,
  "d": {
    "value": 4.2,
    "unit": "celsius"
  }
}
```

Tio 对 Data：

* 不解析字段；
* 不写入 Shadow；
* 不存储数据；
* 将原始字节透传给数据消费者；
* 设备和数据消费者使用全局 `protocol.encoding` 编解码。

---

## 10. 消息处理关系

```text
Device
  │
  ├── up/report ──────> 更新 Shadow.reported
  │
  ├── up/get ─────────> 读取 Shadow.desired
  │                         │
  │                         └── down/set ─────> Device
  │
  ├── up/reply ───────> 完成 Method 调用
  │
  ├── event ──────────> 透传给业务消费者
  │
  └── data ───────────> 透传给数据消费者

调用方
  │
  ├── 更新 desired ───> Tio
  │                         │
  │                         └── down/set ─────> Device
  │
  └── 调用 Method ────> Tio
                            │
                            └── down/call ────> Device
```

---

## 11. 实现要求

### Thing

* 根据 `thingId` 识别设备；
* 维护设备连接状态；
* 提供 Shadow 和 Method 的访问入口。

### Shadow

* 持久化 desired 和 reported；
* 为 desired 维护整体 version；
* 使用同一个纯函数完成 desired 和 reported 的递归增量合并；
* 支持嵌套对象合并、标量和数组替换、`null` 删除；
* 处理设备 `report` 和 `get`；
* 将完整 `set` 下发给设备。

### Method

* 生成或接收唯一调用 ID；
* 向设备下发 `call`；
* 根据 ID 关联 `reply`；
* 向调用方返回结果；
* 支持调用超时；
* 不解析方法参数和响应数据。

### Event/Data

* 接收独立 Topic 的设备消息；
* 保留原始 Payload；
* 投递给对应消费者；
* 不进入 Shadow 和 Method。

### 编码

* 启动时根据全局配置创建唯一 Codec，之后保持不变；
* 根据全局 Codec 编解码所有由 Tio 解析或生成的设备 MQTT 应用消息；
* JSON 和 CBOR 解码后必须得到等价的字符串键消息模型；
* Event 和 Data 不解码、不重新编码，透传前后的 Payload 字节必须完全一致；
* Event 和 Data 的设备及消费者仍必须遵循同一个全局编码配置。

### 简单实现约束

实现应优先选择可直接验证的最小结构：

* `up` Topic 只建立一个订阅，由一个 dispatcher 处理 `report`、`get` 和 `reply`；
* 不得为同一上行 Topic 建立会竞争或重复消费消息的多个 dispatcher；
* Codec 在启动时构造并显式传入需要它的组件，不提供运行时全局切换；
* 状态合并只保留一个无副作用的共享函数，并使用表驱动测试覆盖全部规则；
* Method pending call 只保存关联所需的最小信息，并在成功、超时和取消时统一清理；
* 默认使用同步调用和 Go 原生并发原语；没有容量或性能数据证明时，不引入 worker pool、插件注册表或额外协议模式；
* Event/Data 直接使用 Broker 的原始消息投递能力，不增加无意义的解码再编码层；
* 每个协议分支必须对未知消息类型和非法结构显式返回错误或记录拒绝原因，不得静默吞掉。

---

## 12. 验收条件

1. 设备可以通过增量 `report` 更新 reported state；嵌套对象递归合并、标量和数组替换、`null` 删除字段。
2. desired 合并后实际变化且与 reported 存在 delta 时，设备收到一个完整 `set`；重复写入或新 desired 等于 reported 时不主动下发。
3. 设备发送 `get` 后，可以收到当前完整 `set`。
4. desired version 仅在合并后的 desired 实际变化时递增；no-op 更新和 `report` 不递增。
5. 调用方可以通过 Tio 调用任意 Method，`d` 可以携带任意不透明业务参数。
6. `reply` 可以根据 ID 返回给对应调用方，完整 `d` 不被解析或裁剪。
7. Event 消息可以通过独立 Topic 原字节透传给业务消费者。
8. Data 消息可以通过独立 Topic 原字节透传给数据消费者。
9. Event 和 Data 不修改 Shadow。
10. 全局配置可以选择 JSON 或 CBOR，并覆盖当前协议模式的全部设备 MQTT 应用消息；两种编码得到等价的字符串键消息模型。
11. `protocol.mode` 可以选择 `legacy` 或 `simple`，默认 `legacy`；未知或空 mode 拒绝启动，且只注册选中模式的路径。
12. 设备只能向自己的 `up/event/data` 发布并订阅自己的 `down`；网关只能按绑定关系访问，其他方向、Topic 和 Thing 均被 ACL 拒绝。
