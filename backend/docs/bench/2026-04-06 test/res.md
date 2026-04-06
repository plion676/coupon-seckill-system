ROUTINE ======================== ...EvalSeckill
      .         13     84:   res, err := seckillScript.Run(ctx, rds.RDB, ...).Result()
业务协程都卡在了lua脚本

添加channel分片策略后，压测性能只增长了一点点，尝试使用Linux环境