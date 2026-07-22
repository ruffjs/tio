# NATS 集成设计：替代内嵌 MQTT Broker，支持多实例部署

**日期：** 2026-07-21
**状态：** Draft

## 1. 目标

- 用内嵌 NATS Server 替代当前的 mochi-mqtt 内嵌 MQTT Broker
- NATS 的 MQTT Gateway 处理设备的 MQTT 连接，设备端协议不变
- 通过 NATS 集群能力实现 tio 多实例对等部署
- 简化架构：删除 embed（mochi-mqtt）和 emqx 两种 connector 类型，只保留 NATS
- 简化 `connector` 接口，去掉 MQTT 特有语义（qos、retained、messageID）
- 保持现有设备侧语义：Shadow/Direct Method 仍按 MQTT QoS 1 投递，Presence 仍支持 retained

### 1.1 本阶段非目标

- JobCenter 暂不支持多实例协调，本阶段默认不启动，也不纳入多实例验收范围
- 不承诺业务 handler 的端到端 exactly-once；写数据库的 handler 仍需幂等
- 不开放任意 MQTT topic 字符集，应用 topic 必须符合第 5.4 节的受限格式

## 2. 总体架构

```
                    ┌─────────────────────────────────┐
                    │           tio instance 1         │
                    │  ┌───────────┐  ┌────────────┐  │
  MQTT devices ────►│  │NATS Server│  │ MQTT Gateway│  │
                    │  │ (embedded)│  │  (NATS)    │  │
                    │  └─────┬─────┘  └────────────┘  │
                    │        │ NATS protocol           │
                    │  ┌─────┴──────────────────────┐  │
                    │  │ NATS Connector              │  │
                    │  │ (Publisher/Subscriber/      │  │
                    │  │  Connectivity)              │  │
                    │  └────────────────────────────┘  │
                    │  shadow / method / ntp           │
                    └────────────┬─────────────────────┘
                                 │ NATS cluster
                    ┌────────────┴─────────────────────┐
                    │           tio instance 2         │
                    │  (same structure)                │
                    └──────────────────────────────────┘
                                 │
                          共享 MySQL 数据库
```

- 多个 tio 实例各自内嵌 NATS 节点，通过 NATS cluster routes 自动组网
- 所有实例共享同一个 MySQL 数据库
- 设备通过任意实例的 MQTT Gateway 连接
- 跨实例消息路由和事件广播由 NATS 原生处理
- 相同业务 handler 使用 NATS queue group，集群内只有一个实例处理一条请求
- JobCenter 在本阶段关闭；启用前必须另行设计 leader/lease 或任务 claim

## 3. 包结构

### 3.1 新增

```
connector/
├── connector.go          # 简化后的接口定义
└── nats/                 # 【新增】
    ├── nats.go           # NewNatsConnector / ConfigureAuth 入口
    ├── server.go         # 内嵌 NATS server 管理
    ├── connector.go      # 实现 connector.Connector 接口
    ├── mqtt_publisher.go # 仅用于设备侧 QoS 1 / retained 发布
    ├── presence.go       # 设备上下线（presence）处理
    ├── auth.go           # MQTT gateway 认证 hook
    └── topic.go          # MQTT topic ↔ NATS subject 转换

rule/source/
├── source.go             # 注册表
└── nats.go               # 【新增】NATS source

rule/sink/
└── nats.go               # 【新增】NATS sink
```

### 3.2 删除

| 删除项 | 原因 |
|--------|------|
| `connector/mqtt/` 整个目录 | embed、emqx、client 全部删除 |
| `connector/mqtt/embed/` | mochi-mqtt 不再使用 |
| `connector/mqtt/emqx/` | EMQX 集成不再使用 |
| `connector/mqtt/client/` | 原通用 client 删除；Paho 依赖保留，仅封装为 NATS connector 的内部 MQTT publisher |
| `connector/mqtt/mqtt.go` | connector 初始化逻辑由 `connector/nats/nats.go` 替代 |
| `rule/source/embedmqtt.go` | 改为 `nats.go` |
| `rule/source/mqtt.go` | 不再需要 |
| `rule/sink/mqtt.go` | 改为 `nats.go`（或 NATS sink） |

### 3.3 保留不变

- `shadow/link.go`、`shadow/merge.go`、`shadow/sqlparser.go` 等业务逻辑
- `job/` 代码保留，但本阶段不初始化 JobCenter
- `thing/` 全部
- `api/`、`web/`、`metrics/`
- `db/`
- `pkg/eventbus/`（继续用于单进程内 Presence 广播，不承担跨实例传输）
- `config/config.go`（结构变化，文件保留）

## 4. 简化后的 Connector 接口

