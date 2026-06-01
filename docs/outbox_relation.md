# Outbox + Canal → Kafka → 投影表：用户关系系统

## 概述

以 **Outbox 模式 + Canal 订阅 Binlog + Kafka 异步投递** 实现用户关注/取关功能。

**核心链路：**

```
POST /api/v1/users/:id/follow
  → Service (Redis 幂等检查)
    → Repository BEGIN TX
        ├── INSERT INTO following        (主表，物理删除)
        └── INSERT INTO outbox           (事件表，Binlog)
    → COMMIT
    → 返回 201

Canal 伪装 MySQL 从库 → 读取 outbox 表 Binlog
  → Canal Kafka Adapter → 投递到 "user_relation_events"

Consumer (主进程 Goroutine) → 消费 Kafka 消息
  → FOLLOW:  INSERT INTO follower ... ON DUPLICATE KEY UPDATE
  → UNFOLLOW: DELETE FROM follower
  → last_event_id 条件判断保证幂等

Reconciler (cron) → 定期对账 Redis 计数 + 清理僵尸投影行
```

---

## 表结构

### `following` — 关注主表（源表，物理删除）

| 列 | 类型 | 说明 |
|---|---|---|
| `id` | BIGINT UNSIGNED PK | Snowflake ID |
| `from_user_id` | BIGINT UNSIGNED | 关注者 |
| `to_user_id` | BIGINT UNSIGNED | 被关注者 |
| `created_at` | DATETIME(3) | |
| `updated_at` | DATETIME(3) | |

唯一索引：`uk_from_to (from_user_id, to_user_id)` — 保证同一对用户只有一条记录。
查询索引：`idx_from_created (from_user_id, created_at, to_user_id)` — "我关注了谁" 游标分页。

- 关注 = `INSERT ... ON DUPLICATE KEY UPDATE`
- 取关 = `DELETE`（物理删除，不保留软删行）

### `follower` — 粉丝投影表（异步写入，最终一致）

| 列 | 类型 | 说明 |
|---|---|---|
| `id` | BIGINT UNSIGNED PK | 使用 `following.id`（aggregate_id） |
| `to_user_id` | BIGINT UNSIGNED | 被关注者（粉丝列表查询维度） |
| `from_user_id` | BIGINT UNSIGNED | 粉丝 |
| `created_at` | DATETIME(3) | 来自事件 `occurred_at` |
| `updated_at` | DATETIME(3) | |
| `last_event_id` | VARCHAR(128) NULL | 幂等去重 |

唯一索引：`uk_to_from (to_user_id, from_user_id)` — Consumer 通过此键做 `ON DUPLICATE KEY UPDATE`。
查询索引：`idx_to_created (to_user_id, created_at, from_user_id)` — "谁关注了我" 游标分页。

- 由 Kafka Consumer 异步写入
- `last_event_id` 用于消费幂等：仅当新事件 ID > 已存事件 ID 时才更新
- Consumer 使用 `ON DUPLICATE KEY UPDATE`，**不更新 id 字段**，保持首次写入的 ID

### `outbox` — 事件表（Canal Binlog 来源）

| 列 | 类型 | 说明 |
|---|---|---|
| `id` | BIGINT UNSIGNED PK | Snowflake ID |
| `aggregate_type` | VARCHAR(64) | `FOLLOW` / `UNFOLLOW` |
| `aggregate_id` | BIGINT UNSIGNED | `following.id` |
| `type` | VARCHAR(64) | `USER_RELATION_CHANGE` |
| `payload` | JSON | 事件详情 |
| `created_at` | TIMESTAMP(3) | |

索引：`ix_outbox_agg (aggregate_type, aggregate_id)`，`ix_outbox_ct (created_at)`。

### Payload JSON Schema

```json
{
  "event_id": "11398677092372481",
  "from_user_id": "11398675146215424",
  "to_user_id": "11398675603394560",
  "occurred_at": "2026-06-01T10:54:16.396Z"
}
```

`event_id` 是另一个 Snowflake ID，用于 Consumer 端幂等去重。

---

## API

所有接口需认证（JWT Bearer Token）。

| 方法 | 路径 | 说明 |
|---|---|---|
| `POST` | `/api/v1/users/:to_user_id/follow` | 关注用户（需 `Idempotency-Key` 头） |
| `POST` | `/api/v1/users/:to_user_id/unfollow` | 取关用户（需 `Idempotency-Key` 头） |
| `GET` | `/api/v1/users/:user_id/following?cursor=&limit=20` | "我关注了谁" 列表（走 `following` 主表，强一致） |
| `GET` | `/api/v1/users/:user_id/followers?cursor=&limit=20` | "谁关注了我" 列表（走 `follower` 投影表，最终一致） |

### 游标分页

两个列表接口均使用 `(created_at, user_id)` 组合游标，Base64 编码。响应中 `next_cursor` 为下一页游标，`next_cursor` 为空时表示已到尾页。

### 互关状态

粉丝列表返回的每个条目包含 `is_mutual` 字段，通过批量查询 `following` 主表计算。

### 幂等键

客户端必须在请求头携带 `Idempotency-Key: <UUID>`。服务端将首次成功响应缓存到 Redis（TTL 5 分钟），重复请求直接返回缓存结果。Redis 不可用时 fail-open，允许请求通过（Consumer 端幂等兜底）。

---

## 关键设计决策

### 物理删除而非软删除

`following` 和 `follower` 表均采用物理删除。关注 = INSERT，取关 = DELETE。理由：
- 不需要 `rel_status` 字段，简化并发逻辑
- 避免数据膨胀
- 审计需求由 Kafka 消息保留或归档消费者满足

