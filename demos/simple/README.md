# Simple 设备 Topic 交互测试

从仓库运行（需要 Go，依赖复用项目的 `go.mod`）：

```sh
./demos/simple/test-device.sh test
```

参数为 thingId，省略时使用 `test`。脚本从仓库根目录的 `tio.sqlite` 只读获取密码，自动读取服务器模式和设备编码，支持已启用、使用密码认证的 Simple JSON/CBOR 设备。Legacy 部署或 Dual 中的 Legacy 设备会在测试前退出。

默认连接本机 MQTT `1883` 和 HTTP `9000`，HTTP 账号为 `admin`，后端观察账号为 `$biz`，两者默认密码均为 `public`。定制时通过环境变量设置，密码不会写入脚本或输出：

| 环境变量 | 默认值 / 用途 |
|---|---|
| `TIO_THING_ID` | `test`；命令行 thingId 优先 |
| `TIO_SQLITE` | `tio.sqlite`；相对路径以仓库根目录为基准 |
| `TIO_DEVICE_PASSWORD` | 可选，覆盖数据库中的设备密码，用于其他部署 |
| `TIO_MQTT_URL` | `tcp://127.0.0.1:1883` |
| `TIO_API_URL` | `http://127.0.0.1:9000` |
| `TIO_API_USER` / `TIO_API_PASSWORD` | `admin` / `public` |
| `TIO_SUPER_USER` / `TIO_SUPER_PASSWORD` | `$biz` / `public` |

脚本在一台设备和一个 superuser 后端客户端之间验证：

- 允许订阅 `tio/{id}/down/#` 和自定义 Topic；拒绝整树、`up/#` 和其他设备的订阅，检查实际 SUBACK。
- 后端收到 Presence 在线通知。
- `ntp_req` → `ntp_resp`，核对时间戳。
- `shadow_get` → `shadow_get_reply`。
- `shadow_update` → `shadow_update_reply`，并通过 HTTP 核对持久化状态；检查旧版本 `409` 和无效状态 `400`。
- 服务端 HTTP 设置多层 desired 基线，再只修改最深的 `control.target.value` → `shadow_desired` → 设备上报并核对 desired/reported 收敛。
- 服务端 HTTP `/invoke` → `method_req` → 设备 `method_resp` → HTTP 返回结果，核对请求 ID 和数据。
- Data/Event 原始二进制透传，以及自定义 Topic 双向收发。

嵌套测试结构如下；每次运行使用唯一的一级字段名：

```json
{
  "__tio_probe_<runId>": {
    "control": {"target": {"value": "reported", "unit": "C"}, "enabled": true},
    "label": "<runId>"
  }
}
```

先设置完整 desired 基线并接收属性通知，再只修改最深的 `control.target.value`，断言 `shadow_desired` 的 `name` 为测试字段名、`value` 为完整一级属性值，并保留 `unit`、`enabled` 和 `label`。每条消息携带本次 Shadow 版本；设备按属性处理通知。

每项成功输出 `PASS`，并等待 100ms 后继续下一项；失败返回非零退出码。脚本使用独立 Client ID 和唯一的 `__tio_probe_*` Shadow 字段，结束时删除该字段并验证清理结果。测试会增加 Shadow 版本并产生连接、事件和日志记录；它不会恢复版本号。使用专用测试设备，并暂停该设备的其他测试程序，以免并发请求或真实设备处理测试命令。

正常结束、测试失败或 Ctrl+C 都会尝试清理。若服务器断开或进程被强制终止，按错误中给出的字段名检查 desired/reported。
