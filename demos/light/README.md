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
