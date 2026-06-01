# CountInt：Redis SDS 二进制计数系统

## 概述

将用户维度和文章维度的计数从 **Redis Hash + ZSet** 迁移到 **Redis String（SDS）+ 二进制编码（CountInt）**，用一块连续内存存储多个 int64 计数字段。

**核心思路：**

```
之前（Hash）：                    之后（CountInt SDS）：
                                
ucnt:123 (Hash)                 ucnt:123 (String, 16 bytes)
  ├── "following_count" → 100   ┌──────────────────────────────────┐
  ├── "follower_count"  → 50    │ 100 0 0 0 0 0 0 0 │ 50 0 0 0 0 0 0 0 │
  └── ...                       └──┬─────────────────┬──────────────┘
   每个 field 存一遍字段名字      following_count    follower_count
   每个 field 有 hash 元数据     (offset 0, 8B)     (offset 8, 8B)
                                
pcnt:456 (ZSet vote 统计)       pcnt:456 (String, 8 bytes)
  └── ZCount("1","1") -         ┌──────────────────────────────────┐
      ZCount("-1","-1")         │ 42 0 0 0 0 0 0 0 │
                                 └──┬─────────────────┘           
                                   like_count (offset 0, 8B)
```

**变化：**
- 没有字段名，只存值。访问 = 起始地址 + 类型偏移。
- 一个用户 / 一个帖子只占一个 Key。
- 千万级用户规模下，比 Hash + 字符串字段省大量内存。

**Scope：**
- 文章维度：`like_count`（点赞数）
- 用户维度：`following_count`（关注数）、`follower_count`（粉丝数）

---

## 二进制布局

### User CountInt (`ucnt:{user_id}`)

```
Byte  0         8        16
      ├─────────┼────────┤
      │following │follower│
      │ (int64)  │(int64) │
      └─────────┴────────┘
      
Blob size: 16 bytes
```

| 字段 | 偏移 | 类型 | 说明 |
|---|---|---|---|
| `following_count` | 0 | int64 LE | 该用户关注了多少人 |
| `follower_count` | 8 | int64 LE | 该用户有多少粉丝 |

### Post CountInt (`pcnt:{post_id}`)

```
Byte  0         8
      ├─────────┤
      │  like   │
      │(int64)  │
      └─────────┘
      
Blob size: 8 bytes
```

| 字段 | 偏移 | 类型 | 说明 |
|---|---|---|---|
| `like_count` | 0 | int64 LE | 点赞数（仅统计 upvote） |

**为什么用 little-endian：** Go 的 `encoding/binary` 默认操作 LE，CPU x86 架构原生匹配，零转换开销。

---

## 原子操作：Lua 脚本

读取 → 修改 → 写回 必须原子化，否则并发 INCR 会互相覆盖。Redis 不支持原生"对 String 里某个偏移的 int64 做加减"，所以用 Lua 脚本：

```lua
-- KEYS[1]    = count key (e.g. "ucnt:123")
-- ARGV[1]    = byte offset (0 for following, 8 for follower)
-- ARGV[2]    = delta (signed int, e.g. +1 or -1)
-- ARGV[3]    = blob size (16 for user, 8 for post)

local val = redis.call('GET', KEYS[1])
if not val then
    val = string.rep(string.char(0), tonumber(ARGV[3]))
end

-- Read int64 LE at offset
local cur = 0
for i = 0, 7 do
    local b = string.byte(val, ARGV[1] + i + 1) or 0
    cur = cur + b * (256 ^ i)
end

-- Apply delta, floor at 0
local nv = cur + tonumber(ARGV[2])
if nv < 0 then nv = 0 end

-- Write back int64 LE
local out = {}
for i = 0, 7 do
    out[i] = string.char(math.floor(nv / (256 ^ i)) % 256)
end

local new_val = string.sub(val, 1, ARGV[1])
    .. table.concat(out)
    .. string.sub(val, ARGV[1] + 9)
redis.call('SET', KEYS[1], new_val)
return nv
```

**Lua 数字精度说明：** Redis Lua 用 IEEE 754 double，精确整数范围为 ±2^53（约 9 千万亿）。对于关注数 / 粉丝数 / 点赞数来说绰绰有余。

启动时通过 `SCRIPT LOAD` 预加载脚本，后续调用使用 `EVALSHA`，避免每次传输完整脚本。

---

## 实时更新链路

### 关注 / 取关 → CountInt

