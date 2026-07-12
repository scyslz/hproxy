#!/bin/sh
# 快速压测 - 50 个并发请求

HPID=$(pidof hproxy)
echo "=== 快速压测 ==="
echo ""
echo "初始 FD: $(ls /proc/$HPID/fd 2>/dev/null | wc -l)"
echo ""

echo "发送 50 个并发请求..."
for i in $(seq 1 50); do
  curl -sk "https://127.0.0.1:443" -o /dev/null -w '' 2>/dev/null &
done
wait
echo "完成"
echo ""

echo "请求后 FD: $(ls /proc/$HPID/fd 2>/dev/null | wc -l)"
echo ""
echo "等待 10s..."
sleep 10
echo "10s 后 FD: $(ls /proc/$HPID/fd 2>/dev/null | wc -l)"
echo ""
echo "检查错误日志:"
tail -20 /root/hproxy.log | grep -E 'too many|error' | wc -l