```go
// connector/connector.go

// Presence 相关常量保留
const (
    TopicEventPresenceTmpl = "$iothub/events/things/{thingId}/presence"
    TopicEventPresenceAll  = "$iothub/events/things/+/presence"
    TopicPresenceTmpl      = "$iothub/things/{thingId}/presence"
    TopicPresenceAll       = "$iothub/things/+/presence"
)

const (
    EventConnected    = "connected"
    EventDisconnected = "disconnected"
)

type ClientInfo struct {
    ClientId         string     `json:"clientId"`
    Username         string     `json:"username"`
    Connected        bool       `json:"connected"`
    ConnectedAt      *time.Time `json:"connectedAt"`
    DisconnectedAt   *time.Time `json:"disconnectedAt"`
    DisconnectReason string     `json:"disconnectReason"`
    RemoteAddr       string     `json:"remoteAddr"`
}

type Connector interface {
    Connectivity
    PubSub
}

type PubSub interface {
    Subscriber
    Publisher
}

type Publisher interface {
    // NATS Core -> MQTT QoS 0，适合 NTP 响应等可丢消息。
    Publish(topic string, payload []byte) error

    // 通过 connector 内部 MQTT publisher 以 QoS 1 发布。
    // 返回 nil 表示本地 NATS MQTT gateway 已 PUBACK，不表示设备已处理。
    PublishReliable(topic string, payload []byte) error

    // 通过 connector 内部 MQTT publisher 以 QoS 1 + retain 发布。
    // 空 payload 按 MQTT 语义删除 retained message。
    PublishRetained(topic string, payload []byte) error
}

type Subscriber interface {
    // 广播订阅：每个调用者都收到消息。仅用于本实例确实需要独立副本的场景。
    Subscribe(ctx context.Context, topic string, callback func(msg Message)) error

    // 竞争订阅：同一个 queue 中只有一个 tio 实例收到消息。
    QueueSubscribe(ctx context.Context, topic, queue string, callback func(msg Message)) error
}

type Message interface {
    // 保持 MQTT topic 格式（connector 内部做转换）
    Topic() string
    Payload() []byte
}

type Connectivity interface {
    ConnectChecker
    Start(ctx context.Context) error
    Close(thingId string) error
    Remove(thingId string) error
}

type ConnectChecker interface {
    IsConnected(thingId string) (bool, error)
    // 每次调用返回独立的广播订阅；ctx 结束时关闭该 channel。
    SubscribePresence(ctx context.Context) <-chan PresenceEvent
    ClientInfo(thingId string) (ClientInfo, error)
    AllClientInfo() ([]ClientInfo, error)
}

type PresenceEvent struct {
    Timestamp        int64  `json:"timestamp"`
    EventType        string `json:"eventType"`
    ThingId          string `json:"thingId"`
    ClientId         string `json:"clientId"`
    RemoteAddr       string `json:"remoteAddr"`
    DisconnectReason string `json:"disconnectReason,omitempty"`
}
```

### 4.1 接口变化对照

| 旧接口 | 新接口 | 变化 |
|--------|--------|------|
| `Publish(topic, qos, retained, payload)` | `Publish(topic, payload)` + `PublishReliable` + `PublishRetained` | 按设备侧投递语义分方法 |
| `Subscribe(ctx, topic, qos, cb)` | `Subscribe(ctx, topic, cb)` | 去掉 qos |
| 无 | `QueueSubscribe(ctx, topic, queue, cb)` | 多实例 handler 竞争消费 |
| `Message.Qos()` | 删除 | NATS 无 QoS 概念 |
| `Message.Retained()` | 删除 | 通过 PublishRetained 处理 |
| `Message.MessageID()` | 删除 | NATS 无此概念 |
| `Message.Topic()` | 保留 | 对外值仍是 MQTT topic，避免用 Subject 命名误导调用方 |
| `Message.Ack()` | 删除 | NATS core 无需 Ack |

### 4.2 消费者方法映射

| 场景 | 调用方法 | 原 QoS |
|------|---------|--------|
| Shadow update/get 响应 | `PublishReliable()` | 1 |
| Direct method 请求/响应 | `PublishReliable()` | 1 |
| NTP 响应 | `Publish()` | 0 |
| Presence | `PublishRetained()` | 1 + retained |

`PublishReliable` 不使用 `nats.JetStreamContext.Publish`。NATS 原生消息投递到 MQTT subscription 时固定为 QoS 0；为保持现有 QoS 1，connector 内部保留一个连接本地 MQTT gateway 的系统 publisher。Paho 只存在于该私有实现中，不暴露给业务代码。

业务请求订阅映射如下：

| 场景 | 订阅方法 | Queue |
|------|---------|-------|
| Shadow update/get 请求 | `QueueSubscribe` | `tio-shadow` |
| NTP 请求 | `QueueSubscribe` | `tio-ntp` |
| Direct method 响应 | `Subscribe` | 每个实例均需收到，以匹配本地 waiting token |
| Presence 系统事件 | `SubscribePresence` | 广播给本实例所有消费者 |

Direct Method 不使用 NATS 原生 request/reply。设备仍通过固定 MQTT request/response topic 通信，无法消费 NATS `Reply` header 或向动态 inbox 回复。发起请求的 tio 实例必须先以 `(thingId, clientToken)` 注册本地 pending，再通过 `PublishReliable` 发送请求；设备在固定 response topic 回显 `clientToken`。所有 tio 实例使用普通 `Subscribe` 接收响应，只有持有匹配 pending 的实例完成 waiter，其他实例静默忽略。响应订阅必须在实例对外就绪前完成创建和 `Flush`，禁止改为 `QueueSubscribe`，否则响应可能被没有本地 waiter 的实例独占。重复的在途 `(thingId, clientToken)` 必须拒绝，超时或 context 取消必须删除 pending。

## 5. NATS 内嵌 Server

### 5.1 依赖

```
go.mod:
+ github.com/nats-io/nats-server/v2    # 内嵌 NATS server
+ github.com/nats-io/nats.go           # NATS client
- github.com/mochi-mqtt/server/v2      # 移除
  github.com/eclipse/paho.mqtt.golang  # 保留，仅用于内部 MQTT QoS 1/retained publisher
```

### 5.2 Server 启动

