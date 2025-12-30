#!/bin/bash

# 配置信息
USER="admin"
PASS="public"
HOST="localhost:9000"
QUEUE="test_queue"
CLIENT_ID="shell-pull-client"

# 生成 Basic Auth 认证信息 (注意：这里只需要 base64 字符串，websocat 会自动添加 Header)
# AUTH=$(echo -n "$USER:$PASS" | base64 | tr -d '[:space:]')
AUTH="$USER:$PASS"

URL="ws://$HOST/api/v1/mqttBroker/embed/queue/pull?queue=$QUEUE&clientId=$CLIENT_ID"

echo "### Connecting to queue: $QUEUE (Pull Mode via Shell) ###"
echo "URL: $URL"

# 检查是否安装了 websocat
if ! command -v websocat &> /dev/null; then
    echo "错误: 未安装 websocat。请先安装它（例如: brew install websocat）。"
    exit 1
fi

# 定期（每1秒）发送 pull 请求的循环
# 使用 - 显式指定标准输入作为其中一个地址，解决 No URL specified 报错
{
    # 初始拉取
    echo '{"action":"pull","count":5}'
    while true; do
        echo '{"action":"pull","count":5}'
    done
} | websocat --text - "$URL" --basic-auth "$AUTH"
