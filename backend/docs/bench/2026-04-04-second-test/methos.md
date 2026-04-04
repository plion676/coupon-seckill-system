**在同一目录下进行**
**创建post文件**
1..20000 | ForEach-Object { "POST http://localhost:8080/seckill?coupon_id=1&user_id=$_" } > targets_multi.txt


**attack测试**
Get-Content targets_multi.txt | vegeta attack -rate 0 -workers 500 -max-workers 500 -duration 30s -output=results.bin



vegeta report results | Select-String -Pattern "Requests|Latencies|Status Codes|Success" 
-Context 0,2