```go
// connector/nats/server.go

type NatsServer struct {
    server *natsserver.Server
    opts   NatsServerConfig
}

type NatsServerConfig struct {
    ServerName        string    // 每个节点唯一，例如 tio-node-1
    ClusterName       string    // 集群内一致，例如 tio
    Port              int       // NATS client port (默认 4222)
    ClusterPort       int       // 集群通信端口 (默认 6222)
    ClusterAdvertise  string    // 容器/NAT 环境必须配置
    Routes            []string  // 集群路由
    MqttPort          int       // MQTT gateway 端口 (默认 1883)
    WsPort            int       // WebSocket 端口
    StoreDir          string    // 每个节点独占的持久化目录
    JetStreamMaxMemory int64
    JetStreamMaxStore  int64
    MqttStreamReplicas int      // 单机为 1，三节点生产集群为 3
    PresenceReplicas   int      // 单机为 1，三节点生产集群为 3
    ClusterUser       string
    ClusterPassword   string
    ClientTLS         TLSConfig // NATS app/system client listener
    MqttTLS           TLSConfig // 设备 MQTT listener，mTLS 在这里配置
    WebsocketTLS      TLSConfig
    RouteTLS          TLSConfig
}

type TLSConfig struct {
    CertFile          string
    KeyFile           string
    CAFile            string
    RequireClientCert bool
}

func StartNatsServer(cfg NatsServerConfig, authn natsserver.Authentication) (*NatsServer, error) {
    opts := &natsserver.Options{
        ServerName: cfg.ServerName,
        Host:       "0.0.0.0",
        Port:       cfg.Port,
        JetStream:  true,
        StoreDir:   cfg.StoreDir,
        JetStreamMaxMemory: cfg.JetStreamMaxMemory,
        JetStreamMaxStore:  cfg.JetStreamMaxStore,
        Cluster: natsserver.ClusterOpts{
            Name: cfg.ClusterName,
            Host: "0.0.0.0",
            Port: cfg.ClusterPort,
            Advertise: cfg.ClusterAdvertise,
        },
        Mqtt: natsserver.MQTTOpts{
            Port: cfg.MqttPort,
            StreamReplicas: cfg.MqttStreamReplicas,
        },
        Websocket: natsserver.WebsocketOpts{
            Port: cfg.WsPort,
        },
        CustomClientAuthentication: authn,
    }
    // 解析 routes；构造 MQTT、NATS client 和 route TLS 配置。
    // MQTT 只监听一个端口：配置证书后该端口启用 TLS，不再提供独立 mqttSslPort。
    svr, err := natsserver.NewServer(opts)
    if err != nil {
        return nil, err
    }
    svr.Start()
    if !svr.ReadyForConnections(10 * time.Second) {
        return nil, errors.New("nats server not ready")
    }
    return &NatsServer{server: svr, opts: cfg}, nil
}
```

启动约束：

- MQTT gateway 要求 JetStream，`JetStream` 不能关闭
- 显式创建 APP 和 SYS 两个 account：APP 开启 JetStream并承载设备/业务消息，SYS 设置为 `SystemAccount`；动态设备用户和 app client 注册到 APP，监控 client 注册到 SYS
- `storeDir` 必须是本节点独占的本地持久卷，多个实例禁止共享目录
- `serverName` 每节点唯一，`clusterName` 所有节点一致
- 三节点生产集群将 MQTT internal streams 和 Presence KV 的 replicas 固定为 3；单机固定为 1，不依赖节点数自动推断
- 启动后先等待 NATS ready，再幂等创建/校验 `TIO_PRESENCE` KV；已有 bucket 的 replicas、history 或 TTL 不匹配时启动失败，不能静默沿用
- 必须检查 `NewServer`、route 解析、JetStream 初始化和 KV bootstrap 的每一个错误
- 一个 NATS server 只有一个 MQTT listener。`mqttTls` 为空时为明文 MQTT，非空时为 TLS MQTT；单节点不再同时暴露 1883 和 8883。确需同时兼容时，在同一 NATS cluster 中部署两组节点并由独立 Service/LB 暴露明文和 TLS 端口
- `mqttTls.caFile` 存在时使用 `VerifyClientCertIfGiven`，由 `AuthzFn` 根据 thing 的认证类型决定是否必须有证书，使密码设备和 mTLS 设备可共用 TLS listener

### 5.3 MQTT Gateway 认证与授权

内嵌 NATS server 使用 `Options.CustomClientAuthentication`。该接口可以读取 CONNECT username/password、TLS connection state 和 remote address，并通过 `ClientAuthentication.RegisterUser(*server.User)` 为本次连接绑定动态用户及 subject permissions，因此可以复用 tio 现有的 thing 认证逻辑，而不需要预先为每个 thing 写静态 NATS 配置。

```go
// connector/nats/auth.go

type AuthzFn func(authCtx AuthContext) (AuthResult, bool)

type AuthContext struct {
    ClientIdentifier string
    Username         string
    Password         string
    HasClientCert    bool
    CertCN           string
}

type AuthResult struct {
    Principal  string  // thingId
    AuthMethod string
}
```

认证器按以下顺序处理：

1. 匹配固定内部用户并限制 connection type：app/system clients 只允许 NATS，内部 MQTT publisher 只允许 MQTT
2. 其余连接必须是 MQTT；提取 username/password、client identifier、TLS peer certificate 和 remote address
3. 调用现有 `AuthzFn`，支持普通 thing 密码、provisioning 和 mTLS 证书身份
4. 认证成功后使用 `RegisterUser` 注册一次性 `server.User`，Username 固定为认证结果中的 `Principal`，而不是信任 CONNECT username
5. 根据 `TopicAcl` 的结果生成最小 NATS permissions；MQTT topic 先按第 5.4 节转换为 NATS subject

普通 thing 的权限至少包含：

- publish/subscribe 自己的 `$iothub.things.<thingId>.>` 和 `$iothub.user.things.<thingId>.>`
- MQTT QoS 1 subscription 所需的内部 `$MQTT.sub.>` 权限
- 禁止 `$SYS.>`、`$JS.>`、`$KV.>`、`$tio.control.>` 及其他 thing 的保留 subject

