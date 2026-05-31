# 渐进式发布 (Progressive Publishing)

## 概念

传统发帖是一步到位：前端发 JSON → 后端写库 → 返回成功。如果过程中任何环节出问题（文件上传失败、内容违规、摘要生成错误），要么全成功要么全失败，排查和重试都很困难。

**渐进式发布**把这个过程拆成六个独立的、可重试的最小步骤。每一步只做一件事：

```
draft ──→ upload ──→ verify ──→ metadata ──→ publish
  │          │          │           │            │
  └── 任一步失败，帖子停留在当前阶段，从失败处重试 ──┘
```

## 用户视角：操作与后台发生了什么

### 第一步：创建草稿

**用户操作**：点击"写文章"按钮。

**前端发送**：

```
POST /api/v1/post/draft
(空 body，仅携带 JWT)
```

**后端处理**：生成雪花 ID → 插入一条 `status='draft'` 的空记录。

**返回**：`{"post_id": "10753760422793216"}`

**此时数据库**：

```
| id | title | status | author_id | content_* |
|----|-------|--------|-----------|-----------|
| 107... | NULL | draft  | 10001     | 全是 NULL  |
```

**如果失败**：重试一次即可，不会有脏数据。

---

### 第二步：申请上传凭证

**用户操作**：在 Markdown 编辑器中开始写作。

**何时触发**：用户每拖入一张图片、或点"准备发布"时。

**前端发送（正文）**：

```
POST /api/v1/storage/presign
{
  "scene": "post_content",
  "post_id": "10753760422793216",
  "content_type": "text/markdown",
  "ext": ".md"
}
```

**前端发送（图片）**：

```
POST /api/v1/storage/presign
{
  "scene": "post_image",
  "post_id": "10753760422793216",
  "content_type": "image/png",
  "ext": ".png"
}
```

**后端处理**：验证帖子属于当前用户 → 调用腾讯云 COS SDK 生成预签名 PUT URL（有效期 10 分钟）。

**返回**：

```json
{
  "object_key": "posts/10753760422793216/content.md",
  "put_url": "https://ceddit-xxx.cos.ap-nanjing.myqcloud.com/posts/107...?q-sign-algorithm=sha1&...",
  "expires_in": 600
}
```

**`scene` 与 COS 路径**：

| scene | COS 路径 | 用途 |
|-------|---------|------|
| `post_content` | `posts/{post_id}/content.md` | 正文文件，每个帖子固定一个 |
| `post_image` | `posts/{post_id}/images/{snowflake_id}.png` | 图片，每张唯一 ID 防碰撞 |

**如果失败**：重新申请一个新 URL，旧的 URL 过期后自动失效。

---

### 第三步：上传文件到 COS（不经过后端）

**用户操作**：点了"准备发布"（正文），或者拖入图片时自动触发（图片）。

**前端操作**：

```
PUT {put_url}
Content-Type: text/markdown  （或 image/png）

Body: <文件的二进制内容>
```

**COS 返回**：`200 OK`，响应头包含 `ETag: "abc123..."`。

**关键**：这一步数据流是 前端 → COS，完全不经过你的后端服务器。后端只签发了"临时通行证"，不知道文件内容。这省了服务器带宽，大文件也不怕。

**前端需要记住的**：`ETag`（从响应头取）、`SHA256`（上传前自己算的）、`文件字节数`（上传前自己算的）。

**如果失败**：重新 PUT，预签名 URL 在 10 分钟内有效。

---

### 第四步：内容校验

**用户操作**：无（前端自动触发，在上传完成后）。

**前端发送**：

```
POST /api/v1/post/10753760422793216/content/confirm
{
  "object_key": "posts/10753760422793216/content.md",
  "etag": "\"abc123def456\"",
  "size": 202,
  "sha256": "7177e0983f642916cf4c4ae745d2241f84029f2cf5ee73eb0e6e16223364ce37"
}
```

**后端处理**：

```
① HEAD Object → COS 返回 ETag="abc123def456", Size=202
② 对比: ETag 一致? ✓   Size 一致? ✓
③ GET Object → 下载文件 → 算 SHA256
④ 对比: SHA256 一致? ✓
⑤ 全部通过 → 把 object_key/etag/size/sha256 写入 MySQL
```

**返回**：`204 No Content`

**如果校验失败**：返回 400 并说明哪个环节不一致（ETag / Size / SHA256），帖子保持 draft 状态，前端重新上传。

**这个步骤的价值**：防止文件传输时损坏、防止前端谎报、确认 COS 上真有文件。

---

### 第五步：完善标题

**用户操作**：在编辑器中填好标题，点击"完善信息"或直接点"发布"。

**前端发送**：

```
PATCH /api/v1/post/10753760422793216
{
  "title": "Go 并发编程指南"
}
```

**后端处理**：UPDATE `title` 字段。

**返回**：`204 No Content`

**为什么标题和正文分开提交**：正文已经在 COS 上了（步骤 3-4），标题是纯文本字段在数据库。分开可以独立修改——改了标题不需要重新上传文件。

---

### 第六步：正式发布

**用户操作**：点击"发布"按钮。

**前端发送**：

```
POST /api/v1/post/10753760422793216/publish
(空 body)
```

