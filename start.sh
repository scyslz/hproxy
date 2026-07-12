#!/bin/bash
# start.sh — 启动/重启 lucky2smartdns
# 用法: ./start.sh [config.json]

set -e

APP_NAME="lucky2smartdns"
APP_PATH="/root/${APP_NAME}"
CONFIG="${1:-/root/config.json}"
LOG_FILE="/tmp/lucky2smartdns.log"
PID=$(pidof $APP_NAME 2>/dev/null || true)

# 1. 停止旧进程
if [ -n "$PID" ]; then
    echo "[Stop] 停止旧进程 (PID: $PID)..."
    kill $PID 2>/dev/null
    sleep 1
    # 确认已停止
    if pidof $APP_NAME >/dev/null 2>&1; then
        echo "[Stop] 强制停止..."
        kill -9 $PID 2>/dev/null
        sleep 1
    fi
    echo "[Stop] 旧进程已停止"
else
    echo "[Stop] 无旧进程"
fi

# 2. 检查文件
if [ ! -f "$APP_PATH" ]; then
    echo "[Error] $APP_PATH 不存在"
    exit 1
fi
if [ ! -f "$CONFIG" ]; then
    echo "[Error] $CONFIG 不存在"
    exit 1
fi
if [ ! -x "$APP_PATH" ]; then
    chmod +x "$APP_PATH"
fi

# 3. 启动
echo "[Start] 启动 $APP_NAME -config=$CONFIG"
nohup $APP_PATH -config=$CONFIG >$LOG_FILE 2>&1 &
sleep 2

# 4. 验证
if pidof $APP_NAME >/dev/null 2>&1; then
    echo "[OK] 启动成功 (PID: $(pidof $APP_NAME))"
    echo "[Log] 最近日志："
    tail -10 $LOG_FILE
else
    echo "[Error] 启动失败，查看日志："
    cat $LOG_FILE | tail -20
    exit 1
fi