Gateway thing 的 permissions 在连接时加入当前已绑定 things。绑定关系改变后，thing service 必须调用 `Close(gatewayThingId)` 强制 gateway 重连并重新生成 permissions，避免旧连接继续持有过期权限。

`NoAuthUser` 在所有环境均禁用。认证器初始化失败、数据库不可用或无法生成完整 permissions 时一律拒绝连接，不允许降级为匿名访问。

需要锁定并测试一个明确的 `nats-server/v2` 版本；升级依赖时必须重新运行 custom authentication、mTLS、provisioning、ACL 越权和 MQTT QoS 1 集成测试。

### 5.4 MQTT Topic ↔ NATS Subject 转换

```go
// connector/nats/topic.go

func MqttTopicToNatsSubject(topic string) (string, error) {
    // 校验后再做逐 token 转换；不接受空 token、字面量 '.'、空白或非法 wildcard。
}

func NatsSubjectToMqttTopic(subject string) (string, error) {
    // 仅转换通过本模块校验、位于应用 namespace 下的 subject。
}
```

| MQTT Topic | NATS Subject |
|-----------|-------------|
| `$iothub/things/{thingId}/shadow/get` | `$iothub.things.{thingId}.shadow.get` |
| `$iothub/things/+/shadow/get` | `$iothub.things.*.shadow.get` |
| `$iothub/things/#` | `$iothub.things.>` |

为保证双向转换无歧义，本项目的应用 MQTT topic 采用以下受限格式：

- topic level 不能为空，因此不支持开头/结尾 `/` 或连续 `//`
- level 中禁止字面量 `.`、空白、`*`、`>`；thingId、shadow name 等进入 topic 的标识符必须执行相同校验
- `+` 必须独占一个 level；`#` 必须独占最后一个 level；publish topic 禁止 wildcard
- 订阅 `foo/#` 时创建 `foo` 和 `foo.>` 两个底层 NATS subscriptions，因为 MQTT 的 `#` 也匹配父 topic；取消时同时释放
- 规则引擎 source 配置继续使用 MQTT topic 格式，不直接接受原始 NATS subject

转换函数必须返回错误，调用方不得把非法 topic 交给 NATS。该限制与 NATS MQTT gateway 的 topic 映射行为保持一致，并由表驱动测试覆盖。

## 6. NATS Connector 实现

### 6.1 核心结构

```go
// connector/nats/connector.go

type natsConnector struct {
    natsConn    *nats.Conn              // APP account
    sysConn     *nats.Conn              // SYS account，只用于 events/monitoring
    natsSvr     *NatsServer
    js          nats.JetStreamContext  // MQTT gateway 和 Presence KV bootstrap
    kv          nats.KeyValue          // 仅用于全局 presence 状态
    mqttPub     *mqttPublisher         // 本地 MQTT QoS 1 / retained system client
    presenceBus *eventbus.EventBus[PresenceEvent] // 每个订阅者独立 channel
    clients     sync.Map               // 本地客户端信息缓存
    ctx         context.Context
    cancel      context.CancelFunc
}

var _ connector.Connector = (*natsConnector)(nil)
```

`NewNatsConnector` 只做配置校验和对象构造，不启动网络服务。`ConfigureAuth` 必须且只能在 `Start` 前调用一次；未配置认证、重复配置或重复启动均返回错误。这样 thing service 可以先构造，再作为认证/ACL 的依赖注入 connector，避免初始化依赖环。

`Start` 采用失败回滚：任何一步失败都关闭已创建的 MQTT/NATS clients 并 shutdown embedded server。应用退出时先停止接受 HTTP 请求和新 MQTT 连接，再 drain 业务 subscriptions、关闭内部 clients，最后调用 `Server.Shutdown()` 并等待完成，避免遗留半初始化节点或丢失正在处理的 callback。

### 6.2 Publisher

```go
// QoS 0: NATS Core -> MQTT QoS 0
func (c *natsConnector) Publish(topic string, payload []byte) error {
    subject, err := MqttTopicToNatsSubject(topic)
    if err != nil { return err }
    return c.natsConn.Publish(subject, payload)
}

// MQTT QoS 1；等待本地 gateway PUBACK。
func (c *natsConnector) PublishReliable(topic string, payload []byte) error {
    return c.mqttPub.Publish(topic, 1, false, payload)
}

// MQTT QoS 1 + retain；由 NATS MQTT gateway 管理 retained store。
func (c *natsConnector) PublishRetained(topic string, payload []byte) error {
    return c.mqttPub.Publish(topic, 1, true, payload)
}
```

内部 MQTT publisher 使用独立且全局唯一的 client ID（包含 `serverName`），只连接本机 gateway，并使用只允许发布业务输出 topic 的 system user。连接断开时自动重连；`PublishReliable`/`PublishRetained` 必须等待 token 完成并传播错误。它不能订阅业务 topic，也不能访问 `$SYS.>`、`$JS.>` 或 `$tio.control.>`。

### 6.3 Subscriber

```go
func (c *natsConnector) Subscribe(ctx context.Context, subject string,
    callback func(msg connector.Message)) error {
    return c.subscribe(ctx, subject, "", callback)
}

func (c *natsConnector) QueueSubscribe(ctx context.Context, subject, queue string,
    callback func(msg connector.Message)) error {
    if queue == "" { return errors.New("queue is required") }
    return c.subscribe(ctx, subject, queue, callback)
}
```

`subscribe` 负责 topic 校验和 `foo/#` 的双 subscription 展开。`queue == ""` 时调用 `Subscribe`，否则调用 `QueueSubscribe`。所有底层 subscriptions 创建成功后执行 `Flush`，任一失败则回滚已创建项；启动 goroutine 监听 `ctx.Done()` 并对全部 subscriptions 执行 `Drain`/`Unsubscribe`，因此 handler 或规则重启不会遗留订阅。callback panic 必须被隔离并记录，不能终止 NATS async dispatcher。