```
POST /api/v1/users/:id/follow
  → MySQL TX: INSERT INTO following + INSERT INTO outbox
  → Canal binlog → Kafka (user_relation_events)

Consumer (goroutine or standalone):
  → MySQL: INSERT/UPDATE follower projection
  → Redis Lua EVALSHA:
      from_user: ucnt:{from} offset=0 delta=+1   (following_count++)
      to_user:   ucnt:{to}   offset=8 delta=+1   (follower_count++)

POST /api/v1/users/:id/unfollow
  → same pipeline, deltas = -1
```

更新失败不影响主链路 — 只在日志 Warn，由 Reconciler 兜底修复。这就是"计数是近实时视图，允许短期偏差"的工程实践。

### 投票 → CountInt

```
POST /api/v1/vote { post_id, direction }

  old_direction = Redis ZScore(post:voted:XXX, user_id)
  
  if old != 1 and new == 1  → pcnt:{post_id} like_count += 1
  if old == 1 and new != 1  → pcnt:{post_id} like_count -= 1
  else                      → no change
  
  (同时保留原有 ZSet 记录，用于投票去重和投票期限制)
```

**为什么 only track upvotes：** 按照需求，文章维度只需关注点赞数（like_count）。踩（downvote）仍然通过 ZSet 记录用于排序和业务逻辑，但不反映到 CountInt 的 like_count 字段。

### 读取 → CountInt

```
GET /api/v1/post/:id        → redis.GetPostLikeCount(postID)
GET /api/v1/post             → 每个 post 调用 redis.GetPostLikeCount(postID)

GET /api/v1/users/:id/following  → redis.GetUserCounts(userID).following
GET /api/v1/users/:id/followers  → redis.GetUserCounts(userID).follower
```

读取直接用 `GET` + Go 侧 `binary.LittleEndian.Uint64` 解码，不走 Lua。读操作无原子性问题。

**CountInt 不存在时的行为：** 新用户 / 新帖子的 CountInt Key 尚未创建，`GetUserCounts` 和 `GetPostLikeCount` 返回 `(0, 0)` 和 `0`。对 API 响应来说语义正确。

---

## 对账（Reconciler）

即使 Consumer 实时更新 CountInt，仍需要定期对账，处理极端情况（Consumer 重启丢消息、Redis 主从切换丢少量写入、运维误操作等）。

```
cron: */5 * * * * ceddit-reconciler
```

### 对账逻辑

```
① 用户维度：
   SELECT from_user_id, COUNT(*) FROM following GROUP BY from_user_id
   SELECT to_user_id,   COUNT(*) FROM follower   GROUP BY to_user_id
   → 合并两个 Map，得到每个用户的 (following, follower) 真值
   → SET ucnt:{user_id} = EncodeUserCounts(following, follower)

② 文章维度：
   SELECT id FROM post WHERE status = 'published'
   → 对每个 post，ZCount(post:voted:{id}, "1", "1") 得真实点赞数
   → SET pcnt:{post_id} = EncodePostLikeCount(like_count)

③ 清理僵尸数据：
   DELETE f FROM follower f
   LEFT JOIN following fw ON fw.from_user_id = f.from_user_id
                         AND fw.to_user_id   = f.to_user_id
   WHERE fw.id IS NULL
```

**对账的意义：** 系统从"可能永远偏差"变成"短期偏差，长期收敛到正确"。

---

## RDB + AOF：不落 MySQL 的持久化保障

既然计数完全由 Redis 维护（无 MySQL 计数表），必须确保 Redis 重启后能恢复：

| 机制 | 作用 | 配置 |
|---|---|---|
| **RDB 快照** | 定期将内存快照写入磁盘，重启快速恢复 | `save 900 1`（15 分钟至少 1 个 key 变化）<br>`save 300 10`<br>`save 60 10000` |
| **AOF 日志** | 每条写操作追加到日志文件，重启时重放 | `appendonly yes`<br>`appendfsync everysec`（每秒刷盘，最多丢 1 秒数据） |
| **主从 + 哨兵** | 实例挂掉自动切换（后续扩展） | 未部署 |

```
Redis 重启流程:
  ① 加载 RDB 快照 → 恢复到某个时间点
  ② 重放 AOF 日志 → 将状态逼近到宕机前 1 秒内
  ③ Reconciler 定期对账 → 修正任何残留偏差
```

---

## 代码结构

