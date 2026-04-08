**测试方法**
- 启动 API ：在窗口 1 执行 go run cmd/api/main.go 。
- 启动 Worker ：在窗口 2 执行 go run cmd/worker/main.go 。
- 预热数据 ：
- 用本地 Postman 重新发一个 POST /coupon/create ，设置 stock 为 100000 。
- 验证预热 ：执行 redis-cli GET coupon:stock:1 ，必须看到 100000 。

# 执行测试
vegeta attack -targets=targets_local.txt -rate=10000 -duration=11s | vegeta report

**测试后清理环境方法(SSH终端)**
# 1. 杀掉 API 和 Worker 进程
pkill go && pkill main

# 2. 清空 Redis 所有数据 (极其重要)
redis-cli FLUSHALL

# 3. 清空 MySQL 订单和优惠券表
mysql -uroot -p123456 practice -e "TRUNCATE TABLE orders; TRUNCATE TABLE coupons;"

# 确保 user_id 不重复，生成 150,000 个请求
rm -f targets_local.txt
for i in {1..200000}; do
  echo "POST http://localhost:8080/seckill?coupon_id=1&user_id=$i" >> targets_local.txt
done