### 6.4 natsMessage

```go
type natsMessage struct {
    topic   string
    payload []byte
}

func (m *natsMessage) Topic() string   { return m.topic }
func (m *natsMessage) Payload() []byte { return m.payload }
```

### 6.5 Connectivity

```go
func (c *natsConnector) Start(ctx context.Context) error {
    svr, err := StartNatsServer(c.cfg.Server, c.authenticator)
    if err != nil { return err }
    c.natsSvr = svr
    if err := c.connectAppAndSystemClients(ctx); err != nil { return err }
    if err := c.bootstrapJetStream(ctx); err != nil { return err }
    if err := c.initPresence(ctx); err != nil { return err }
    if err := c.mqttPub.Connect(ctx); err != nil { return err }
    return nil
}

func (c *natsConnector) IsConnected(thingId string) (bool, error) {
    entry, err := c.kv.Get("presence." + thingId)
    if err != nil {
        if err == nats.ErrKeyNotFound {
            return false, nil
        }
        return false, err
    }
    var evt PresenceEvent
    if err := json.Unmarshal(entry.Value(), &evt); err != nil { return false, err }
    return evt.EventType == EventConnected, nil
}

func (c *natsConnector) SubscribePresence(ctx context.Context) <-chan PresenceEvent {
    ch := c.presenceBus.Subscribe(presenceEventName)
    go func() {
        <-ctx.Done()
        c.presenceBus.Unsubscribe(presenceEventName, ch)
    }()
    return ch
}

func (c *natsConnector) Close(thingId string) error {
    // 从 KV 查 owner server/client connection ID，向 owner 的控制 subject 发 request。
    // owner 调用 embedded server DisconnectClientByID；收到确认后等待 disconnect event。
}

func (c *natsConnector) Remove(thingId string) error {
    if err := c.Close(thingId); err != nil && !errors.Is(err, ErrNotConnected) { return err }
    if err := c.PublishRetained(connector.TopicPresence(thingId), nil); err != nil { return err }
    return c.kv.Delete("presence." + thingId)
}

func (c *natsConnector) ClientInfo(thingId string) (ClientInfo, error) {
    // 查 KV
}

func (c *natsConnector) AllClientInfo() ([]ClientInfo, error) {
    // 遍历 KV 中所有 presence.* 条目
}
```

## 7. 多实例协调

### 7.1 Presence 事件跨实例分发

Presence 不依赖未公开的 MQTT hook，使用独立 system account 连接订阅：

- `$SYS.ACCOUNT.<app-account-id>.CONNECT`
- `$SYS.ACCOUNT.<app-account-id>.DISCONNECT`
- `$SYS.SERVER.<server-id>.SHUTDOWN`

只处理 `ClientInfo.ClientType/Kind` 表明为 MQTT，且 user 是动态认证得到的 thing principal 的事件。状态值增加 `ServerId`、NATS connection `Id`、`Generation` 和 `ObservedAt`；KV key 为 `presence.<thingId>`。同一 thing 重连时使用 generation/时间戳做 compare-and-update，旧连接的延迟 DISCONNECT 不能覆盖新连接的 CONNECT。

系统事件本身是 Core NATS，不能假设 CONNECT/DISCONNECT 永不丢失。因此每个实例还运行 reconciliation loop：

1. 每 10 秒通过 system service 请求所有节点的 account `CONNZ`
2. 聚合当前 MQTT connections，并与 `TIO_PRESENCE` KV 比较
3. 补发缺失的 connected/disconnected 变化；连续两轮均不存在才判定 disconnected，避免拓扑短暂抖动
4. 启动完成后必须先做一次全量 reconciliation，再向业务层报告 ready

每个确认后的变化按固定顺序处理：

1. CAS 更新 `TIO_PRESENCE` KV；CAS 失败时重新读取并按 generation 决定是否重试。只有成功改变状态的 CAS winner 继续执行 2–4，其他实例不重复发布
2. publish `$tio.events.presence.<thingId>`，供所有 tio 实例更新本地缓存
3. 通过 `PublishRetained(TopicPresence(...))` 发布 retained 状态
4. 通过 `PublishReliable(TopicPresenceEvent(...))` 发布事件通知
5. 本实例收到 `$tio.events.presence.>` 后调用进程内 eventbus 的 `Publish`；每个 `SubscribePresence(ctx)` 调用者均得到独立副本

eventbus 的慢消费者策略必须显式：每个订阅者使用有界 buffer；buffer 满时记录 subscriber 名称和指标，并触发该订阅者下一轮从 KV 全量同步，不能静默丢失后继续假装状态完整。

### 7.2 Client 信息聚合

```
System events + reconciliation → CAS 写入 NATS KV
AllClientInfo()                → 遍历 KV 所有 presence.* 条目 → 返回全局视图
IsConnected()                  → 查 KV 单条 → 返回单设备状态
Close()                        → request `$tio.control.<serverId>.disconnect`
```

每个实例仅订阅自己的 `$tio.control.<serverId>.disconnect`。请求携带 thingId、connection ID 和随机 request ID；owner 再次校验当前 connection 属于该 thing 后调用 `NatsServer.server.DisconnectClientByID`。控制 subject 只允许 system connector user 访问。超时返回错误，禁止用“直接清 KV”冒充已断开。

### 7.3 集群组网

静态路由配置，NATS 原生自动发现：

```yaml
nats:
  server:
    serverName: tio-node-1
    clusterName: tio
    port: 4222
    clusterPort: 6222
    clusterAdvertise: 10.0.0.1:6222
    routes:
      - "nats://10.0.0.2:6222"
      - "nats://10.0.0.3:6222"
```

