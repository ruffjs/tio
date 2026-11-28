# tio

![build][build]
[![license][license]](LICENSE)

[中文](README.md) | [English](README_en.md)

`tio` is a tiny iothub core.
>Why this name? A shourt name is better cause the name will used in multiple places, eg: http api path, domain name, config file. `t` represent **tiny**, `io` represent its duty is to provide a communication channel between things and software.

## Main Features

- Lightweight: You can combine different databases and message middleware as needed. No need to pay for unused features
- Simple: Focus on and be a core function of iothub core. And provide a web ui, which facilitates debugging and familiarization with Tio interfaces
- Useful: Simplify the interaction process and implementation with the device. Especially, the interaction between the server and devices is simplified through the abstraction of device `shadows`
- Production-ready: tio has been used in multiple projects and products in production environments

## Main Components

- Thing: The basic management for thing, eg: CRUD, authz
- Connector: Device connectivity layer based on the embedded comqtt MQTT broker, with MQTT/TLS/mTLS/WS support; NATS is an optional egress
- Shadow：Like [AWS IoT Shadow](https://docs.aws.amazon.com/iot/latest/developerguide/device-shadow-document.html)、[Azure Device Twin](https://learn.microsoft.com/zh-cn/azure/iot-hub/iot-hub-devguide-device-twins)、[Aliyun Device Shadow](https://www.alibabacloud.com/help/en/iot-platform/latest/78e011). Major public cloud vendors all have an abstraction of device shadows (with different names), and their connotations are highly consistent. In practical project development, they are indeed very useful tools, greatly reducing the complexity and cognitive burden of interactions between upper-layer business systems and devices.
- Direct Method: The server uses a "request-response" mode for calling methods on the device, similar to an HTTP request. This implementation is based on [Azure Direct method](https://learn.microsoft.com/en-us/azure/iot-hub/iot-hub-devguide-direct-methods).
- Job: Job management. By sending tasks to devices in bulk and scheduling them, and managing the status and lifecycle of tasks, task operations can include direct-method invocation, Shadow updates, custom operations, etc. On one hand, it strengthens the functionality and usability of direct-method invocation and Shadow updates by allowing batched, scheduled, and asynchronous execution of these operations, with operation records (i.e., Job and Task records). On the other hand, custom Jobs provide a general mechanism for executing various types of remote operations on devices conveniently.


```
          App on Device                                     Back end
                          ┌───────────────────────┐
                          │        Shadow         │
                          │  ┌─────────────────┐  │
                          │  │      Tags       ├──┼─────── Read,write
                          │  └─────────────────┘  │
                          │  ┌─────────────────┐  │
                          │  │     States      │  │
                          │  │   ┌──────────┐  │  │
        Read,receive ─────┼──┼───┤ Desired  ├──┼──┼─────── Read,write
change notifications      │  │   └──────────┘  │  │        change notifications
                          │  │   ┌──────────┐  │  │
          Read,write ─────┼──┼───┤ Reported ├──┼──┼─────── Read
                          │  │   └──────────┘  │  │        change notifications
                          │  └─────────────────┘  │
                          └───────────────────────┘
                          
```

Shadow Query:

The Shadow Query interface adopts a SQL-like approach for querying, coupled with the flexibility and extensibility of Shadow attributes, providing a great deal of freedom to the upper layer of usage. This allows for a highly adaptable way to retrieve Shadow data based on desired views and query conditions. Refer to [Azure](https://learn.microsoft.com/en-us/azure/iot-hub/iot-hub-devguide-query-language) for more information.


## Connector

Built on the embedded [comqtt](https://github.com/wind-c/comqtt) MQTT broker — no external message broker required.

- MQTT v3.1.1 over TCP, TLS/mTLS and WS/WSS
- Password, client-certificate, provision and gateway-binding authentication
- Single-node ingress; NATS is an optional egress bridge, not a device entry point

## Optional NATS bridge

NATS does not take part in device ingress, authentication, presence or sessions. With `nats.enabled: false` (the default) tio runs on the embedded comqtt broker alone and every MQTT feature works.

When enabled, the bridge forwards only whitelisted device uplinks plus one presence message (`tio.{thingId}.presence`). Shadow, Method, NTP and Job control-plane messages never appear in NATS. With `nats.jetStream.enabled: false` messages are fire-and-forget; when enabled they are retained according to `maxAgeSeconds`/`maxMsgs`/`maxBytes` and can be replayed by external consumers. MQTT retained state always lives in the comqtt `storage`, independent of JetStream.

Configure it with a top-level `nats:` block in `config.yaml`, or point `nats.configFile` at a separate file (see [config-nats.default.yaml](config-nats.default.yaml) for every option):

```yaml
nats:
  enabled: true
  mode: embedded            # embedded | external
  url: ""                   # external mode
  configFile: ./nats.yaml
```

`embedded` starts an in-process NATS server (`nats.server` holds port, cluster and TLS settings); `external` connects to `url` with the `appClient` credentials. `appClient`/`systemClient` are internal tio accounts, unrelated to device authentication.

Migrating from the old layout: `connector.type`, `connector.nats` and `connector.natsConfigFile` are gone — NATS is the top-level `nats` block; `nats.server.mqttPort`/`mqttTls` become `connector.mqttBroker` port/TLS fields; `nats.superUsers` moves to `connector.mqttBroker.superUsers`; `nats.mqttPublisher` is removed. Old NATS MQTT sessions/retained data and JetStream streams are not compatible with the new bridge; use a new or cleaned `jetStream.storeDir`.

## Supported DB

- MySQL: For production environments
- sqlite3: For testing, development or light use scenarios. sqlite3 even supports memory mode when the config is `":memory:"` — check this in `config.yaml`

SQLite uses a single connection, so all reads and writes are serialized: as devices or write rates grow, Shadow, Job and Presence contend for it. Use MySQL in production; SQLite is for development and small deployments.

## Run

- Check if the configuration in `config.yaml` file is what you want.
- `go run cmd/tio/main.go`
- vist `http://127.0.0.1:9000/` for web admin and debug tools
- vist `http://127.0.0.1:9000/docs` for api docs

### Production deployment checklist

The default configuration is for local development. Before going live:

- Replace every default credential: API `basicAuth`, MySQL, `connector.mqttBroker.superUsers`, and `nats.appClient`/`nats.systemClient` when NATS is enabled (see `config-nats.default.yaml`). NATS internal accounts fail startup when their password is empty.
- NATS credentials must be set to non-empty values explicitly. Omitting a field keeps the embedded default (`public`) instead of clearing it; use `superUsers: []` when no super user is needed.
- Turn debugging off: `log.level: info`, `showSql: false` for SQLite/MySQL, `pprof.enabled: false`.
- Do not expose `/docs`, `/metrics` or pprof to the public internet.
- Request bodies are capped at 1 MiB (8 MiB for batch import and rule config); direct-method `data` also counts against the 1 MiB limit.

**Upgrade note**: `auth.allowPasswordBootstrap` defaults to `false`. Deployments that relied on a thing with an empty `authValue` storing the first password it presented will reject those devices after upgrading. Set `authValue` in bulk before upgrading, or temporarily set `auth.allowPasswordBootstrap: true` for one onboarding round and turn it back off.

## Build

```bash
cd web && yarn && yarn build

# build
# CGO_ENABLED=1 is for sqlite3, if you don't use sqlite, you can remove this parameter.

CGO_ENABLED=1 go build -o tio cmd/tio/main.go

# run

./tio

```

Build docker image

```bash
bash build/docker/build.sh
```

Build deb package for Debian-Based Linux Distributions

```bash
# deb package in ./dist directory
bash build/deb/build.sh
```

## Develop

### enable git hooks

```bash
chmod +x ./githooks/*
git config core.hooksPath githooks
```

### code directory structure

```bash
.
├── api           # configuration for api and swagger, etc.
├── auth          # device authentication
├── shadow        # core of Tio, including definition and implementation of shadow and direct methods (part of message communication in connector)
├── thing         # basic CRUD for Thing
├── job           # job management. Job operations include direct-method invocation, shadow updates, custom operations, etc.
├── ntp           # device NTP service
├── connector     # device connectivity (comqtt MQTT broker)
├── bridge        # optional NATS egress
├── cmd           # main entry code
│   └── tio
└── web           # debugging management background
├── config        # program configurations
├── db            # DB configurations
│   ├── mysql
│   └── sqlite
├── build         # building scripts and configurations
│   ├── deb       # building deb packages for Debian-based systems
│   └── docker
├── githooks      # githooks for code formatting and submission
└──pkg           # business-independent libraries
```

### tech stack

golang + sqlite/mysql + comqtt (optional NATS)

web：vue3 + element-plus

## License

[MIT](LICENSE)

[build]: https://github.com/ruffjs/tio/actions/workflows/release.yml/badge.svg
[license]: https://img.shields.io/badge/license-MIT-blue.svg
