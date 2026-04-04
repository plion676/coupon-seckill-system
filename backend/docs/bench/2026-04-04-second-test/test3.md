# 2026-04-04 第四次压测报告（单机极限测试）

## 场景
- 初始库存：10000
- 压测策略：Rate=0 (不限速), Workers=500
- 持续时间：30s
- redis连接数限制:500
- 将redis的queue换成了stream数据结构，并加入了本地缓存

**第一次**
这一次中worker写入sql时间还是过长，一次写17条数据，用了300ms，
XLEN甚至不动了，数据库写入还是太慢了，尝试加入协程
**第二次**
Status Codes [code:count]
  0:7769  200:4396
加入协程之后，connectex: No connection could be made... actively refused it.受windows端口限制影响，改端口限制

2.改测试压力

3.扣减和XAdd不在一个原子操作内