单实例时 routes 为空，NATS 单机运行，行为等同于当前 embed 模式。

生产集群的最小受支持规模为 3 个 JetStream 节点。两节点集群无法在失去一个节点后维持多数派，不作为生产拓扑。route 连接必须配置独立凭据和 mTLS；不能复用设备或 system client 凭据。

## 8. 规则引擎

### 8.1 NATS Source（替代 embedmqtt.go 和 mqtt.go）

```go
// rule/source/nats.go

const TypeNats = "nats"

func init() {
    Register(TypeNats, newNatsSource)
}

type NatsConfig struct {
    Topic string `json:"topic"` // 受限 MQTT topic 格式，支持 +/# 通配符
}

type natsSource struct {
    ctx      context.Context
    name     string
    config   NatsConfig
    sub      *nats.Subscription
    handlers sync.Map
    mu       sync.RWMutex
    started  bool
    status   model.StatusInfo
    metric   Metric
}
```

- 规则引擎 source 产出 `Msg{ThingId, Topic, Payload}`，Topic 保持 MQTT 格式
- MQTT topic 到 NATS subject 的转换在 source 内部完成
- source 一律使用 `QueueSubscribe`，queue 由稳定 rule ID 计算为 `tio-rule-<ruleId>`；同一条规则在多实例中只执行一次，不允许用户通过空配置意外切换为广播
- `ctx` 取消时释放全部底层 subscriptions
- processor 和 sink 无需改动

### 8.2 NATS Sink（替代 mqtt.go sink）

```go
// rule/sink/nats.go

// 配置中指定目标 MQTT topic 和 delivery: core | qos1 | retained
// 分别调用 connector.Publish / PublishReliable / PublishRetained
```

## 9. 配置

### 9.1 新配置结构

```go
// config/config.go

const ConnectorNats = "nats"

type Config struct {
    Log struct {
        Level string `json:"level,omitempty"`
    } `json:"log"`
    API struct {
        Port      int          `json:"port"`
        Cors      bool         `json:"cors"`
        BasicAuth UserPassword `json:"basicAuth"`
    } `json:"api"`
    DB struct {
        Typ    string        `json:"type" mapstructure:"type"`
        Mysql  mysql.Config  `json:"mysql"`
        Sqlite sqlite.Config `json:"sqlite"`
    } `json:"db"`
    Connector       Connector `json:"connector"`
    ProvisionSecret string    `json:"-"`
    Shadow          struct {
        IgnoreMetadataFor []string `json:"ignoreMetadataFor"`
    } `json:"shadow"`
    Pprof struct {
        Port    int  `json:"port"`
        Enabled bool `json:"enabled"`
    } `json:"pprof"`
}

type Connector struct {
    Typ  string     `json:"type" mapstructure:"type"`
    Nats NatsConfig `json:"nats"`
}

type NatsConfig struct {
    Server       NatsServerConfig `json:"server"`
    AppClient    NatsClientConfig `json:"appClient"`
    SystemClient NatsClientConfig `json:"systemClient"`
    MqttPublisher NatsClientConfig `json:"mqttPublisher"`
    SuperUsers   []UserPassword   `json:"superUsers"`
}

type NatsServerConfig struct {
    ServerName        string   `json:"serverName"`
    ClusterName       string   `json:"clusterName"`
    Port              int      `json:"port"`
    ClusterPort       int      `json:"clusterPort"`
    ClusterAdvertise  string   `json:"clusterAdvertise"`
    Routes            []string `json:"routes"`
    MqttPort          int      `json:"mqttPort"`
    WsPort            int      `json:"wsPort"`
    StoreDir          string   `json:"storeDir"`
    JetStreamMaxMemory int64   `json:"jetStreamMaxMemory"`
    JetStreamMaxStore  int64   `json:"jetStreamMaxStore"`
    MqttStreamReplicas int     `json:"mqttStreamReplicas"`
    PresenceReplicas   int     `json:"presenceReplicas"`
    ClusterUser        string   `json:"clusterUser"`
    ClusterPassword    string   `json:"-"`
    ClientTLS          TLSConfig `json:"clientTls"`
    MqttTLS            TLSConfig `json:"mqttTls"`
    WebsocketTLS       TLSConfig `json:"websocketTls"`
    RouteTLS           TLSConfig `json:"routeTls"`
}

type TLSConfig struct {
    CertFile          string `json:"certFile"`
    KeyFile           string `json:"keyFile"`
    CAFile            string `json:"caFile"`
    RequireClientCert bool   `json:"requireClientCert"`
}

type NatsClientConfig struct {
    User     string `json:"user"`
    Password string `json:"-"`
    TLS      TLSClientConfig `json:"tls"`
}

type TLSClientConfig struct {
    CAFile   string `json:"caFile"`
    CertFile string `json:"certFile"`
    KeyFile  string `json:"keyFile"`
}
```

### 9.2 默认配置

```yaml
# config.default.yaml

api:
  port: 9000
  cors: true
  basicAuth:
    name: admin
    password: public

db:
  type: sqlite
  sqlite:
    filePath: tio.sqlite
    showSql: true
  mysql:
    host: 127.0.0.1
    port: 3306
    user: tio
    password: public
    db: tio
    charset: utf8
    timezone: Asia%2FShanghai
    maxIdleConns: 4
    maxOpenConns: 50
    connMaxLifetime: 60
    showSql: true

connector:
  type: nats
  nats:
    server:
      serverName: tio-node-1
      clusterName: tio
      port: 4222
      clusterPort: 6222
      clusterAdvertise: ""
      routes: []
      mqttPort: 1883
      wsPort: 8083
      storeDir: ./data/nats/tio-node-1
      jetStreamMaxMemory: 268435456
      jetStreamMaxStore: 1073741824
      mqttStreamReplicas: 1
      presenceReplicas: 1
      clusterUser: $tio-route
      clusterPassword: public
      clientTls: {}
      mqttTls: {}
      websocketTls: {}
      routeTls: {}
    systemClient:
      user: $tio-sys
      password: public
    appClient:
      user: $tio-app
      password: public
    mqttPublisher:
      user: $tio-mqtt-publisher
      password: public
    superUsers:
      - name: $biz
        password: public

log:
  level: debug
```

