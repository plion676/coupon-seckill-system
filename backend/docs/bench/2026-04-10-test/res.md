**压测数据**
Requests      [total, rate, throughput]         63138, 10222.39, 8217.31
Duration      [total, attack, wait]             7.684s, 6.176s, 1.507s
Latencies     [min, mean, 50, 90, 95, 99, max]  95.279µs, 1.188s, 1.096s, 2.351s, 2.658s, 2.998s, 3.265s
Bytes In      [total, mean]                     1199622, 19.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:63138

- 经过极限压测说明，当流量洪峰时，平均延迟达到1.188s，p99接近3s，严重影响用户体验，为了系统稳定性，需要引入限流机制。