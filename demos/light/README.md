# Light Demo 代码

本示例展示了设备端和服务端集成 tio 的示例代码

示例场景和功能： 

- 路灯的开关和亮度控制
- 日出日落时间配置
- 实时让灯闪速几下
- 灯的状态定期上报，可用于服务端存储记录以监控分析或告警

分别展示了 tio 的几种常规用法

- 通过 shadow 控制设备
- 通过 shadow 配置设备
- 通过设备直接方法控制设备（或获取最新状态）
- 设备定期上报相关指标数据

Light Demo 启动方法  

1. 启动 tio `go run cmd/tio/main.go`（使用默认配置， 若修改了配置，请修改 demo 代码中响应的配置项）
2. 启动 demo server `go run demos/light/server/main.go`
3. 启动 demo device `go run demos/light/device/main.go`

端到端测试只需要启动 Tio，不需要另外启动 demo server 和 demo device。Tio 必须使用
`admin/public` API 认证、监听 API 端口 `9000` 和明文 MQTT 端口 `1883`，并使用
`legacy/JSON` 设备协议。测试不会随 `go test ./...` 自动运行；服务启动后，在仓库根目录执行：

```bash
make test-demo-light
```

Makefile 会先检查 API 和 MQTT 端口；条件不满足时打印启动提示并退出。
