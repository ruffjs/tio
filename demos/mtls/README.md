# mTLS Demo - MQTT 双向证书认证

本示例展示了如何在 tio 中使用双向 TLS 证书认证（mTLS）接入 MQTT broker，并和现有用户名密码设备并存。

## 场景说明

- 客户端使用证书进行身份认证
- 服务端验证客户端证书（双向验证）
- **使用证书的 CN (Common Name) 作为 thingId**
- **证书设备必须预先创建为 `authType=certificate`**
- 其他设备仍可继续使用用户名密码接入

## 文件结构

```
demos/mtls/
├── certs/
│   ├── generate-certs.sh    # 证书生成脚本
│   ├── ca.pem               # CA 根证书
│   ├── ca-key.pem           # CA 私钥
│   ├── server-cert.pem      # 服务端证书
│   ├── server-key.pem       # 服务端私钥
│   ├── client-cert.pem      # 客户端证书
│   └── client-key.pem       # 客户端私钥
├── device/
│   └── main.go              # mTLS 客户端示例代码
└── README.md
```

## 快速开始

### 1. 生成测试证书

如果 `certs/` 目录下还没有证书，先运行证书生成脚本：

```bash
cd demos/mtls/certs
./generate-certs.sh
```

生成的证书包括：
- `ca.pem` / `ca-key.pem`: CA 根证书和私钥
- `server-cert.pem` / `server-key.pem`: 服务端证书和私钥
- `client-cert.pem` / `client-key.pem`: 客户端证书（CN=mtls-client）

### 2. 配置 tio 服务器

tio 使用 NATS 作为 MQTT 网关。NATS 的 MQTT 端口只有一个，配置 TLS 后该端口即使用 TLS。

**方式一：创建单独的 mTLS 配置文件**（推荐，不影响普通 MQTT 连接）

创建 `config.mtls.yaml`：

```yaml
api:
  port: 9000
  basicAuth:
    name: admin
    password: public

db:
  type: sqlite
  sqlite:
    filePath: tio.sqlite

connector:
  type: nats
  nats:
    server:
      serverName: tio-node-1
      clusterName: tio
      port: 4222
      mqttPort: 8883              # MQTT over TLS 端口
      wsPort: 8083
      monitorPort: 8222
      storeDir: ./data/nats/tio-node-1
      mqttStreamReplicas: 1
      presenceReplicas: 1
      # MQTT TLS 配置（双向认证）
      mqttTls:
        certFile: "./demos/mtls/certs/server-cert.pem"
        keyFile: "./demos/mtls/certs/server-key.pem"
        caFile: "./demos/mtls/certs/ca.pem"
        requireClientCert: true
    appClient:
      user: $tio-app
      password: public
    systemClient:
      user: $tio-sys
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

**方式二：修改现有 config.yaml**

在现有 `config.yaml` 的 `connector.nats.server` 下添加 `mqttTls` 配置，并将 `mqttPort` 改为 `8883`：

```yaml
connector:
  type: nats
  nats:
    server:
      # ... 其他配置 ...
      mqttPort: 8883              # 改为 TLS 端口
      mqttTls:
        certFile: "./demos/mtls/certs/server-cert.pem"
        keyFile: "./demos/mtls/certs/server-key.pem"
        caFile: "./demos/mtls/certs/ca.pem"
        requireClientCert: true
```

> **注意**：启用 mTLS 后，所有 MQTT 连接都需要使用 TLS 和客户端证书。如果需要同时支持普通密码连接和证书连接，请使用方式一并运行两个 tio 实例。

### 3. 创建证书设备

先查看客户端证书的 `CN`，这个值必须和 tio 中的 `thingId` 一致：

```yaml
openssl x509 -in demos/mtls/certs/client-cert.pem -noout -subject
```

默认输出中的 `CN=mtls-client` 就是设备 `thingId`。

然后创建证书设备：

```bash
curl -X POST http://localhost:9000/api/v1/things \
  -u admin:public \
  -H "Content-Type: application/json" \
  -d '{"thingId":"mtls-client","authType":"certificate"}'
```

也可以继续创建普通密码设备，两类设备可以同时存在：

```bash
curl -X POST http://localhost:9000/api/v1/things \
  -u admin:public \
  -H "Content-Type: application/json" \
  -d '{"thingId":"plain-device","password":"public","authType":"password"}'
