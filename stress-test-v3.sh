#!/bin/sh
# 压测脚本 - 兼容 OpenWrt ash shell

echo "=== hproxy 压力测试 ==="
echo ""

HPID=$(pidof hproxy)
echo "hproxy PID: $HPID"
echo ""

# 初始状态
echo "1. 初始状态:"
FD1=$(ls /proc/$HPID/fd 2>/dev/null | wc -l)
echo "   FD: $FD1"
echo ""

# 轻度测试：100 个串行请求
echo "2. 轻度测试: 100 个请求..."
i=1
while [ $i -le 100 ]; do
  curl -sk "https://127.0.0.1:443" -o /dev/null -w '' 2>/dev/null
  i=$((i + 1))
done
echo ""
FD2=$(ls /proc/$HPID/fd 2>/dev/null | wc -l)
echo "   请求后 FD: $FD2 (变化: $((FD2 - FD1)))"
echo ""

# 中等测试：10 个并发 x 5 轮
echo "3. 中等测试: 10 并发 x 5 轮..."
batch=1
while [ $batch -le 5 ]; do
  i=1
  while [ $i -le 10 ]; do
    curl -sk "https://127.0.0.1:443" -o /dev/null -w '' 2>/dev/null &
    i=$((i + 1))
  done
  wait
  FD_NOW=$(ls /proc/$HPID/fd 2>/dev/null | wc -l)
  echo "   Batch $batch: FD=$FD_NOW"
  batch=$((batch + 1))
done
echo ""

# 最终状态
echo "4. 最终状态:"
FD3=$(ls /proc/$HPID/fd 2>/dev/null | wc -l)
echo "   FD: $FD3"
echo ""

# 等待 30s 看连接释放
echo "5. 等待 30s 让连接释放..."
sleep 30
FD4=$(ls /proc/$HPID/fd 2>/dev/null | wc -l)
echo "   30s 后 FD: $FD4"
echo ""

# 检查错误
echo "6. 检查错误日志 (最近 50 行):"
ERRORS=$(tail -50 /root/hproxy.log | grep -E 'too many|error|accept|Loop' | wc -l)
if [ "$ERRORS" -eq 0 ]; then
    echo "   ✅ 无新增错误"
else
    echo "   ⚠️  发现 $ERRORS 个错误"
    tail -50 /root/hproxy.log | grep -E 'too many|error|accept|Loop' | head -5
fi
echo ""

# 总结
echo "=== 测试总结 ==="
echo "初始 FD: $FD1"
echo "轻度测试后: $FD2"
echo "中度测试后: $FD3"
echo "30s 后: $FD4"
echo ""
if [ $FD3 -lt 100 ] && [ $FD4 -lt 50 ]; then
    echo "✅ 测试通过: FD 数量正常，无泄漏"
else
    echo "⚠️  需要关注: FD 数量较高"
fi
