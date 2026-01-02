# 多 Embedding 模型支持文档

本文档详细说明了 emomo 项目中多 Embedding 模型支持的实现细节。

## 目录

- [背景与目的](#背景与目的)
- [架构设计](#架构设计)
- [数据模型变化](#数据模型变化)
- [配置说明](#配置说明)
- [Ingest CLI 使用](#ingest-cli-使用)
- [Search API 使用](#search-api-使用)
- [代码变更详情](#代码变更详情)
- [最佳实践](#最佳实践)

---

## 背景与目的

### 需求背景

原有系统仅支持单一 Embedding 模型（Jina Embeddings），向量维度固定为 1024。随着需求发展，需要支持更多 Embedding 模型（如 Qwen3-Embedding-8B），这些模型具有不同的向量维度（如 4096）。

### 设计目标

1. **不影响原有数据**：现有的 Jina embedding 数据和功能完全保留
2. **多模型共存**：同一张图片可以被不同 Embedding 模型处理
3. **资源复用**：已上传的 S3 图片和 VLM 描述可复用，无需重复调用
4. **配置灵活**：通过配置文件和命令行参数选择 Embedding 模型

---

## 架构设计

### 整体架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Ingest CLI / API                             │
│                   --embedding=jina | qwen3                          │
└───────────────────────────────┬─────────────────────────────────────┘
                                │
                ┌───────────────┴───────────────┐
                │                               │
                ▼                               ▼
┌───────────────────────────┐   ┌───────────────────────────────────┐
│   JinaEmbeddingProvider   │   │ OpenAICompatibleEmbeddingProvider │
│   (jina-embeddings-v3)    │   │    (Qwen3-Embedding-8B)           │
│   dim: 1024               │   │    dim: 4096                      │
└───────────────┬───────────┘   └───────────────┬───────────────────┘
                │                               │
                ▼                               ▼
┌───────────────────────────┐   ┌───────────────────────────────────┐
│   Qdrant Collection       │   │      Qdrant Collection            │
│   "emomo"                 │   │   "emomo-qwen3-embedding-8b"      │
│   dim: 1024               │   │      dim: 4096                    │
└───────────────────────────┘   └───────────────────────────────────┘
                │                               │
                └───────────────┬───────────────┘
                                ▼
                ┌───────────────────────────────┐
                │          SQLite/PG            │
                │   memes + meme_vectors        │
                └───────────────────────────────┘
```

### 数据流程

#### 新图片处理流程

```
图片 → 计算 MD5 → 检查 meme_vectors(md5, collection) → 不存在
                                    ↓
                    VLM 生成描述 → S3 上传 → 创建 meme 记录
                                    ↓
                    Embedding 生成 → Qdrant 写入 → 创建 meme_vectors 记录
```

#### 已有图片新 Embedding 流程（资源复用）

```
图片 → 计算 MD5 → 检查 meme_vectors(md5, collection) → 不存在
                                    ↓
               按 MD5 查询 meme 记录 → 存在 → 复用 storage_key + vlm_description
                                    ↓
                    Embedding 生成 → Qdrant 写入 → 创建 meme_vectors 记录
```

---

## 数据模型变化

### 新增表：meme_vectors

用于记录 meme 与向量的关联关系，支持同一 meme 在不同 collection 中有多个向量。

```sql
CREATE TABLE meme_vectors (
    id              TEXT PRIMARY KEY,
    meme_id         TEXT NOT NULL,           -- 关联 memes.id
    md5_hash        TEXT NOT NULL,           -- 冗余存储，加速查询
    collection      TEXT NOT NULL,           -- Qdrant collection 名称
    embedding_model TEXT NOT NULL,           -- Embedding 模型名称
    qdrant_point_id TEXT NOT NULL,           -- Qdrant 中的 point ID
    status          TEXT DEFAULT 'active',   -- 状态：active, deleted
    created_at      TIMESTAMP,
    
    UNIQUE (md5_hash, collection)            -- 同一图片在同一 collection 只有一条
);

CREATE INDEX idx_meme_vectors_meme ON meme_vectors(meme_id);
```

### memes 表保持不变

原有 `memes` 表结构不变，保持向后兼容。

---

## 配置说明

### config.yaml 配置

```yaml
# 默认 Embedding 配置（向后兼容）
embedding:
  provider: jina
  model: jina-embeddings-v3
  dimensions: 1024
  collection: emomo

# 命名 Embedding 配置（新增）
embeddings:
  jina:
    provider: jina
    model: jina-embeddings-v3
    dimensions: 1024
    collection: emomo
  qwen3:
    provider: modelscope
    model: Qwen/Qwen3-Embedding-8B
    base_url: ""      # via MODELSCOPE_BASE_URL
    dimensions: 4096
    collection: emomo-qwen3-embedding-8b
```

### 环境变量

| 环境变量 | 说明 |
|----------|------|
| `JINA_API_KEY` | Jina API Key |
| `MODELSCOPE_API_KEY` | ModelScope API Key |
| `MODELSCOPE_BASE_URL` | ModelScope API Base URL |
| `EMBEDDING_BASE_URL` | 默认 Embedding Base URL |

### EmbeddingConfig 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `provider` | string | 提供商类型：`jina`, `modelscope`, `openai-compatible` |
| `model` | string | 模型名称/ID |
| `api_key` | string | API 认证密钥 |
| `base_url` | string | API 基础 URL（OpenAI 兼容 API 需要）|
| `dimensions` | int | 向量维度 |
| `collection` | string | 对应的 Qdrant collection 名称 |

---

## Ingest CLI 使用

### 命令格式

```bash
./ingest [options]
```

### 命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--source` | `chinesebqb` | 数据源，格式：`staging:<source_id>` 或 `chinesebqb` |
| `--limit` | `100` | 最大处理数量 |
| `--embedding` | `""` | Embedding 配置名称（如 `jina`, `qwen3`）|
| `--force` | `false` | 强制重新处理，跳过去重检查 |
| `--retry` | `false` | 重试 pending 状态的记录 |
| `--config` | `""` | 配置文件路径 |

### 使用示例

```bash
# 使用默认 Jina Embedding
./ingest --source=staging:fabiaoqing --limit=50

# 使用 Qwen3 Embedding
./ingest --source=staging:fabiaoqing --limit=50 --embedding=qwen3

# 强制重新处理（跳过去重检查）
./ingest --source=staging:fabiaoqing --limit=50 --embedding=qwen3 --force
```

### 输出日志示例

```json
{
  "level": "info",
  "source": "staging:fabiaoqing",
  "limit": 50,
  "embedding": "qwen3",
  "embedding_model": "Qwen/Qwen3-Embedding-8B",
  "embedding_dim": 4096,
  "qdrant_collection": "emomo-qwen3-embedding-8b",
  "msg": "Starting ingestion"
}
```

---

## Search API 使用

### POST /api/v1/search

完整查询接口，支持 JSON body。

**请求体：**

```json
{
  "query": "开心的表情",
  "top_k": 10,
  "collection": "qwen3",
  "category": "emoji",
  "is_animated": false,
  "source_type": "fabiaoqing"
}
```

**响应示例：**

```json
{
  "results": [
    {
      "id": "uuid-xxx",
      "url": "https://storage.example.com/xx/xxxxx.png",
      "score": 0.89,
      "description": "一个开心的表情...",
      "category": "emoji",
      "tags": ["开心", "笑"],
      "is_animated": false,
      "width": 256,
      "height": 256
    }
  ],
  "total": 1,
  "query": "开心的表情",
  "expanded_query": "",
  "collection": "qwen3"
}
```

### GET /api/v1/stats

获取统计信息，包括可用的 collections。

**响应示例：**

```json
{
  "total_active": 1000,
  "total_pending": 5,
  "total_categories": 10,
  "available_collections": ["emomo", "qwen3"]
}
```

---

## 代码变更详情

### 新增文件

| 文件 | 说明 |
|------|------|
| `internal/domain/meme_vector.go` | MemeVector 数据模型 |
| `internal/repository/meme_vector_repo.go` | MemeVector 数据库操作 |

### 修改文件

| 文件 | 修改说明 |
|------|----------|
| `internal/repository/db.go` | 添加 `MemeVector` 表自动迁移 |
| `internal/service/embedding.go` | 重构为接口化设计，添加 `EmbeddingProvider` 接口、多 Provider 实现 |
| `internal/config/config.go` | 添加 `Embeddings` map 配置、辅助方法 |
| `configs/config.yaml` | 添加 qwen3 Embedding 配置 |
| `internal/repository/qdrant_repo.go` | 向量维度动态化、添加辅助方法 |
| `internal/service/ingest.go` | 新去重逻辑、资源复用、支持 EmbeddingProvider 接口 |
| `cmd/ingest/main.go` | 添加 `--embedding` 参数 |
| `internal/service/search.go` | 支持多 Collection、`RegisterCollection` 方法 |
| `cmd/api/main.go` | 初始化多个 embedding provider |

### 接口变更

#### EmbeddingProvider 接口（新增）

```go
type EmbeddingProvider interface {
    Embed(ctx context.Context, text string) ([]float32, error)
    EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
    EmbedQuery(ctx context.Context, query string) ([]float32, error)
    GetModel() string
    GetDimensions() int
}
```

#### IngestService 构造函数变更

```go
// 旧版
func NewIngestService(
    memeRepo *repository.MemeRepository,
    qdrantRepo *repository.QdrantRepository,
    objectStorage storage.ObjectStorage,
    vlm *VLMService,
    embedding *EmbeddingService,  // 旧类型
    log *logger.Logger,
    cfg *IngestConfig,
) *IngestService

// 新版
func NewIngestService(
    memeRepo *repository.MemeRepository,
    vectorRepo *repository.MemeVectorRepository,  // 新增
    qdrantRepo *repository.QdrantRepository,
    objectStorage storage.ObjectStorage,
    vlm *VLMService,
    embedding EmbeddingProvider,  // 接口类型
    log *logger.Logger,
    cfg *IngestConfig,  // 增加 Collection 字段
) *IngestService
```

#### SearchService 构造函数变更

```go
// 新版支持 EmbeddingProvider 接口
func NewSearchService(
    memeRepo *repository.MemeRepository,
    qdrantRepo *repository.QdrantRepository,
    embedding EmbeddingProvider,  // 接口类型
    queryExpansion *QueryExpansionService,
    objectStorage storage.ObjectStorage,
    log *logger.Logger,
    cfg *SearchConfig,  // 增加 DefaultCollection 字段
) *SearchService

// 新增方法
func (s *SearchService) RegisterCollection(
    name string,
    qdrantRepo *repository.QdrantRepository,
    embedding EmbeddingProvider,
)
```

---

## 最佳实践

### 1. 环境变量管理

建议将敏感的 API Key 通过环境变量配置，而非写在配置文件中：

```bash
# .env 文件
JINA_API_KEY=jina_xxx
MODELSCOPE_API_KEY=sk-xxx
MODELSCOPE_BASE_URL=https://api-inference.modelscope.cn/v1
```

### 2. 分批导入策略

对于大量数据，建议分批导入：

```bash
# 先使用默认 Jina 导入所有数据
./ingest --source=staging:fabiaoqing --limit=1000

# 再使用 Qwen3 为已有数据生成新向量
./ingest --source=staging:fabiaoqing --limit=1000 --embedding=qwen3
```

### 3. Collection 命名规范

建议使用清晰的命名规范：

- `emomo` - 默认 collection（Jina）
- `emomo-<model>-<version>` - 其他模型，如 `emomo-qwen3-embedding-8b`

### 4. 监控与调试

启动 API 时会输出可用的 collections：

```json
{
  "port": 8080,
  "default_collection": "emomo",
  "available_collections": ["emomo", "qwen3"]
}
```

---

## 常见问题

### Q: 如何添加新的 Embedding 模型？

1. 在 `configs/config.yaml` 的 `embeddings` 下添加新配置
2. 如果是 OpenAI 兼容 API，设置 `provider: modelscope` 或 `provider: openai-compatible`
3. 确保 Qdrant 中已创建对应维度的 collection
4. 重启 API 服务或使用 CLI 导入

### Q: 如何查看某个图片在哪些 collection 中有向量？

查询 `meme_vectors` 表：

```sql
SELECT collection, embedding_model, created_at 
FROM meme_vectors 
WHERE meme_id = 'xxx';
```

### Q: 资源复用的条件是什么？

- S3 图片路径按 MD5 生成，相同图片自动复用
- VLM 描述从已有 meme 记录复用（按 MD5 匹配）
- 只有 Embedding 向量会重新生成

### Q: 如何删除某个 collection 的所有向量？

1. 删除 Qdrant collection：`curl -X DELETE "http://localhost:6333/collections/emomo-qwen3-embedding-8b"`
2. 删除数据库记录：`DELETE FROM meme_vectors WHERE collection = 'emomo-qwen3-embedding-8b'`
