#!/bin/sh
# OpenWrt TCP 参数优化 - 加速 socket 释放

echo "=== TCP 参数优化 ==="

# 1. 启用 TIME_WAIT 快速回收和重用
echo "1. 优化 TIME_WAIT..."
echo 1 > /proc/sys/net/ipv4/tcp_tw_reuse
echo 30 > /proc/sys/net/ipv4/tcp_fin_timeout
echo "   tcp_tw_reuse: $(cat /proc/sys/net/ipv4/tcp_tw_reuse)"
echo "   tcp_fin_timeout: $(cat /proc/sys/net/ipv4/tcp_fin_timeout)"

# 2. 增加连接队列
echo "2. 增加连接队列..."
echo 65536 > /proc/sys/net/core/somaxconn
echo 65536 > /proc/sys/net/core/netdev_max_backlog
echo "   somaxconn: $(cat /proc/sys/net/core/somaxconn)"
echo "   netdev_max_backlog: $(cat /proc/sys/net/core/netdev_max_backlog)"

# 3. 优化 TCP keepalive
echo "3. 优化 TCP keepalive..."
echo 600 > /proc/sys/net/ipv4/tcp_keepalive_time
echo 30 > /proc/sys/net/ipv4/tcp_keepalive_intvl
echo 3 > /proc/sys/net/ipv4/tcp_keepalive_probes
echo "   tcp_keepalive_time: $(cat /proc/sys/net/ipv4/tcp_keepalive_time)"

# 4. 减少 TCP 重试次数
echo "4. 减少重试次数..."
echo 3 > /proc/sys/net/ipv4/tcp_syn_retries
echo 3 > /proc/sys/net/ipv4/tcp_synack_retries
echo "   tcp_syn_retries: $(cat /proc/sys/net/ipv4/tcp_syn_retries)"

# 5. 启用 TCP 快速打开 (如果支持)
echo "5. 启用 TCP 快速打开..."
echo 3 > /proc/sys/net/ipv4/tcp_fastopen 2>/dev/null || echo "   (不支持)"

# 6. 增加系统 FD 限制
echo "6. 系统 FD 限制..."
ulimit -n 65536 2>/dev/null
echo "   ulimit -n: $(ulimit -n)"

echo ""
echo "=== 优化完成 ==="
echo "查看当前 TCP 状态统计:"
cat /proc/net/sockstat | head -10
