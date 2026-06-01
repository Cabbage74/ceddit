# 三级缓存架构与热键探测系统

## 概述

面向公共 Feed 流场景，设计了 **"三级缓存 + 热点探测 + 缓存一致性"** 的完整缓存体系：

```
L2 (本地 Caffeine-style)  →  完整页面响应，直接返回，延迟最低
L1 (Redis 页面骨架)       →  ID 列表 + hasMore，快速装配
L0 (Redis 碎片缓存)       →  每帖元数据 + 计数，按需拼装
```

**核心理念**：将"页面装配"与"个性化覆盖"解耦 —— 前者在缓存层解决，后者仅在返回前以位图查询叠加，避免把个性化状态污染公共缓存。

## 架构图

```
请求 → L2 本地命中? ──yes──→ 叠加 liked/faved → 返回
           │
          no
           │
           ▼
        L1 骨架命中? ──yes──→ L0 碎片拼装 ──→ 写回 L2 ──→ 叠加 → 返回
           │
          no
           │
           ▼
      单飞锁 (single-flight)
           │
           ▼
      数据库回源 ──→ 写 L0 碎片 ──→ 写 L1 骨架 ──→ 写 L2 本地 ──→ 叠加 → 返回
```

## 一、三层缓存详解

### L2 — 本地内存缓存 (`pkg/cache/local.go`)

| 特性 | 说明 |
|------|------|
| 存储内容 | 完整 `FeedPageResponse`（条目数组 + 页码/页大小 + hasMore） |
| 存储位置 | 应用进程内存（Go map + sync.RWMutex） |
| TTL | 15s 基础 + 热键扩展 + 随机抖动（±10s） |
| 容量 | 可配置，默认 1000 条，超出时简单 FIFO 淘汰 |
| 清理 | 后台 goroutine 每 30s 清理过期条目 |

- 命中后直接返回，无需任何网络或序列化开销，延迟极低（微秒级）
- 专用于公共 Feed 的热门页，吸收热点 QPS 峰值
- 类似 Java Caffeine 的角色，但更轻量

### L1 — Redis 页面骨架缓存

| 特性 | 说明 |
|------|------|
| 存储内容 | `pageSkeleton`：ID 列表 (`[]int64`) + hasMore 标志 |
| 存储位置 | Redis String（JSON 序列化） |
| TTL | 10s 基础 + 热键扩展 + 随机抖动（±10s） |
| 键格式 | `feed:public:ids:{size}:{hourSlot}:{page}` |

- 命中后从 L0 碎片批量装配页面，不再访问数据库
- 使用**小时分片** (`hourSlot`) 降低跨小时内容更新导致的大面积失效
- TTL 短于 L0，确保骨架能反映最新的 ID 排序

### L0 — Redis 碎片缓存

| 特性 | 说明 |
|------|------|
| 存储内容 | `postFragment`（标题、作者、描述、时间） + `countFragment`（点赞数、收藏数） |
| 存储位置 | Redis String（JSON 序列化），每帖两个独立键 |
| TTL | 60s 基础 + 随机抖动（±30s） |
| 键格式 | `feed:frag:{postID}` / `feed:cnt:{postID}` |

- 粒度是"条目级"，碎片可跨页面复用
- 缺失时按需批量回源并回填，提升后续命中率
- 计数碎片与页面骨架 TTL 对齐，确保组装时数据可用

## 二、热键探测 (`pkg/cache/hotkey.go`)

### 滑动窗口计数模型

为每个缓存键维护一个 `[]int64` 计数数组，每个元素对应滑动窗口内的一个时间切片。

```
窗口总时长 = 60s, 分段粒度 = 10s → 6 个切片

 [t0]  [t1]  [t2]  [t3]  [t4]  [t5]
  ↑
 current (原子指针，O(1) 递增)

每次访问 → arr[current]++（原子操作，无锁）
每次轮转 → (current+1) % 6, arr[next] = 0
热度计算 → sum(arr[0..5])
```

