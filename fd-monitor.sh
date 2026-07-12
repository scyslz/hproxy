#!/bin/sh
# hproxy FD 监控和自动重启脚本

THRESHOLD=3000  # FD 数量阈值
CHECK_INTERVAL=10  # 检查间隔（秒）

echo "=== hproxy FD 监控 (阈值: $THRESHOLD) ==="

while true; do
    HPID=$(pidof hproxy)
    
    if [ -z "$HPID" ]; then
        echo "$(date '+%Y-%m-%d %H:%M:%S') hproxy 未运行，尝试启动..."
        ulimit -n 4096
        /root/hproxy -config=/root/config.json > /root/hproxy.log 2>&1 &
        sleep 5
        continue
    fi
    
    FD_COUNT=$(ls /proc/$HPID/fd 2>/dev/null | wc -l)
    
    if [ "$FD_COUNT" -gt "$THRESHOLD" ]; then
        echo "$(date '+%Y-%m-%d %H:%M:%S') ⚠️  FD 数量异常: $FD_COUNT > $THRESHOLD，重启 hproxy..."
        killall hproxy 2>/dev/null
        sleep 2
        ulimit -n 4096
        /root/hproxy -config=/root/config.json > /root/hproxy.log 2>&1 &
        sleep 5
        echo "$(date '+%Y-%m-%d %H:%M:%S') hproxy 已重启"
    else
        echo "$(date '+%Y-%m-%d %H:%M:%S') OK (FD: $FD_COUNT)"
    fi
    
    sleep $CHECK_INTERVAL
done
