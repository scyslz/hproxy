#!/bin/sh
# hproxy 压力测试脚本 - OpenWrt

echo "=== hproxy 压力测试 ==="
echo ""

# 配置
HPID=$(pidof hproxy)
TEST_DURATION=60  # 测试时长（秒）
CONCURRENCY=50    # 并发数
TOTAL_REQUESTS=10000

echo "测试配置:"
echo "  目标: https://127.0.0.1:443"
echo "  时长: ${TEST_DURATION}s"
echo "  并发: ${CONCURRENCY}"
echo "  总请求: ${TOTAL_REQUESTS}"
echo "  hproxy PID: ${HPID}"
echo ""

# 1. 初始状态
echo "1. 初始状态检查..."
FD_BEFORE=$(ls /proc/$HPID/fd 2>/dev/null | wc -l)
TW_BEFORE=$(cat /proc/net/sockstat | grep tw | awk '{print $2}')
echo "   FD 数量: $FD_BEFORE"
echo "   TIME_WAIT: $TW_BEFORE"
echo ""

# 2. 开始压测
echo "2. 开始压测..."
echo "   启动 $CONCURRENCY 个并发进程，每个发送 $((TOTAL_REQUESTS/CONCURRENCY)) 个请求..."

start_time=$(date +%s)

# 使用 curl 进行压测
for i in $(seq 1 $CONCURRENCY); do
    (
        for j in $(seq 1 $((TOTAL_REQUESTS/CONCURRENCY))); do
            curl -sk "https://127.0.0.1:443" -o /dev/null -w '' 2>/dev/null
        done
    ) &
done

# 等待所有进程完成
wait

end_time=$(date +%s)
duration=$((end_time - start_time))

echo "   压测完成，耗时: ${duration}s"
echo ""

# 3. 压测中监控（每 5 秒采样一次）
echo "3. 压测中状态监控..."
for i in $(seq 1 5); do
    sleep 5
    FD_NOW=$(ls /proc/$HPID/fd 2>/dev/null | wc -l)
    TW_NOW=$(cat /proc/net/sockstat | grep tw | awk '{print $2}')
    echo "   ${i}x5s: FD=$FD_NOW, TW=$TW_NOW"
done
echo ""

# 4. 最终状态
echo "4. 最终状态检查..."
FD_AFTER=$(ls /proc/$HPID/fd 2>/dev/null | wc -l)
TW_AFTER=$(cat /proc/net/sockstat | grep tw | awk '{print $2}')
echo "   FD 数量: $FD_AFTER (变化: $((FD_AFTER - FD_BEFORE)))"
echo "   TIME_WAIT: $TW_AFTER (变化: $((TW_AFTER - TW_BEFORE)))"
echo ""

# 5. 检查错误日志
echo "5. 检查错误日志..."
ERRORS=$(tail -100 /root/hproxy.log | grep -E 'too many|error|错误|accept' | wc -l)
if [ $ERRORS -eq 0 ]; then
    echo "   ✅ 无错误"
else
    echo "   ❌ 发现 $ERRORS 个错误"
    tail -100 /root/hproxy.log | grep -E 'too many|error|错误|accept'
fi
echo ""

# 6. 等待 30s 让 socket 释放
echo "6. 等待 30s 让 socket 释放..."
sleep 30

FD_FINAL=$(ls /proc/$HPID/fd 2>/dev/null | wc -l)
TW_FINAL=$(cat /proc/net/sockstat | grep tw | awk '{print $2}')
echo "   30s 后 FD: $FD_FINAL"
echo "   30s 后 TIME_WAIT: $TW_FINAL"
echo ""

# 7. 总结
echo "=== 测试总结 ==="
echo "初始 FD: $FD_BEFORE"
echo "压测后 FD: $FD_AFTER"
echo "最终 FD: $FD_FINAL"
echo ""
echo "初始 TW: $TW_BEFORE"
echo "压测后 TW: $TW_AFTER"
echo "最终 TW: $TW_FINAL"
echo ""

if [ $FD_AFTER -lt 100 ] && [ $ERRORS -eq 0 ]; then
    echo "✅ 测试通过: FD 数量正常，无泄漏，无错误"
else
    echo "❌ 测试失败: 请检查 FD 数量或错误日志"
fi
