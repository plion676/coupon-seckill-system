1..20000 | ForEach-Object { "POST http://localhost:8080/seckill?coupon_id=1&user_id=$_" } > targets_multi.txt

vegeta attack -rate 1500 -duration 15s -targets="targets_multi.txt" -output="results.bin"

