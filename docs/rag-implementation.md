# 基于 RAG 的单篇知文知识问答实现与原理

> 实现日期：2026-06-01
> 语言：Go 1.26 + Gin + go-openai + Elasticsearch 8.x

## 一、概述

在「ceddit」项目中，用户在阅读一篇知文（Post）时，可以直接就文章内容发问，获得结合原文上下文的智能回答。后端实现了一套基于 **RAG（Retrieval-Augmented Generation，检索增强生成）** 的单篇知识问答方案。

### 核心流程

```
用户提问 → 确保文章已索引 → ES 向量检索相关片段 → 拼接为 Prompt → DeepSeek 流式生成 → SSE 返回
```

### 架构特点

- **向量存储**：Elasticsearch 8.x `dense_vector`（HNSW + 余弦相似度），生产可用
- **优雅降级**：ES 不可用时自动回退到内存向量存储
- **零重依赖**：ES 交互使用 `net/http` 标准库直调 REST API，编译轻量快速
- **流式输出**：DeepSeek Chat API 流式模式 + Gin SSE，前端实时看到逐字回答

---

## 二、新增 / 修改文件清单

### 新增文件

| 文件 | 说明 |
|------|------|
| `pkg/rag/types.go` | 核心类型 + `VectorStore` 接口 + ES JSON 结构 |
| `pkg/rag/chunker.go` | Markdown 分块（按标题切段 + 800 字切片 + 100 字重叠） |
| `pkg/rag/chunker_test.go` | 分块器单元测试（10 个用例） |
| `pkg/rag/embedder.go` | Embedding API 客户端（OpenAI 兼容协议） |
| `pkg/rag/vectorstore.go` | `MemoryVectorStore` — 内存向量存储，实现 `VectorStore` 接口 |
| `pkg/rag/vectorstore_test.go` | 向量存储单元测试（9 个用例） |
| `pkg/rag/es_store.go` | `ESVectorStore` — ES 向量存储，`net/http` 直调 REST API |
| `pkg/rag/indexer.go` | 索引管理（指纹检测、构建、幂等重建） |
| `pkg/rag/indexer_test.go` | 索引器单元测试（6 个用例） |
| `pkg/rag/qa.go` | Q&A 服务（检索 + Prompt 构造 + DeepSeek 流式生成） |
| `service/rag.go` | 服务编排（`InitRAG`、`EnsurePostIndexed`、`StreamPostAnswer`） |
| `controller/rag.go` | HTTP SSE 流式问答接口处理器 |

### 修改文件

| 文件 | 变更 |
|------|------|
| `routes/routes.go` | 新增 `GET /api/v1/posts/:id/qa/stream`（可选认证） |
| `main.go` | 新增 `service.InitRAG()` 初始化 |
| `config.yaml` | 新增 `elasticsearch` 和 `rag.embedding` 配置节 |
| `docker-compose.yml` | 新增 Elasticsearch 8.17 服务（低内存配置 256MB heap） |
| `service/post.go` | `ConfirmContent` / `PublishPost` 中异步预索引触发 |

---

## 三、架构设计

### 3.1 索引构建

**触发时机：**

1. **预索引阶段**：知文内容确认（`ConfirmContent`）或发布（`PublishPost`）时，后台 goroutine 异步构建索引。
2. **问答兜底阶段**：用户调用问答接口时，再次执行 `EnsureIndexed`，若此前未成功索引可兜底重建。

**分块策略（两级）：**

1. **按 Markdown 标题切段**：每遇到 `#` 开头的标题行即为新段落，避免跨节语义污染。
2. **按长度细分 + 重叠**：每个段落再按 ≤800 字符切割，块间 100 字符重叠保证语义连续性。

**指纹检测与幂等：**

- 以 `metadata.post_id` 查询 ES 中已索引文档
- 对比 `ContentSHA256` / `ContentETag` 与当前内容
- 一致则跳过，不一致则 `_delete_by_query` 删除旧分块后重新导入

### 3.2 向量存储：Elasticsearch dense_vector

使用 **Elasticsearch 8.x** 作为向量存储，索引名 `zhiguang-ai-index`。

**索引 Mapping：**

```json
{
  "mappings": {
    "properties": {
      "text": { "type": "text" },
      "embedding": {
        "type": "dense_vector",
        "dims": 1536,
        "similarity": "cosine",
        "index": true,
        "index_options": {
          "type": "hnsw",
          "m": 16,
          "ef_construction": 200
        }
      },
      "metadata": {
        "properties": {
          "post_id":       { "type": "long" },
          "chunk_id":      { "type": "keyword" },
          "position":      { "type": "integer" },
          "title":         { "type": "text" },
          "content_url":   { "type": "keyword" },
          "content_sha256": { "type": "keyword" },
          "content_etag":  { "type": "keyword" }
        }
      }
    }
  }
}
```

**向量检索（KNN）：**