上面的密码只用于本地开发。生产环境必须从环境变量/secret 注入，不得在 YAML 中使用默认密码。三节点部署必须为每个节点设置唯一 `serverName` 和 `storeDir`，统一 `clusterName`，并将两个 replicas 配置改为 3。

## 10. main.go 初始化流程

```go
func main() {
    cfg := config.ReadConfig()
    initLogger(cfg.Log)

    ctx, cancel := context.WithCancel(context.Background())
    // ... signal handling ...

    dbConn := newDb(cfg)
    autoMigrate(dbConn)

    // 仅构造 connector，此时不启动 server，解决 thingSvc 与动态认证的依赖环。
    natsConnector, err := natsConn.NewNatsConnector(cfg.Connector)
    if err != nil { ... }
    var conn connector.Connector = natsConnector

    methodHandler := shadow.NewMethodHandler(conn)
    shadowStateHandler := shadow.NewShadowHandler(conn)
    ntpHandler := ntp.NewNtpHandler(conn)

    shadowSvc := shadowWire.InitSvc(dbConn, conn, cfg.Shadow)
    thingSvc := thingWire.InitSvc(ctx, dbConn, shadowSvc, conn)

    // 本阶段不创建、不启动 JobCenter，也不注册 Job API。

    provisionSvc := thing.NewProvision(thingSvc, cfg.ProvisionSecret)
    if cfg.ProvisionSecret == "" { provisionSvc = nil }
    authzFn := auth.AuthzMqttClient(ctx, cfg.Connector.Nats.SuperUsers, thingSvc, provisionSvc)
    aclFn := auth.TopicAcl(thingSvc, cfg.Connector.Nats.SuperUsers)
    if err := natsConnector.ConfigureAuth(authzFn, aclFn); err != nil { ... }

    if err := conn.Start(ctx); err != nil { ... }

    // Connector ready 后才创建业务订阅。
    shadowSvc.Init(ctx)
    ruleMgr := rule.NewRuleMgr()
    ruleMgr.Boot(ctx, shadowSvc)
    if err := methodHandler.InitMethodHandler(ctx); err != nil { ... }
    if err := ntpHandler.InitNtpHandler(ctx); err != nil { ... }
    if err := shadow.Link(ctx, shadowStateHandler, shadowSvc, cfg.Shadow); err != nil { ... }

    // HTTP API（不变）
    // ...
}
```

## 11. 受影响的消费者改动

### 11.1 shadow/shadow.go

```go
// 旧：
err = h.client.Publish(topic, DefaultQos, false, j)
// 新：
err = h.client.PublishReliable(topic, j)
h.client.QueueSubscribe(ctx, requestTopic, "tio-shadow", func(msg connector.Message) { ... })
```

### 11.2 shadow/service.go

```go
// 旧：
connEventCh := s.connectorChecker.OnConnect()
// 新：
connEventCh := s.connectorChecker.SubscribePresence(ctx)
```

### 11.3 shadow/method.go

```go
// 旧：
h.connector.Publish(topic, 1, false, j)
h.connector.Subscribe(ctx, topic, 1, func(msg connector.Message) { ... })
// 新：
h.connector.PublishReliable(topic, j)
h.connector.Subscribe(ctx, topic, func(msg connector.Message) { ... })
presenceCh := h.connector.SubscribePresence(ctx)
```

Direct method response 使用广播订阅，因为只有发起调用的实例持有对应 waiting token；thing online 监听改用独立的 Presence 广播订阅。

### 11.4 ntp/ntp.go

```go
// 旧：
h.client.Subscribe(ctx, topic, DefaultQos, func(msg connector.Message) { ... })
h.client.Publish(TopicResp(thingId), 0, false, j)
// 新：
h.client.QueueSubscribe(ctx, topic, "tio-ntp", func(msg connector.Message) { ... })
h.client.Publish(TopicResp(thingId), j)
```

### 11.5 job/

本阶段代码保留但不适配、不初始化。后续启用前必须先补充多实例调度设计。

### 11.6 auth/

- `auth/mqtt_auth.go` 重写为 NATS MQTT gateway 认证
- `auth.TopicAcl()` 适配为 NATS subject ACL

## 12. 不需要改动的模块

| 模块 | 原因 |
|------|------|
| `shadow/link.go` | 通过 StateHandler 接口，签名不变 |
| `shadow/merge.go`、`shadow/sqlparser.go` | 纯数据处理 |
| `job/` | 代码保留，本阶段禁用 |
| `thing/` | 仍调用 `Connectivity.Close/Remove`，这两个签名不变 |
| `api/`、`web/`、`metrics/` | HTTP 层，不涉及 connector |
| `db/` | 数据库层 |

## 13. 依赖变化

```
go.mod:
+ github.com/nats-io/nats-server/v2
+ github.com/nats-io/nats.go
- github.com/mochi-mqtt/server/v2
  github.com/eclipse/paho.mqtt.golang  # 保留，仅供 connector/nats/mqtt_publisher.go 内部使用
- github.com/mochi-mqtt/server/v2/hooks/storage/badger
- github.com/mochi-mqtt/server/v2/hooks/storage/redis
- github.com/dgraph-io/badger/v4  (如果仅 mochi 使用)
```