```

### 4. 启动 tio 服务

> **注意**：tio 目前只支持读取 `config.yaml`，不支持命令行指定配置文件。如需测试 mTLS，请备份原 config.yaml 后替换为 mTLS 配置。

```bash
# 备份原配置（可选）
cp config.yaml config.yaml.bak

# 使用 mTLS 配置
cp config.mtls.yaml config.yaml

# 启动 tio
go run cmd/tio/main.go
```

### 5. 启动 mTLS 客户端

```bash
cd demos/mtls/device
go run main.go
```

客户端会自动从证书的 CN 中提取 `thingId`，并使用该证书与 `tio` 进行双向验证。

## 验证 mTLS 连接

成功连接后，客户端日志会显示：

```
INFO Extracted thingId from client certificate CN thingId=mtls-client
INFO Successfully connected to MQTT broker using mutual TLS clientId=mtls-client
```

服务端日志会显示：

```
INFO Mqtt client authorized via certificate thingId=mtls-client clientId=mtls-client
```

## 使用 OpenSSL 验证连接

你也可以使用 OpenSSL 命令行工具测试 mTLS 连接：

```bash
# 使用客户端证书连接到 MQTT broker
openssl s_client -connect localhost:8883 \
  -CAfile certs/ca.pem \
  -cert certs/client-cert.pem \
  -key certs/client-key.pem
```

## 查看证书信息

查看客户端证书的 CN：

```bash
openssl x509 -in certs/client-cert.pem -noout -subject
# 输出：subject=CN=mtls-client,...
```

## 自定义 thingId

如需使用不同的 `thingId`，重新生成证书时指定不同的 `CLIENT_CN`：

```bash
cd demos/mtls/certs
CLIENT_CN=demo-device-01 ./generate-certs.sh
```

然后创建同名证书设备：

```bash
curl -X POST http://localhost:9000/api/v1/things \
  -u admin:public \
  -H "Content-Type: application/json" \
  -d '{"thingId":"demo-device-01","authType":"certificate"}'
```

设备端如果服务地址或证书路径不同，也可以用环境变量覆盖：

```bash
TIO_MTLS_HOST=127.0.0.1 \
TIO_MTLS_PORT=8883 \
TIO_MTLS_SERVER_NAME=localhost \
TIO_MTLS_CA_FILE=../certs/ca.pem \
TIO_MTLS_CLIENT_CERT_FILE=../certs/client-cert.pem \
TIO_MTLS_CLIENT_KEY_FILE=../certs/client-key.pem \
go run demos/mtls/device/main.go
```

## 证书说明

### CA 证书
- 用于签发服务端和客户端证书
- 服务端用 CA 验证客户端证书
- 客户端用 CA 验证服务端证书

### 服务端证书
- CN=localhost
- SAN: DNS:localhost, IP:127.0.0.1
- 由 CA 签名

### 客户端证书
- CN=mtls-client（作为 thingId 使用）
- 由 CA 签名
- 用于客户端身份认证

## mTLS 认证流程

1. 客户端发起 TLS 连接，出示客户端证书
2. 服务端验证客户端证书（通过 CA 证书）
3. 服务端从证书中提取 CN 作为 thingId
4. 服务端查询 `thingId` 对应设备，且要求 `authType=certificate`
5. 认证通过，允许连接（无需密码）

## 生产环境建议

1. **证书管理**: 生产环境应使用正式的 CA 签发证书
2. **证书轮换**: 定期更新证书，设置合理的有效期
3. **私钥保护**: 确保私钥文件权限设置为 600
4. **证书吊销**: 实现 CRL 或 OCSP 进行证书吊销检查
5. **最小权限**: 为不同客户端颁发不同权限的证书
6. **CN 规范**: 制定统一的 CN 命名规范，便于设备管理
7. **设备预注册**: 生产环境建议显式预注册证书设备，并保持 `thingId == 证书 CN`

## 清理证书

如需重新生成证书，可删除现有证书后重新运行脚本：

```bash
cd demos/mtls/certs
rm -f *.pem *.srl
./generate-certs.sh
```
