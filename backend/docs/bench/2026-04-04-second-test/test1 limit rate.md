# 2026-04-04 第二次压测报告（异步下单限速版）

## 场景
- 初始库存：10000
- 模拟用户：50000
- 压测参数：rate=500, duration=60s (总请求 30000)

## 结果概览
| 指标 | 数值 |
| --- | --- |
| Requests（total, rate, throughput） | 30000, 500.02, 147.69 |
| QPS（Throughput） | 147.69 |
| Success Ratio | 33.28% |
| Status Codes | 0: 17239, 200: 9983, 409: 2762, 500: 16 |

## 延迟
- p95：9.997s
- p99：30.04s
- max：31.241s

## 现象与问题分析
1. **大量 Code 0 (17239次)**：
   - 错误信息：`connectex: No connection could be made because the target machine actively refused it.`
   - **分析**：受redis连接池限制影响，或操作系统临时端口耗尽
2. **200 OK (9983次)**：

3. **延迟极高 (p99 30s)**：
   - **分析**：由于服务器处理不过来，请求在 Gin 的队列中严重堆积，直到触发了 Vegeta 的默认 30s 超时。
