# 设备通信配置方案

## 1. 目标与配置

Tio 支持 Legacy、Simple 设备消息协议，以及 JSON、CBOR 控制消息编码。每个 Thing 通过专用字段保存通信配置，创建设备时指定，后续连接直接沿用持久化值。

```yaml
protocol:
  mode: dual
  encoding: json
```

| 部署模式 | 消息协议 | 载荷编码 |
|---|---|---|
| legacy | 全部 Legacy | protocol.encoding |
| simple | 全部 Simple | protocol.encoding |
| dual | Thing.messageProtocol，空值使用 Legacy | Thing.payloadEncoding，空值使用 protocol.encoding |

固定模式保留各自设备处理器和 HTTP 入口；Dual 同时启动两套处理器。部署 mode、encoding 的空值或未知值导致启动失败。同一集群采用一致的部署配置。

## 2. Thing 数据模型和管理 API

Thing 表增加两个字符串列，默认空字符串：

| Go 字段 | JSON 字段 | 数据库列 | 显式值 |
|---|---|---|---|
| MessageProtocol | messageProtocol | message_protocol | legacy、simple |
| PayloadEncoding | payloadEncoding | payload_encoding | json、cbor |

字段描述应用消息交互规则和序列化编码；MQTT/NATS 继续作为连接和传输方式。tags 只保存业务元数据，通信配置读取、校验及更新均围绕 Thing 字段进行。

创建请求示例：

```http
POST /api/v1/things/
Content-Type: application/json

{"thingId":"device-001","password":"device-password","messageProtocol":"simple","payloadEncoding":"cbor","tags":{"zone":"Shanghai"}}
```

管理更新使用现有 Thing 更新入口：

```http
PATCH /api/v1/things/device-001
Content-Type: application/json

{"messageProtocol":"simple","payloadEncoding":"cbor"}
```

约定：

- 创建时可以省略字段或使用空字符串，运行时分别取部署默认值。
- upsert 中省略或传空通信字段时保留已有值；需要清空时使用 Thing 更新接口。
- 更新接口使用可选字段，省略或 null 表示保持原值，空字符串表示恢复默认值。
- 创建、upsert 和更新在写入前复用服务层校验；非法输入不会改变凭证、启用状态或通信字段。
- 两个通信字段在 Thing 表中通过同一 UPDATE 提交。通信配置与凭证、启用状态可在同一条 Thing 更新中保存；同时修改业务 tags 时仍沿用现有跨表写入流程。
- GET、列表、创建响应和 OpenAPI 暴露这两个字段。schema 由现有 AutoMigrate 新增列。

## 3. 权限与消息处理

Dual 下设备获得自身 Legacy 与 Simple 权限集合的并集，网关获得自身及已绑定子设备的两套权限：

- 保留 Simple up/down 方向限制、现有自定义 Topic 能力及 Legacy 权限范围。
- Broker 拒绝包含受限路径的整树通配订阅；设备分别订阅 down、data 等所需范围。
- 跨设备和未绑定目标访问被拒绝；绑定查询失败拒绝认证，重复目标的权限去重。
- 业务超级用户获得两套命名空间及现有 JetStream/Inbox 权限；Simple 模式额外允许订阅 Presence Topic（供后端对接）；内部角色边界保持一致。

认证不依赖通信字段的合法性；非法配置由管理更新接口修复。业务控制消息在解码和执行前检查 Thing.messageProtocol；不匹配时停止处理。Shadow、Method、NTP 的编解码使用 Thing.payloadEncoding。

控制处理器负责协议选择；直接订阅者可能收到设备发布在另一套命名空间中的消息。data/event 和自定义 Topic 继续按原字节传输，消费者按自身契约消费。

通信配置改变无需刷新权限。绑定关系改变仍按连接权限快照机制重连刷新。

## 4. 读取接口与快照

connector.DeviceProfile 包含 MessageProtocol、PayloadEncoding，DeviceProfileGetter 提供 GetDeviceProfile。

thing.DeviceProfiles 通过 Thing 服务读取持久化字段并解析默认值。设备处理器、Method 路由和 Presence 通过该接口访问配置，固定模式继续使用部署配置。运行时查询不依赖本地 Shadow 缓存。

- 不存在的目标对应 404；非法持久化配置对应 409；配置读取不可用对应 503。
- JSON、CBOR 实例各创建一次并复用。
- 配置读取使用 2 秒 TTL 的进程内缓存，本节点写入后立即失效；其他节点的配置变更最多延迟一个 TTL。
- 单次控制操作只读取一次配置，直接回复使用同一 codec 快照。
- Method pending 保存调用 codec 快照，在途调用保持同一通信配置。
- 主动通知读取目标配置，读取失败则停止发布并记录原因。

