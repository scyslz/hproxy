#!/bin/sh
# hproxy 启动脚本 - OpenWrt

# 停止现有进程
killall hproxy 2>/dev/null
sleep 1

# 调大文件描述符限制（OpenWrt 默认只有 1024）
ulimit -n 65536

# 启动 hproxy
cd /root
./hproxy -config=/root/config.json > /root/hproxy.log 2>&1 &

echo "hproxy 已启动 (PID: $!)"
echo "FD 限制: $(ulimit -n)"