**后端处理**：

```
① 检查前置条件:
   - title 不为空? ✓   (否则返回 "title is required")
   - 内容已校验?  ✓   (否则返回 "content must be confirmed")
② 从 COS 下载正文文件 (用 content_object_key)
③ 调用 DeepSeek API:
   prompt: "请用50字以内概括以下文章的内容：\n\n<正文内容>"
   → 返回: "Go并发编程核心是goroutine和channel..."
④ 写入 MySQL:
   UPDATE post SET status='published',
                   description='Go并发编程核心是...',
                   publish_time=NOW()
⑤ 写入 Redis: 将 post_id 加入时间线和评分有序集合
```

**返回**：`204 No Content`

**DeepSeek 调用失败时**：`description` 留空，帖子正常发布，不阻塞流程。

**此时帖子对外可见**，出现在 `/api/v1/post` 和 `/api/v1/easypost` 列表中。

---

## 读取帖子

**用户操作**：访问帖子详情页。

**前端发送**：

```
GET /api/v1/post/10753760422793216
```

**后端处理**：查 MySQL → 查 COS 取正文 → 组装返回。

**返回**：

```json
{
  "post_id": "10753760422793216",
  "title": "Go 并发编程指南",
  "description": "Go并发编程核心是goroutine和channel...",
  "content": "## Go 并发\n\nGoroutine 是轻量级线程...",
  "author_name": "alice",
  "vote_num": 0,
  "status": "published",
  "content_object_key": "posts/10753760422793216/content.md",
  "content_etag": "\"abc123...\"",
  "content_size": 202,
  "content_sha256": "7177e098...",
  "create_time": "2026-05-30T16:11:36Z",
  "publish_time": "2026-05-30T16:11:38Z"
}
```

注意 `content` 字段是后端从 COS 取出后直接返回的文字内容，不是 URL。前端不需要关心 COS。

---

## 数据库表

```sql
CREATE TABLE `post` (
    `id` bigint(20) NOT NULL,                     -- 雪花ID
    `title` varchar(256) DEFAULT NULL,            -- 标题 (步骤⑤)
    `description` varchar(256) DEFAULT NULL,      -- AI摘要 (步骤⑥)
    `content_object_key` varchar(512) DEFAULT NULL,-- COS路径 (步骤④)
    `content_etag` varchar(128) DEFAULT NULL,     -- COS ETag (步骤④)
    `content_size` bigint(20) unsigned DEFAULT NULL, -- 文件大小 (步骤④)
    `content_sha256` char(64) DEFAULT NULL,       -- SHA256 (步骤④)
    `author_id` bigint(20) NOT NULL,
    `status` varchar(16) NOT NULL DEFAULT 'draft',-- draft | published
    `create_time` timestamp NOT NULL,
    `publish_time` timestamp NULL,
    `update_time` timestamp NOT NULL,
    PRIMARY KEY (`id`)
);
```

## COS 目录结构

```
ceddit-1325808776 (桶)
└── posts/
    └── {post_id}/
        ├── content.md           ← 正文
        └── images/
            ├── 10755713093603328.png  ← 图片1
            ├── 10755713202655232.jpg  ← 图片2
            └── ...
```

## 配置项

```yaml
cos:
  secret_id: "AKID..."          # 腾讯云 API 密钥
  secret_key: "..."
  region: "ap-nanjing"          # COS 地域
  bucket: "ceddit-1325808776"   # 存储桶名称

deepseek:
  api_key: "sk-..."             # DeepSeek API Key
  base_url: "https://api.deepseek.com"
  model: "deepseek-v4-flash"

mysql:
  auto_migrate: true            # 启动时自动建表（开发环境）
  migrate_file: "sql/example.sql"
```

## 状态流转

```
                    ┌──────────┐
                    │  draft   │  ← ① 创建草稿
                    └────┬─────┘
                         │ ② ③ ④ 上传+校验成功
                         │ ⑤ 补标题
                         ▼
                    ┌──────────┐
                    │ draft    │  (内容已就绪，title已填)
                    └────┬─────┘
                         │ ⑥ 发布
                         ▼
                    ┌──────────┐
                    │published │  对外可见
                    └──────────┘
```

任何步骤失败 → 状态不推进 → 用户从失败步骤重试。没有"半死状态"。

## API 总览

| 方法 | 路径 | 步骤 | 认证 | 说明 |
|------|------|------|------|------|
| POST | `/api/v1/post/draft` | ① | JWT | 创建空草稿 |
| POST | `/api/v1/storage/presign` | ② | JWT | 获取 COS 上传 URL |
| PUT | `{presigned_url}` | ③ | 无 | 前端直传 COS |
| POST | `/api/v1/post/:id/content/confirm` | ④ | JWT | 校验文件完整性 |
| PATCH | `/api/v1/post/:id` | ⑤ | JWT | 补标题 |
| POST | `/api/v1/post/:id/publish` | ⑥ | JWT | 正式发布 |
| GET | `/api/v1/post/:id` | — | JWT | 帖子详情（含正文） |
| GET | `/api/v1/post` | — | JWT | 已发布列表 |
| GET | `/api/v1/easypost` | — | JWT | 简易列表 |