### 并发控制：ON DUPLICATE KEY UPDATE

不预先 SELECT，不用 `SELECT ... FOR UPDATE`，不用分布式锁。直接 `INSERT ... ON DUPLICATE KEY UPDATE id = VALUES(id)`。并发请求中"最后写入者获胜"，`uk_from_to` 唯一索引保证只有一行。

### Consumer 幂等：last_event_id

```sql
-- 关注事件
INSERT INTO follower (id, to_user_id, from_user_id, created_at, updated_at, last_event_id)
VALUES (?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    updated_at = IF(last_event_id IS NULL OR last_event_id < VALUES(last_event_id), VALUES(updated_at), updated_at),
    last_event_id = IF(last_event_id IS NULL OR last_event_id < VALUES(last_event_id), VALUES(last_event_id), last_event_id);

-- 取关事件
DELETE FROM follower WHERE id = ? AND (last_event_id IS NULL OR last_event_id < ?);
```

`event_id` 是 Snowflake ID，词法序 ≈ 时间序。重复消息的条件判断结果为 false，`affected_rows = 0`，跳过。

### Kafka 分区策略

Topic `user_relation_events`，按 `to_user_id` 哈希分区。同一被关注者的所有事件进入同一分区，保证关注/取关顺序。

---

## 基础设施

### Docker Compose（4 容器）

| 容器 | 镜像 | 端口 |
|---|---|---|
| `ceddit-mysql` | `mysql:8.0` | 13306 |
| `ceddit-redis` | `redis:7-alpine` | 16379 |
| `ceddit-kafka` | `bitnami/kafka:3.9` (KRaft) | 9092 |
| `ceddit-canal` | `canal/canal-server:v1.1.7` | 11111, 11112 |

MySQL 额外参数：`--log-bin=mysql-bin --binlog-format=ROW --server-id=1 --binlog-row-image=FULL`

### Canal 配置

- `canal/conf/canal.properties` — Server 全局配置，`canal.serverMode = kafka`
- `canal/conf/example/instance.properties` — 实例配置，过滤 `ceddit\\.outbox`，投递到 `user_relation_events`

### Kafka

- KRaft 模式，无需 Zookeeper
- 8 分区，自动建 Topic
- Consumer Group：`follower-projection`

---

## 启动

```bash
# 首次启动 / 完全重置
make fresh

# 日常启动（保留数据）
make run
```

`make fresh` 执行流程：
1. `docker compose down -v` — 停止并删除所有容器和数据卷
2. `docker compose up -d` — 启动 MySQL + Redis + Kafka + Canal
3. `scripts/setup.sh` — 等待 MySQL 健康，授权 Canal 复制权限
4. `go build` — 编译
5. `./ceddit` — 启动（HTTP 服务 + Kafka Consumer 同进程）

单个二进制 `./ceddit` 同时运行 HTTP 服务器和 Kafka 消费者（goroutine）。

### 手动启动（分步骤）

```bash
# 1. 基础设施
make infra-up          # docker compose up -d

# 2. 授权 Canal（仅首次）
docker exec -it ceddit-mysql mysql -uroot -p040703 < sql/init/01-canal-privileges.sql

# 3. API 服务器 + Consumer
make run               # ./ceddit

# 4. 对账（另一终端，或 cron）
make run-reconciler    # ./ceddit-reconciler
```

---

## 代码结构

```
ceddit/
├── main.go                         # 入口：HTTP + consumer goroutine
├── service/
│   ├── consumer.go                 # Kafka 消费 + follower 投影写入
│   ├── relation.go                 # 关注/取关业务 + Redis 幂等缓存
│   └── ...
├── repository/mysql/
│   ├── relation.go                 # following + outbox 事务写入 + 分页查询
│   ├── mysql.go                    # +DB() 暴露 *sql.DB 给 consumer
│   └── ...
├── controller/relation.go          # 4 个 API Handler
├── models/relation.go              # Following, Follower, Outbox, 请求/响应类型
├── routes/routes.go                # 路由注册
├── cmd/
│   ├── consumer/main.go            # 独立消费程序（可选，已被主进程替代）
│   └── reconciler/main.go          # 对账任务（Redis 计数 + 僵尸行清理）
├── canal/conf/
│   ├── canal.properties            # Canal Server Kafka 模式配置
│   └── example/instance.properties # Instance 配置（订阅 ceddit.outbox）
├── docker-compose.yml              # MySQL + Redis + Kafka(KRaft) + Canal
├── config.yaml                     # 应用 + Kafka 配置
├── sql/
│   ├── example.sql                 # 迁移文件（含 following, follower, outbox）
│   └── init/01-canal-privileges.sql
├── scripts/setup.sh               # 预启动脚本
└── Makefile                        # make fresh / make run / make infra-*
```

---

## 待完善项

- [x] 基础关注/取关 API
- [x] Outbox 事务写入
- [x] Canal → Kafka → Consumer 链路
- [x] Consumer 幂等（last_event_id）
- [x] 请求幂等（Idempotency-Key + Redis）
- [x] 关注/粉丝列表（游标分页 + 互关判断）
- [x] 对账任务（reconciler）
- [ ] Redis 计数实时更新（目前由 reconciler 定时全量重算）
- [ ] Canal 分区键按 `to_user_id` 哈希的验证（当前 Canal 配置 `kafka.partition.hash = true` 需配合行级分区规则）
- [ ] 死信队列（DLQ Kafka topic）用于 Consumer 无法处理的消息
- [ ] 集成测试覆盖 Canal → Kafka → Consumer 完整链路