**核心参数**（`config.yaml` 可配置）：

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `window_seconds` | 60 | 滑动窗口总时长 |
| `segment_seconds` | 10 | 时间切片粒度 |
| `level_low` | 50 | 低热度阈值（次/窗口） |
| `level_medium` | 200 | 中热度阈值 |
| `level_high` | 500 | 高热度阈值 |

### 热度分级与动态 TTL

```
heat < 50   → NONE  → 仅基础 TTL
heat ≥ 50   → LOW   → 基础 TTL + 20s
heat ≥ 200  → MEDIUM → 基础 TTL + 60s
heat ≥ 500  → HIGH  → 基础 TTL + 120s
```

- 每次缓存命中时实时计算热度等级
- TTL 扩展 + 随机抖动防止集中失效
- 后台 goroutine 按 `segment_seconds` 频率自动轮转窗口，旧计数自然衰减

### 与京东 HotKey 的对比

本方案是**轻量化本地热键治理**，与京东 HotKey 探测的共同点：
- 都使用滑动窗口作为核心统计机制
- 都基于阈值分级决策

区别：
- 本方案是**本地探测**，无跨节点通信开销，复杂度极低
- 京东 HotKey 是分布式探测集群，适合超大规模集群的热点调度
- 本方案通过"本地判定 + Redis TTL 延长"覆盖大部分热点场景

## 三、缓存一致性策略

### 事件驱动双删 (`InvalidatePublicFeed`)

```
内容变更（发布/编辑）
  │
  ├─ 1. 立即删除 Redis L1 骨架键（SCAN + DEL）
  ├─ 2. 立即删除 L2 本地缓存（按前缀匹配）
  ├─ 3. 等待 50ms（让并发回源写入完成）
  ├─ 4. 再次删除 L1 + L2（防止并发回源写回旧值）
  └─ 5. 热键计数器随下次 Record 自然重建
```

**为什么不删 L0 碎片？** 碎片是条目级数据，内容更新频率低，由 TTL 自然过期即可。双删 L1 骨架已足够让页面在下次请求时重新组装。

### 单飞锁防回源风暴 (`pkg/cache/singleflight.go`)

```
并发请求同一页（缓存同时失效）
  │
  ├─ Request 1 → 获取锁 → 回源 DB → 写回缓存 → 释放锁
  ├─ Request 2 → 等锁   → 复用结果 ✓
  ├─ Request 3 → 等锁   → 复用结果 ✓
  └─ Request N → 等锁   → 复用结果 ✓
```

- 以 L1 骨架键为"单飞键"
- 同一页在并发下仅允许一次数据库回源
- 进入锁后**重查缓存**，避免重复回源
- 与热键 TTL 延长协同：热点页失效频率降低，单飞触发次数显著减少

### 随机抖动抗雪崩

所有缓存层的 TTL 均加入 ±5~±30s 的随机抖动：

```go
// L1 TTL 示例
actualTTL = baseTTL + hotKeyExtend + rand.Intn(jitterRange)
```

即使大量页面处于同一热度等级，其失效时间也会被随机分散，避免"集中失效 → 集体回源"的缓存雪崩。

## 四、个性化覆盖（不解耦则乱）

```
公共缓存（L0/L1/L2）：
  ├─ 标题、作者、描述、发布时间
  ├─ 总点赞数、总收藏数
  └─ ❌ 不存 "我是否点赞了"

返回前内存叠加：
  ├─ 查询位图 "喜欢:帖子:{postID}:{userID}" (GETBIT, ~1ms)
  └─ 在返回的 items[i].liked 字段设置 true/false
```

**为什么个性化不进缓存？**
- 混合个性化会按用户维度切碎缓存 → 命中率剧降
- 每个用户的页面都不同 → 缓存键爆炸
- 改为"公共缓存 + 返回前覆盖"：既稳又快，命中率不受用户数影响

## 五、配置指南 (`config.yaml`)