```
ceddit/
├── pkg/countint/
│   ├── countint.go           # 二进制布局常量 + Encode/Decode 函数
│   └── lua.go                # Lua 原子自增脚本 (CountIncrByScript)
│
├── repository/redis/
│   ├── keys.go               # +KeyUserCountPrefix("ucnt:") +KeyPostCountPrefix("pcnt:")
│   ├── counts.go             # IncrUserCount / IncrPostLikeCount
│   │                         # GetUserCounts / GetPostLikeCount
│   │                         # SetUserCounts / SetPostLikeCount
│   │                         # InitCountScripts
│   ├── redis.go              # (已有) 连接管理
│   ├── post.go               # (已有) ZSet 时间线/投票记录
│   └── vote.go               # (已有) ZSet 投票去重/投票期限制
│
├── service/
│   ├── consumer.go           # 在 projectFollow/projectUnfollow 中同步更新 CountInt
│   ├── vote.go               # 在 VoteForPost 中增量更新 like_count
│   ├── post.go               # GetPost/GetPostList 从 CountInt 读点赞数
│   └── relation.go           # GetFollowingList/GetFollowerList 从 CountInt 读关注/粉丝数
│
├── cmd/
│   ├── consumer/main.go      # 独立消费者：Redis Init → 消费消息 → 更新 CountInt
│   └── reconciler/main.go    # 对账任务：MySQL GROUP BY → SET CountInt 二进制 blob
│
├── main.go                   # +redis.InitCountScripts() 启动时加载 Lua 脚本
├── docker-compose.yml        # Redis AOF + RDB 持久化配置
└── docs/
    └── countint.md           # 本文档
```

---

## 与旧方式的对比

| 维度 | 旧（Hash + ZSet） | 新（CountInt SDS） |
|---|---|---|
| **Key 数量** | 每人 1 个 Hash Key + 多 field<br>每帖 1 个 ZSet Key | 每人 1 个 String Key<br>每帖 1 个 String Key |
| **字段名存储** | 每个 field 存字符串名（如 `"following_count"`） | 无字段名，仅存值 |
| **内存布局** | Hash/ziplist 内部指针 + 元数据 | 连续内存块 |
| **访问方式** | Hash field 查找 → 字符串解析 | 起始地址 + 类型偏移 |
| **原子操作** | `HINCRBY` 单 field 原子（Redis 内置） | Lua 脚本实现（需预加载） |
| **灵活性** | 新增字段只需 `HSET` 新 field | 需改二进制布局 + Lua 脚本 |
| **千万用户内存** | ~几百 MB（Hash overhead） | ~160 MB（纯数据） |

---

## 为什么计数"不落库"

| | MySQL 计数 | Redis CountInt |
|---|---|---|
| **写压力** | 热点行频繁 UPDATE，行锁、redo log 压力大 | Redis 单线程，INCR 是 O(1) 内存操作 |
| **DDL 成本** | 新增计数维度需 ALTER TABLE | 新增字段只改 CountInt 布局 |
| **架构耦合** | 计数和业务数据混在一起 | 计数独立子系统，职责单一 |
| **一致性** | 强一致（事务） | 最终一致（对账修复） |
| **可恢复性** | 数据在磁盘 | RDB + AOF + 对账兜底 |

业务事实（following 表、follower 表、投票记录）由 MySQL 持久化。计数只是"对事实的聚合视图"，完全可以由 Redis 维护 + 对账修正。

---

## 待完善项

- [x] CountInt 二进制布局（User 16B / Post 8B）
- [x] Lua 原子自增脚本 + 启动预加载（EVALSHA）
- [x] Consumer 实时更新 CountInt（关注 / 取关）
- [x] 投票同步更新 post like_count
- [x] API 读取改为 CountInt（帖子列表、帖子详情、关注/粉丝列表）
- [x] Reconciler 使用 CountInt 二进制编码对账
- [x] Redis AOF + RDB 持久化配置
- [ ] Pipeline 批量读取优化：`GetPostList` 当前逐帖 `GET`，可改为 `MGET` + 批量解码
- [ ] 计数 Schema 版本号：在 CountInt blob 头加 1 字节版本，支持字段布局演进
- [ ] 热点用户本地缓存：对千万粉大 V 的 CountInt 做进程内 LRU 缓存，减少 Redis 压力
- [ ] 独立的 Go test 覆盖 Lua 脚本的字节操作正确性
- [ ] 监控：对账偏差告警（当偏差超过阈值时报警）
