# tio

[中文](README.md) | [English](README_en.md)

`tio` 是一个轻量的 iothub 基础实现。
  
>为什么选择这个名字？ 简短：一个简短的名字更容易记，并且可能在多个地方使用，例如：HTTP API 路径、域名、配置文件等。`t` 代表 **tiny**（微小），`io` 代表其职责是在物与软件之间提供通信渠道


## 主要特点

- 轻量：部署最简可以只有一个 golang 编译出的二进制程序（特别适合于开发、测试和设备数量不多的场景）；当然也可以根据需要使用不同的数据库和消息中间件
- 简单：专注于 iothub 的核心功能，不求大而全，保持简单稳定。并提供了 web 调试管理后台，便于调试和对 tio 接口的熟悉
- 实用：简化与物联网设备的交互过程和实现。特别是通过`设备影子`（Shadow）的抽象简化了服务端和设备的交互
- 生产可用：tio 已在生产环境多个项目和产品中使用

## 主要组件

- Thing：用于设备的基本管理，例如：CRUD、授权认证
- Connector：适配各种消息中间件，特别是 MQTT broker
- Shadow：设备影子，类似于 [AWS IoT Shadow](https://docs.aws.amazon.com/iot/latest/developerguide/device-shadow-document.html)、[Azure Device Twin](https://learn.microsoft.com/zh-cn/azure/iot-hub/iot-hub-devguide-device-twins)、[阿里云设备影子](https://help.aliyun.com/document_detail/53930.html)，各大公有云厂商都有设备影子（名称各有不同）的抽象，且其内涵都高度一致，在外面实际的项目开发中确实是非常有用的工具，极大地减少上层业务系统和设备交互的复杂度和心智负担。 
- 设备直接方法：服务端对设备的方法调用采用“请求-响应”模式，类似于 HTTP 请求。参考了 [Azure Direct method](https://learn.microsoft.com/zh-cn/azure/iot-hub/iot-hub-devguide-direct-methods) 的实现


```
            Thing app                                     Back end
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


## 支持的 connector

### 嵌入式 MQTT Broker

运行 tio 时，默认自动运行一个嵌入式 MQTT broker
这对于测试、开发和少量设备场景非常有用

### EMQX MQTT Broker

[EMQX](https://github.com/emqx/emqx)  是一个易于使用的优秀 MQTT broker
tio 集成了其 `v5` 版本，以提供更强的功能性和性能（水平扩展）

## 支持的数据库

- MySQL：用于生产环境
- sqlite3：用于测试、开发或轻量级使用场景
  当配置为 `":memory:"` 时，sqlite3 甚至支持内存模式，请在 `config.yaml` 中进行相应配置

## 运行

- 检查 `config.yaml` 文件中的配置是否符合您的需求
- 运行 `go run cmd/tio/main.go`
- 访问 `http://127.0.0.1:9000/` 打开调试管理后台
- 访问 `http://127.0.0.1:9000/docs` 查看 API 文档

## 构建

```bash
# 构建 web 后台
cd web && yarn && yarn build

# 构建 go 主程序
# CGO_ENABLED=1 用于 sqlite3，如果您不使用 sqlite，可以删除此参数。

CGO_ENABLED=1 go build -o tio cmd/tio/main.go

# 运行

./tio

```

构建 Docker 镜像

```bash
bash build/docker/build.sh
```

构建适用于基于 Debian 的 Linux 发行版的 deb 软件包

```bash
# deb 软件包在 ./dist 目录下
bash build/deb/build.sh
```

## 开发

### 启用 Git 钩子

```bash
chmod +x ./githooks/*
git config core.hooksPath githooks
```

### 代码目录结构说明

```bash
.
├── api           # api 配置和 swagger 配置等
├── auth          # 设备认证
├── shadow        # tio 的核心，含 shadow、direct method 的定义和实现（涉及到消息通信的部分在 connector 中)
├── thing         # thing 基本的 CRUD
├── ntp           # 设备 ntp 服务
├── connector     # connector 实现
│   └── mqtt
│       ├── embed # 内置的 MQTT Broker
│       └── emqx  # 集成 EMQX MQTT Broker
├── cmd           # main 入口代码
│   └── tio
├── web           # 调试管理后台
├── config        # 程序配置
├── db            # db 配置
│   ├── mysql
│   └── sqlite
├── build         # 构建脚本和配置
│   ├── deb       # debian 类系统中用到的 deb 包构建
│   └── docker
├── githooks      # 代码规范和提交相关的 githooks
└── pkg           # 业务无关的一些库
```

### 技术栈

golang + sqlite/mysql +  内置MQTT服务/emqx

前端（调试管理后台）：vue3 + element-plus
