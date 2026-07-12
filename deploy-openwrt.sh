#!/bin/sh
# OpenWrt 上部署 hproxy 的完整脚本

echo "=== hproxy 部署脚本 ==="

# 1. 停止现有的 hproxy
killall hproxy 2>/dev/null
sleep 1

# 2. 调大系统限制
echo "调整系统参数..."
ulimit -n 65536
echo "  ulimit -n: $(ulimit -n)"

# 调大 TCP backlog
echo 65536 > /proc/sys/net/core/somaxconn 2>/dev/null
echo 65536 > /proc/sys/net/core/netdev_max_backlog 2>/dev/null
echo "  somaxconn: $(cat /proc/sys/net/core/somaxconn)"

# 3. 确保端口没有被占用
echo "检查端口占用..."
netstat -tlnp | grep -E ':80|:443' | grep -v hproxy && {
    echo "警告: 80/443 端口被其他进程占用"
    echo "建议停止 nginx 或其他 Web 服务器"
}

# 4. 启动 hproxy
echo "启动 hproxy..."
cd /root
./hproxy -config=/root/config.json > /root/hproxy.log 2>&1 &

sleep 2

# 5. 验证
if pgrep -f hproxy > /dev/null; then
    echo "✅ hproxy 启动成功"
    echo "  PID: $(pgrep -f hproxy)"
    echo "  日志: tail -f /root/hproxy.log"
    echo ""
    echo "查看 FD 限制:"
    tail -5 /root/hproxy.log | grep "FD 限制"
else
    echo "❌ hproxy 启动失败"
    echo "查看日志:"
    cat /root/hproxy.log | tail -20
fi
