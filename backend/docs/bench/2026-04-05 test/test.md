# 2026-04-03 第一次压测报告

## 测试工具
- vegeta

## 场景
- 初始库存：10000
- Rate=1000, Duration=15s (总计约 15000 请求)

## 结果概览
| 指标 | 数值 |
| --- | --- |
| Requests（total, rate, throughput） 
           14999, 981.90, 631.72 |
| QPS | 631.72 |
| Status Codes | 0: 4056, 200: 10000, 409: 883, 500:60 |

## 延迟
- p50：297.74ms
- p95：785.12ms
- p99：12.33s
- max：15.62s

## 现象与结论
- code200精准10000，多余请求被409售罄拦截，无超卖少卖
## 一致性观察
**Redis**

SCARD coupon:users:1 = 10000
GET coupon:stock:1 = 0
XLEN seckill:stream = 10000

**MySQL**

- 落库订单数：10000

**结论**
- 1:1:1

## 下一步改进方向
- 引入atomic本地内存库存扣减，降低Redis多余调用
- 引入对账系统
- 压测指令keepalive=true，减少tcp三次握手四次挥手次数
- 维护一个channel，起一个协程，合并写Stream，减少redis的IOPS
- 在linux环境下测试
限流器 (Rate Limiter) ：
- 使用 golang.org/x/time/rate 实现令牌桶算法。
- 控制消费者退出
- 调大XReadstream中count的数值


