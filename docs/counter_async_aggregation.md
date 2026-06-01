# 计数器异步写聚合方案 (Counter Async Write Aggregation)

## 一、方案背景与动机

在高并发社交场景中，用户的点赞（like）、收藏（fav）、评论等行为会产生海量计数更新请求。原有的"同步直写最终计数"方案存在以下问题：

### 痛点

1. **写热点与写放大**：每次用户操作都直接写入 Redis SDS 计数器，热门实体（爆款帖子）的计数键成为热点，导致 CPU/IO 资源竞争。
2. **随机写加剧开销**：不同指标（like/fav/comment）独立更新同一实体的不同字段，产生大量随机写。
3. **一致性与体验矛盾**：用户需要即时看到自己的操作状态（强一致），但总计数可以容忍秒级延迟（最终一致）。

### 解决方案

引入 **"事实/计数分离 + 异步解耦 + 写聚合 + 批量刷写"** 四层架构：

```
用户操作 → 同步写分片位图（事实层，即时）
         → 异步生产增量事件（Kafka）
              → 聚合消费（Redis Hash 桶）
                   → 定时刷写（1s，Lua 原子操作）→ SDS 最终计数
                   ← 异常时从位图重建 SDS 计数
```

## 二、核心架构

### 四层架构

| 层级 | 组件 | 职责 |
|------|------|------|
| 事实层 | `pkg/bitmap/` (分片位图) | 记录"谁对什么做了什么"的布尔事实（强一致、幂等） |
| 生产端 | `service/vote.go` + `pkg/counter/producer.go` | 位图 toggle → 异步产出 `CounterEvent` 到 Kafka |
| 中间层 | Kafka + `pkg/counter/aggregator.go` | 可靠传输增量事件；聚合到 Redis Hash 桶 |
| 消费端 | `pkg/counter/flusher.go` | 定时扫描活跃桶，原子刷写到 SDS 计数器 |
| 补偿层 | `pkg/counter/rebuild.go` | SDS 缺失/损坏时从位图事实层重建；分布式锁防并发 |

### 数据流

```
1. 用户点赞
   ↓
2. service.VoteForPost()
   ├── 同步: bitmap.Toggle("like", "post", postID, userID, true)
   │         → bm:like:post:{postID}:{chunk} SETBIT (分片位图)
   │         → 返回 {changed, liked}
   │         → 若 changed: 更新排名分数 ZSet
   └── 异步: counter.PublishPostLikeEvent() → Kafka
        ↓
3. counter.RunAggregationConsumer()
   ├── 消费 Kafka 消息
   ├── HINCRBY agg:v1:post:{postID}:{timeSlot} {offset} {delta}
   └── SADD active:agg:v1 {bucketKey}
        ↓
4. counter.RunFlushScheduler() (每 1s)
   ├── SMEMBERS active:agg:v1
   ├── 遍历每个桶 → HGETALL 获取增量
   ├── FlushScript (Lua): 原子执行
   │   ├── HGET delta
   │   ├── GET + SET SDS blob (little-endian int64 算术)
   │   ├── HDEL field
   │   └── 桶空时: SREM + DEL
   └── 完成
        ↓ (异常时)
5. counter.RebuildPostCounts(postID)
   ├── 获取分布式锁 lock:sds-rebuild:post:{postID}
   ├── BITCOUNT 所有分片 bm:like:post:{postID}:*
   ├── 写入 SDS
   ├── 清理对应聚合桶字段
   └── 释放锁
```

## 三、新增/修改文件清单

### 新增文件

