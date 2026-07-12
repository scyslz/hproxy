#!/bin/sh
# hproxy v2 启动脚本

# 配置路径
CONFIG_FILE="/root/hp/v2/config.json"
HP_DIR="/root/hp/v2"
PROXY_BIN="$HP_DIR/hproxy-v2-arm64"

# 确保目录存在
mkdir -p "$HP_DIR"

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