```json
{
  "knn": {
    "field": "embedding",
    "query_vector": [...],
    "k": 5,
    "num_candidates": 20,
    "filter": {
      "term": { "metadata.post_id": 123 }
    }
  }
}
```

- `num_candidates = max(topK * 3, 20)`，先宽召回再截断
- `filter` 按 `post_id` 过滤，确保只检索当前文章，避免跨帖泄露

**ES 客户端：** 使用 Go 标准库 `net/http` + `encoding/json` 直调 ES REST API，不依赖任何第三方 ES 库。这避免了 `go-elasticsearch` 引入的 OpenTelemetry 等重依赖，在低内存服务器（1-2GB）上也能正常编译。

**优雅降级：** `service.InitRAG()` 启动时尝试连接 ES，若 ES 不可用则自动回退到 `MemoryVectorStore`（内存存储），系统功能不受影响。

### 3.3 检索与 Prompt 构造

1. **扩大召回**：`fetchK = max(topK * 3, 20)` — 先拉回更多候选
2. **PostID 过滤**：ES 查询中 `filter: term { metadata.post_id }` 确保只检索当前知文
3. **TopK 截断**：取相似度最高的 topK 条作为上下文
4. **Prompt 结构**：
   - **System**：限定角色 + "只看上下文、不确定就说不确定"的反幻觉约束
   - **User**：问题 + 带标题和编号的上下文片段（`---` 分隔）

### 3.4 流式输出

- DeepSeek Chat API `stream: true` 模式
- Gin `c.Writer.Flush()` 实现 SSE 推送
- `Temperature=0.2`，低随机性保证回答稳定
- 超时 5 分钟，支持客户端断连自动清理

---

## 四、API 接口

### GET /api/v1/posts/{id}/qa/stream

**认证**：可选（公开已发布文章允许匿名问答）

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| question | string | 是 | - | 用户问题 |
| topK | int | 否 | 5 | 检索上下文片段数（1-20） |
| maxTokens | int | 否 | 1024 | 回答最大 token 数（1-4096） |

**响应**：`text/event-stream`

```bash
curl -N "http://localhost:8081/api/v1/posts/1234567890/qa/stream?question=阿司匹林有什么副作用"
```

```
data: 根据
data: 上下文
data: ...
data: [DONE]
```

---

## 五、配置说明

```yaml
# config.yaml

deepseek:
  api_key: "sk-xxx"
  base_url: "https://api.deepseek.com"
  model: "deepseek-v4-flash"

# Elasticsearch 向量存储
elasticsearch:
  host: "127.0.0.1"
  port: 9200
  index: "zhiguang-ai-index"
  username: ""
  password: ""

# RAG Embedding（回退到 deepseek 配置）
rag:
  embedding:
    # base_url: ""       # 可选，默认用 deepseek 的 base_url
    # api_key: ""         # 可选，默认用 deepseek 的 api_key
    model: "text-embedding-v4"
```

**docker-compose 中的 ES：**

```yaml
elasticsearch:
  image: docker.elastic.co/elasticsearch/elasticsearch:8.17.0
  environment:
    - discovery.type=single-node
    - xpack.security.enabled=false
    - xpack.ml.enabled=false
    - "ES_JAVA_OPTS=-Xms256m -Xmx256m"
  mem_limit: 512m
```

> **小内存服务器提示**：如果服务器内存不足，可以注释掉 ES 服务 —— RAG 系统会自动回退到内存向量存储，功能正常使用。

---

## 六、测试

```bash
go test ./pkg/rag/ -v
```

25 个用例全部通过：

| 模块 | 用例数 | 覆盖内容 |
|------|--------|----------|
| Chunker | 7 | 标题切分、短/空/长文本、重叠验证、标题提取 |
| VectorStore | 9 | 增删查、PostID 过滤、TopK 截断、余弦相似度 |
| Indexer | 6 | 索引构建、空 URL 跳过、草稿跳过、指纹跳过、变更重建 |
| Search/Build | 3 | 上下文检索、Prompt 构造 |

---

## 七、设计要点总结

1. **把知文正文结构化拆解** → 按标题 → 段落 → 小块组织，写入 ES dense_vector 索引
2. **ES KNN 向量检索 + post_id 过滤** → 精准定位当前文章的语义相关片段
3. **System Prompt 反幻觉约束** → "只看上下文，不确定就说不确定"
4. **异步预索引** → 发布时后台构建，减少首次问答冷启动
5. **SHA256/ETag 指纹跳过** → 内容未变时无需重建
6. **ES 不可用时自动降级** → 内存向量存储兜底，系统不中断
7. **net/http 标准库直调 ES** → 零重依赖，低内存服务器编译无忧

---

## 八、后续优化方向

1. **混合检索**：BM25 关键词 + KNN 向量融合，提升召回率
2. **异步入队**：将预索引改为 Kafka 消息异步处理
3. **引用标注**：回答中标注引用来源片段，增强可信度
4. **多轮对话**：维护对话历史，支持追问
5. **ES 认证**：生产环境开启 xpack.security 并配置 TLS
