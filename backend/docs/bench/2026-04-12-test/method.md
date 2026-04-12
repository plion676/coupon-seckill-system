# 在测试机的终端执行这一行命令
# 注意：请把 172.16.1.XXX 换成你刚才复制的私有 IP
python3 -c 'for i in range(1, 200001): print(f"POST http://172.16.1.97:8080/seckill?coupon_id=1&user_id={i}")' > targets.txt

# 查看文件前 5 行，确认格式正确
head -n 5 targets.txt

# 替换为你的私有 IP
curl -I "http://172.16.1.97:8080/seckill?coupon_id=1&user_id=999"

vegeta attack -targets=targets.txt -rate=10000 -duration=20s -lazy -output=results.bin
vegeta report results.bin | grep -E "total|99%|Success|Status Codes"