## 5. Shadow、Method、NTP 和 Presence

Shadow 继续共用业务服务。Legacy accepted/rejected/delta/documents 和 Simple shadow_desired 仅向对应协议的设备发布，保留各协议触发条件、payload 和 QoS。Simple reported 更新不主动触发 desired 下发。

Dual 同时提供两个 Method HTTP 入口，按 Thing 字段自动分发：

| 入口 | 请求字段 |
|---|---|
| POST /api/v1/things/{id}/methods/{name} | data、connTimeout、respTimeout |
| POST /api/v1/things/{id}/invoke | method、params、timeout |

响应超时默认 30 秒、最大 300 秒；校验负数和超限值，支持 HTTP 取消。保留现有离线处理、Legacy ClientToken、Simple id 和响应结构。

Simple method_resp 建立独立广播订阅；普通上行队列订阅忽略该类型。各节点仅在本地存在 pending 时解码并关联响应。Legacy 保留广播方式。超时、取消及停止释放等待；节点退出后调用失败或超时，业务自行决定是否重试。

NTP 接收时间在读取设备配置前记录，避免将配置查询耗时计入网络时延。

内部 Presence/KV 和 HTTP 在线状态覆盖全部 Thing。Legacy、Simple、Dual 都对所有 Thing 发布 `$iothub/things/{thingId}/presence`（retained）和 `$iothub/events/things/{thingId}/presence`，供后端服务对接；两条消息编码统一使用部署 protocol.encoding，不随 Thing 配置变化。设备控制消息使用逐设备编码；HTTP、数据库 JSON、内部 NATS/KV、Rule payload、data/event 延续各自编码边界。

## 6. 同连接切换与多节点

切换步骤：

1. 管理端暂停目标 Thing 的 Method、desired 和其他主动下发。
2. 设备或代理网关停止业务消息，排空在途请求、回复和通知。
3. 通过管理更新接口保存新配置。
4. 按新配置验证 Shadow get 和 Method，然后恢复双方业务。

协议和编码变更使用维护窗口及单一配置写入方。服务端无法预知设备何时切换，也不提供自动排空或跨节点切换事务。普通重连直接复用配置，无需维护窗口。

网关按子设备 ID 分别配置，每个子设备配置独立。持久会话中的旧消息需排空，必要时使用 clean session；切换 Legacy/Simple 时清理旧 retained Presence。

所有节点从同一权威数据库或具备读后写一致性的连接读取 Thing 配置。一次已开始的操作沿用自身快照；切换前排空保证设备与服务器的快照一致。多次管理写入由维护流程串行协调。

## 7. 实现与验证

修改面包括 Thing 模型/实体/转换/创建与更新接口、统一 profile 服务、Dual 启动接线、认证权限并集、Presence 发布、设备处理器接线、Method 分发、配置样例和设备文档。复用现有依赖与 schema 管理。

验收覆盖：

- 创建时指定字段、列表、读取、局部更新、默认值及非法输入；upsert 省略字段保留原配置。
- 创建和更新通信字段后，Shadow tags/version 保持原有值；业务 tags 不影响通信配置。
- 两种协议与两种编码的四种组合，重连直接复用配置、同连接切换及 NTP 时间边界。
- 固定 Legacy/Simple 模式忽略 Thing 通信字段；Dual 按 Thing 选择处理器和 codec，错配消息被丢弃。
- 三种模式下所有 Thing 都发布 Presence，Simple 超级用户可订阅 Presence Topic。
- Shadow 出口隔离、两套 Method HTTP 入口、超时、取消、数据库故障和非法配置修复。
- 设备/网关权限、跨设备拒绝、受限通配订阅。
- 两节点的配置共享与 Method 广播响应，固定 Legacy/Simple 回归及新增并发路径 race 检查。

```sh
go test ./...
TIO_TEST_PROTOCOL=dual go test -tags integration ./integration_tests/...
```

## 8. 发布与成本

协调升级所有节点并通过 AutoMigrate 添加 Thing 列，使用新字段配置设备，启用 Dual 后逐批验证混合设备。回退固定模式前，先将设备通信方式对齐目标部署配置。

运行成本主要来自 profile 数据库读取、额外处理器/订阅和随节点数增长的 Method 响应广播。profile 读取已按节点缓存 2 秒，跨节点变更最多延迟一个 TTL。按实际负载测量吞吐、延迟、数据库 QPS、大网关绑定量及广播流量，再决定是否引入一致缓存或定向响应。

日志包含 thingId、messageProtocol、payloadEncoding、操作及错误原因；避免凭证、完整业务 payload 和高基数指标标签。生产压测与真实 MySQL 集群验证作为上线检查执行。
