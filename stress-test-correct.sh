#!/bin/sh
# 正确的压测脚本 - 使用外部域名测试代理

echo "=== hproxy 压力测试 (外部域名) ==="
echo ""

HPID=$(pidof hproxy)
if [ -z "$HPID" ]; then
    echo "❌ hproxy 未运行"
    exit 1
fi

echo "hproxy PID: $HPID"
echo ""

# 初始状态
echo "1. 初始状态:"
FD1=$(ls /proc/$HPID/fd 2>/dev/null | wc -l)
TW1=$(cat /proc/net/sockstat | grep tw | awk '{print $2}')
echo "   FD: $FD1"
echo "   TIME_WAIT: $TW1"
echo ""

# 测试域名（外部地址，会走 DIRECT 模式）
TEST_DOMAIN="www.baidu.com"
echo "测试域名: $TEST_DOMAIN"
echo ""

# 轻度测试：50 个串行请求
echo "2. 轻度测试: 50 个串行请求..."
i=0
while [ $i -lt 50 ]; do
  curl -sk "https://192.168.100.1" -o /dev/null -w '' -H "Host: $TEST_DOMAIN" 2>/dev/null
  i=$((i + 1))
done
echo "完成"
FD2=$(ls /proc/$HPID/fd 2>/dev/null | wc -l)
echo "   请求后 FD: $FD2 (变化: $((FD2 - FD1)))"
echo ""

# 中度测试：20 个并发 x 3 轮
echo "3. 中度测试: 20 并发 x 3 轮..."
batch=1
while [ $batch -le 3 ]; do
  i=0
  while [ $i -lt 20 ]; do
    curl -sk "https://192.168.100.1" -o /dev/null -w '' -H "Host: $TEST_DOMAIN" 2>/dev/null &
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
TW3=$(cat /proc/net/sockstat | grep tw | awk '{print $2}')
echo "   FD: $FD3"
echo "   TIME_WAIT: $TW3"
echo ""

# 等待 30s 看连接释放
echo "5. 等待 30s 让连接释放..."
sleep 30
FD4=$(ls /proc/$HPID/fd 2>/dev/null | wc -l)
TW4=$(cat /proc/net/sockstat | grep tw | awk '{print $2}')
echo "   30s 后 FD: $FD4"
echo "   30s 后 TIME_WAIT: $TW4"
echo ""

# 检查错误
echo "6. 检查错误日志 (最近 50 行):"
ERRORS=$(tail -50 /root/hproxy.log | grep -E 'too many|error|accept' | wc -l)
if [ "$ERRORS" -eq 0 ]; then
    echo "   ✅ 无错误"
else
    echo "   ⚠️  发现 $ERRORS 个错误"
    tail -50 /root/hproxy.log | grep -E 'too many|error|accept' | head -5
fi
echo ""

# 总结
echo "=== 测试总结 ==="
echo "初始 FD: $FD1 → 最终 FD: $FD4 (变化: $((FD4 - FD1)))"
echo "初始 TW: $TW1 → 最终 TW: $TW4 (变化: $((TW4 - TW1)))"
echo ""
if [ $FD4 -lt 100 ] && [ $FD3 -lt 100 ]; then
    echo "✅ 测试通过: FD 数量正常，无泄漏"
else
    echo "⚠️  需要关注: FD 数量较高"
fi