| 文件 | 说明 |
|------|------|
| `pkg/bitmap/shard.go` | 分片位图配置：`ChunkSize`(32K bits), `ChunkOf()`, `BitOf()` |
| `pkg/bitmap/keys.go` | 位图键生成：`bm:{metric}:{etype}:{eid}:{chunk}` |
| `pkg/bitmap/lua.go` | `ToggleScript` Lua 脚本（原子 GETBIT→SETBIT，幂等） |
| `pkg/bitmap/ops.go` | 位图操作：`Toggle`, `IsSet`, `CountShards` |
| `pkg/counter/event.go` | `CounterEvent` 结构体定义和构造函数 |
| `pkg/counter/keys.go` | Redis 聚合桶键生成、活跃桶集合键、时间槽工具 |
| `pkg/counter/lua.go` | `FlushScript` Lua 脚本（原子读增量+应用SDS+删除字段） |
| `pkg/counter/producer.go` | Kafka 生产者（`InitProducer`, `PublishPostLikeEvent`） |
| `pkg/counter/aggregator.go` | Kafka 消费者（聚合增量到 Redis Hash 桶） |
| `pkg/counter/flusher.go` | 定时刷写调度器（1s 周期扫描+原子落盘） |
| `pkg/counter/schema.go` | 计数器字段布局定义（指标名 → SDS 偏移量映射） |
| `pkg/counter/lock.go` | 分布式锁（`SET NX EX` + token 验证释放） |
| `pkg/counter/rebuild.go` | SDS 重建：从位图事实层 BITCOUNT 恢复计数 |

### 修改文件

| 文件 | 变更内容 |
|------|----------|
| `config.yaml` | 新增 `kafka.counter_topic` 和 `kafka.counter_consumer_group` |
| `main.go` | 新增 `bitmap.InitScripts()`, `counter.InitCounterScripts()`, `counter.InitProducer()`, 启动聚合消费者和刷写调度器 goroutine |
| `service/vote.go` | 使用 `redis.ToggleLike()` (位图) 替代 ZSet；`VoteForPost` 返回 `(changed, liked)` |
| `controller/vote.go` | 返回 `{changed, liked}` JSON，客户端无需二次请求获取状态 |
| `repository/redis/vote.go` | 新增 `ToggleLike()`, `IsLiked()` (基于位图)；标记旧 ZSet 函数为 deprecated |

## 四、关键设计决策

### 4.1 事实/计数分离

```
事实层（强一致）                    计数层（最终一致）
┌─────────────────────┐           ┌──────────────────────┐
│ 分片位图 (bitmap)    │           │ SDS 固定结构计数器     │
│                     │           │                      │
│ bm:like:post:456:0  │  事件→    │ pcnt:456             │
│ bm:like:post:456:1  │  聚合→    │   [like_count:8byte]  │
│ ...                 │  刷写→    │                      │
│                     │           │                      │
│ 记录"谁"做了"什么"   │  重建←    │ 记录"统计结果是多少"    │
│ 单用户 GETBIT 即时   │           │ 批量读取高效           │
└─────────────────────┘           └──────────────────────┘
```

- **同步路径**：`bitmap.Toggle()` 立即修改分片位图，`IsLiked()` 用单次 GETBIT 返回（毫秒级），用户刷新即可看到按钮状态。
- **异步路径**：位图 toggle 成功后产出 CounterEvent → Kafka → 聚合桶 → 刷写 → SDS，总计数延迟约 1 秒。
- **异常重建**：SDS 缺失或损坏时，从位图 BITCOUNT 全量分片重建，通过分布式锁防止并发。

### 4.2 分片位图设计

- **键结构**：`bm:{metric}:{entityType}:{entityId}:{chunk}`
- **分片大小**：32K bits (4 KB / shard)
- **用户映射**：`chunk = uid / 32768`, `bit = uid % 32768`
- **为什么分片**：
  - 避免单键过大（热门帖子数百万用户 → 一个 key 几 MB）
  - 分摊热点读写压力到多个 key
  - 并行 BITCOUNT 统计，加速重建
  - 冷分片可独立迁移 / 清理

### 4.3 分区策略

Kafka 生产者使用 `entityType:entityID` 作为分区键（Hash 分区器），确保同一实体的所有事件进入同一分区，保证顺序消费。

### 4.4 聚合桶设计

- **键结构**：`agg:v1:{entityType}:{entityID}:{timeSlot}`
  - `timeSlot` 为小时级时间槽（如 `2024100114`），拆分键空间避免热点
  - Hash 字段为 SDS 字节偏移量，值为该时间窗口内的累计增量
- **活跃桶集合**：`active:agg:v1`（Redis Set），避免 `KEYS *` 全量扫描
- **TTL**：2 小时，远长于 1 秒刷写周期，防止桶被提前清理

### 4.5 原子操作