```yaml
cache:
  hotkey:
    window_seconds: 60        # 滑动窗口总时长
    segment_seconds: 10       # 分段粒度（窗口/分段 = 6 切片）
    level_low: 50             # 低热阈值
    level_medium: 200         # 中热阈值
    level_high: 500           # 高热阈值
    extend_low_seconds: 20    # 低热 TTL 扩展
    extend_medium_seconds: 60 # 中热 TTL 扩展
    extend_high_seconds: 120  # 高热 TTL 扩展

  feed:
    l0_ttl_seconds: 60        # L0 碎片基础 TTL
    l0_jitter_seconds: 30     # L0 随机抖动范围
    l1_ttl_seconds: 10        # L1 骨架基础 TTL
    l1_jitter_seconds: 10     # L1 随机抖动范围
    l2_ttl_seconds: 15        # L2 本地基础 TTL
    l2_jitter_seconds: 10     # L2 随机抖动范围
    l2_max_entries: 1000      # L2 最大条目数
```

**调参原则**：
- `window_seconds` 越大，热度越平滑但响应越慢（建议 60~120s）
- `segment_seconds` 应使切片数保持在 6~12 个
- 热度阈值应基于实际 QPS 分布设定：
  - `level_low` ≈ P50 QPS × window_seconds
  - `level_medium` ≈ P90 QPS × window_seconds
  - `level_high` ≈ P99 QPS × window_seconds
- L0 TTL 应 ≥ L1 TTL，确保组装时碎片可用
- L2 TTL 应 ≈ L1 TTL，避免本地缓存长期持有过期数据

## 六、文件结构

```
pkg/cache/
├── types.go          # FeedPageResponse, FeedItemResponse, pageSkeleton, postFragment, countFragment, HeatLevel
├── config.go         # HotKeyConfig, FeedCacheConfig, LoadConfig()
├── keys.go           # Redis key builders (idsKey, fragmentKey, countKey, l2Key)
├── hotkey.go         # HotKeyDetector — 滑动窗口热度统计 + 分级 TTL 扩展
├── local.go          # localCache — 本地 TTL 内存缓存 (L2)
├── singleflight.go   # SingleFlight — 单飞锁防回源风暴
├── feed_cache.go     # FeedCache — 三级缓存编排器
└── cache_test.go     # 单元测试

service/post.go       # GetPublicFeed(), InvalidatePublicFeedCache(), InitFeedCache()
controller/post.go    # PublicFeedHandler
middleware/auth.go    # JWTAuthOptionalMiddleware (认证可选)
routes/routes.go      # GET /api/v1/feed
main.go               # service.InitFeedCache()
config.yaml           # cache 配置节
```

## 七、核心优势总结

| 维度 | 措施 | 效果 |
|------|------|------|
| **读延迟** | L2 本地命中 → 微秒级返回 | 热点页零网络开销 |
| **后端压力** | L1+L0 组装 → 仅 Redis 交互 | 普通页不访问数据库 |
| **缓存击穿** | 单飞锁 (single-flight) | 并发回源仅一次 |
| **缓存雪崩** | 随机抖动 TTL | 失效时间分散 |
| **缓存一致性** | 事件驱动双删 | 秒级最终一致 |
| **热点识别** | 滑动窗口 + 分级 TTL | 热点自动延长，冷点自然淘汰 |
| **命中率** | 个性化不进缓存 | 公共缓存复用率 100% |
| **落地成本** | 基于现有架构扩展 | 不重构核心业务逻辑 |

## 八、权衡与设计决策

1. **本地探测 vs 分布式探测**：选择本地探测，开销极低（O(1) 计数），无网络通信，与缓存逻辑天然耦合
2. **滑动窗口 vs 指数衰减**：滑动窗口逻辑简单、调参直观，"上快下也快"天然适配热点时变性
3. **碎片不删除 vs 全量双删**：仅双删 L1 骨架 + L2 本地，L0 碎片由 TTL 自然过期，减少 Redis 写压力
4. **命中率 vs 数据新鲜度**：双删保证内容变更时及时失效，扩展 TTL 不超过窗口的 2 倍
