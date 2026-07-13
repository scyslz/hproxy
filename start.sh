#!/bin/sh
# hproxy v2 启动脚本

# 获取脚本所在目录的绝对路径
HP_DIR="$(cd "$(dirname "$0")" && pwd)"
CONFIG_FILE="$HP_DIR/config.json"
PROXY_BIN="$HP_DIR/hproxy"


echo "$HP_DIR"
# 停止旧进程
killall hproxy 2>/dev/null
sleep 1

# 检查二进制
if [ ! -f "$PROXY_BIN" ]; then
    echo "错误: $PROXY_BIN 不存在"
    exit 1
fi

# 检查配置
if [ ! -f "$CONFIG_FILE" ]; then
    echo "错误: $CONFIG_FILE 不存在"
    exit 1
fi

# 启动
cd "$HP_DIR"

$PROXY_BIN "$CONFIG_FILE" > "$HP_DIR/hproxy.log" 2>&1 &
echo $! > "$HP_DIR/hproxy.pid"

echo "hproxy v2 已启动, PID: $(cat $HP_DIR/hproxy.pid)"
echo "日志: $HP_DIR/hproxy.log"