## 14. 风险与注意事项

### 14.1 消息投递语义矩阵

| 消息 | 发布路径 | 业务订阅 | 集群分发 | 语义 |
|------|----------|----------|----------|------|
| Shadow update/get 请求 | 设备 MQTT QoS 1 -> gateway | Core queue subscription | `tio-shadow` 竞争消费 | broker 接收 QoS 1；handler at-most-once，DB 写入需幂等 |
| Shadow 响应/通知 | 内部 MQTT publisher QoS 1 | 设备 MQTT subscription | NATS MQTT session | 设备侧 QoS 取 publish/subscription 较小值 |
| Direct method 请求 | 内部 MQTT publisher QoS 1 | 设备 MQTT subscription | NATS MQTT session | gateway PUBACK，不代表设备业务处理成功；由 method response/timeout 判定 |
| Direct method 响应 | 设备 MQTT QoS 1 -> gateway | Core broadcast subscription | 每实例一份 | 用 client token 只完成本实例 waiting request |
| NTP 请求/响应 | MQTT -> queue / Core publish | `tio-ntp` 竞争消费 | 单实例处理 | QoS 0，可丢 |
| Presence 状态 | system event + reconciliation | KV + 广播 eventbus | 全实例 | 最终一致；reconciliation 最迟约 20 秒修正 |
| Presence MQTT topic | 内部 MQTT publisher retain | 设备 MQTT subscription | gateway retained store | 新订阅者收到最近状态；空 payload 删除 |

### 14.2 已知限制

1. **NATS MQTT gateway 能力边界**：只承诺当前 MQTT 3.1.1 用例。NATS native publish 到 MQTT 固定为 QoS 0，所以 QoS 1/retained 必须经过内部 MQTT publisher，禁止后来为了“简化”改回 `js.Publish`。

2. **Presence 最终一致**：system events 可能丢失，KV 不是活连接的唯一真相；必须保留周期 reconciliation 和相应指标。

3. **Retained cluster 窗口**：NATS MQTT retained 在集群传播存在短暂窗口。测试需要覆盖跨节点发布后立即订阅，并接受文档化的最终一致边界；业务内部连接状态以 Presence KV/reconciliation 为准。

4. **单实例退化**：routes 为空、replicas 为 1 时功能完整，但不具备节点故障后的会话/状态高可用。

5. **数据库选择**：多实例部署必须使用 MySQL；启动时检测到多节点 routes + sqlite 应直接失败。

6. **JobCenter**：本阶段禁用。配置或代码若尝试在多实例模式启动 JobCenter，应返回明确错误而不是继续运行。

7. **MQTT listener 数量**：单个 NATS 节点不能同时提供独立的明文 MQTT 和 TLS MQTT listener。部署必须选择一种模式，或用同一 cluster 的不同节点组分别暴露。

### 14.3 必须通过的验收测试

- 三个 tio/NATS 节点能组建同名 cluster，任一节点停止后其余节点继续服务
- MQTT 密码、provisioning、mTLS 均能连接；跨 thing publish/subscribe、`$SYS.>`、`$JS.>` 和控制 subject 均被拒绝
- Shadow/NTP 请求在三实例中只被一个 handler 处理
- Shadow/Direct Method 服务端发布实际以 MQTT QoS 1 到达 QoS 1 subscription；断连重连覆盖现有会话语义
- retained presence 可被稍后订阅者收到，空 payload 能删除 retained message
- 三个 `SubscribePresence` 消费者都收到同一个事件，不发生竞争消费
- 杀死设备所在 tio 进程后，reconciliation 在约定时间内将 presence 修正为 disconnected
- `Close` 能跨节点定位 owner 并断开正确 connection，不能误断重连后的新 connection
- 取消 handler/rule context 后，NATS subscriptions 数量回到基线
- `foo/#` 同时匹配 `foo` 和 `foo/bar`；所有非法 topic 被本地校验拒绝

## 15. 实施顺序

1. 锁定 NATS 版本，先实现 custom authentication + `RegisterUser`、MQTT QoS 1 和 retained 的最小集成测试
2. 简化 `connector/connector.go` 接口，加入 `QueueSubscribe` 和 `SubscribePresence(ctx)`
3. 实现 `connector/nats/` 包（server、JetStream bootstrap、connector、MQTT publisher、presence、auth、topic）
4. 更新消费者（shadow、method、ntp）适配新接口和 queue group
5. 实现 `rule/source/nats.go` 和 `rule/sink/nats.go`
6. 更新 `config/config.go` 和 `config.default.yaml`
7. 更新 `cmd/tio/main.go` 初始化流程，禁用 JobCenter
8. 更新 `auth/` 认证和 permissions 生成逻辑
9. 删除旧 MQTT broker/adapters；保留或迁移通用 eventbus 给 Presence 使用
10. 完成单机、三节点、节点故障、认证越权、QoS/retained 和订阅清理集成测试
11. 更新 docker-compose：三个独立持久卷、统一 clusterName、唯一 serverName 和 route mTLS

## 16. 参考资料

- [NATS MQTT：JetStream 要求、topic 转换、QoS 与 retained 限制](https://docs.nats.io/running-a-nats-service/configuration/mqtt)
- [NATS server `Authentication` / `ClientAuthentication` API](https://pkg.go.dev/github.com/nats-io/nats-server/v2/server#ClientAuthentication)
- [NATS System Events：CONNECT、DISCONNECT 与 monitoring services](https://docs.nats.io/running-a-nats-service/configuration/sys_accounts)
- [NATS JetStream clustering](https://docs.nats.io/running-a-nats-service/configuration/clustering/jetstream_clustering)