**位图 Toggle** (`bitmap/lua.go`)：
1. GETBIT 读取当前值
2. 若已是目标值 → 返回 `{changed: 0, old: current}`
3. 否则 SETBIT → 返回 `{changed: 1, old: previous}`
4. 整个脚本原子执行，且天然幂等

**刷写 FlushScript** (`counter/lua.go`)：
1. 从聚合桶 HGET delta
2. 应用到 SDS 计数器（little-endian int64 算术）
3. HDEL 聚合字段
4. 桶空时 SREM + DEL
5. 全部在一个 Lua 事务中，无竞态

### 4.6 可靠性与容灾

- **Kafka 生产者**：`acks=all`（等待所有 ISR 副本确认）、LZ4 压缩、5ms 批量窗口
- **Kafka 消费者**：自动重连，失败时从 `FirstOffset` 重放
- **幂等性**：位图 toggle 脚本天然幂等；HINCRBY 在桶级别幂等（刷写原子删字段）
- **补偿机制**：
  - `cmd/reconciler`：定期从 MySQL 源数据修复 SDS（兜底）
  - `counter.RebuildPostCounts()`：按需从位图事实层 BITCOUNT 重建（精确）
- **分布式锁**：`SET lock:sds-rebuild:{etype}:{eid} token NX EX 5s`，防并发重建

### 4.7 写放大降低

- 位图层面：每个用户操作只触发一次 SETBIT（4KB 分片内的 1 bit），内存开销极小
- 计数层面：同一实体在 1 秒窗口内的多次操作，聚合为 1 次 SDS 写入
- 写放大系数降低约 10-1000 倍（取决于并发量）

## 五、配置说明

`config.yaml` 新增项：

```yaml
kafka:
  counter_topic: "counter_events"       # 计数器事件主题
  counter_consumer_group: "counter-agg"  # 聚合消费组 ID
```

Kafka 自动创建主题已启用（`KAFKA_CFG_AUTO_CREATE_TOPICS_ENABLE: "true"`），无需手动创建。

## 六、启动流程

`main.go` 中的初始化顺序：

1. `redis.Init()` — Redis 连接
2. `redis.InitCountScripts()` — 预加载 CountInt SDS Lua 脚本
3. `counter.InitCounterScripts()` — 预加载刷写 Lua 脚本
4. `bitmap.InitScripts(rdb)` — 预加载位图 toggle Lua 脚本
5. `counter.InitProducer()` — 初始化 Kafka 生产者
6. `go counter.RunAggregationConsumer()` — 启动聚合消费者 goroutine（自动重连）
7. `go counter.RunFlushScheduler()` — 启动刷写调度器 goroutine（1s 间隔）

## 七、API 变更

### POST /api/v1/vote

**请求** (不变)：
```json
{"post_id": "456", "direction": 1}
```

**响应** (增强)：
```json
{
  "code": 200,
  "data": {
    "changed": true,   // 新增：状态是否实际变化（防重复点击）
    "liked": true      // 新增：当前点赞状态（客户端直接更新按钮）
  }
}
```

客户端无需额外请求即可更新 UI：`changed=false` 时忽略，`liked` 直接反映按钮状态。

## 八、扩展指南

### 新增指标类型（如收藏 fav）

1. `pkg/counter/schema.go`：新增 `MetricFav` 常量 + `PostFieldFav` 偏移量
2. `pkg/countint/countint.go`：扩展 Post SDS blob（如从 8 字节扩到 16 字节）
3. 业务层：`bitmap.Toggle("fav", "post", postID, userID, true)` + `counter.PublishPostFavEvent()`
4. `counter/flusher.go`：`sdsKeyAndSize()` 更新 blobSize

### 新增实体类型（如用户计数）

1. `pkg/countint/countint.go`：定义新的 offset/blobSize 常量
2. `pkg/counter/schema.go`：新增 `EntityUser` + 对应指标映射
3. `pkg/counter/flusher.go`：`sdsKeyAndSize()` 新增 case
4. `pkg/counter/rebuild.go`：新增 `RebuildUserCounts()` 函数

### 位图分片大小调整

- `pkg/bitmap/shard.go`：修改 `ChunkSize` 常量
- 注意：已存在的位图键使用旧分片大小，需要迁移脚本
