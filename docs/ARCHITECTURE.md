# Emomo 项目架构深度分析

本文档对 Emomo 项目的整体架构进行全面梳理，涵盖前后端、数据流、核心模块及设计决策。

## 目录

- [一、项目概述](#一项目概述)
- [二、系统架构图](#二系统架构图)
- [三、后端架构](#三后端架构-emomo)
- [四、数据模型](#四数据模型)
- [五、前端架构](#五前端架构-emomo-frontend)
- [六、Python 爬虫](#六python-爬虫-crawler)
- [七、外部依赖](#七外部依赖)
- [八、部署架构](#八部署架构)
- [九、关键设计决策](#九关键设计决策)
- [十、相关文档](#十相关文档)

---

## 一、项目概述

**Emomo** 是一个 **AI 表情包语义搜索系统**，采用前后端分离架构，支持通过自然语言描述搜索表情包。

| 组件 | 技术栈 | 目录 |
|------|--------|------|
| **后端** | Go + Gin + GORM | `emomo/` |
| **前端** | React + TypeScript + Vite | `emomo-frontend/` |
| **Python 爬虫** | Python + uv | `emomo/crawler/` |

### 核心功能

- **语义搜索**：输入文字描述即可检索相似表情包
- **多源摄入**：支持本地仓库、Python 爬虫、分批摄入
- **向量管理**：支持多 Embedding 模型/多集合管理
- **存储抽象**：兼容 Cloudflare R2、AWS S3 与其他 S3 兼容服务
- **查询理解**：LLM 驱动的查询扩展与意图识别

---

## 二、系统架构图

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          Frontend (React + Vite)                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐ │
│  │  SearchHero  │  │   MemeGrid   │  │   MemeModal  │  │SearchProgress│ │
│  └──────┬───────┘  └──────────────┘  └──────────────┘  └──────────────┘ │
│         │ HTTP/SSE                                                       │
└─────────┼───────────────────────────────────────────────────────────────┘
          │
          ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                      Go Backend (Gin + GORM)                             │
│                                                                          │
│  ┌───────────────────────────────────────────────────────────────────┐  │
│  │                         API Layer (Gin)                            │  │
│  │   /api/v1/search  │  /api/v1/memes  │  /api/v1/categories         │  │
│  └───────────────────────────────────────────────────────────────────┘  │
│                                │                                         │
│  ┌───────────────────────────────────────────────────────────────────┐  │
│  │                       Service Layer                                │  │
│  │  SearchService │ IngestService │ VLMService │ EmbeddingRegistry   │  │
│  └───────────────────────────────────────────────────────────────────┘  │
│                                │                                         │
│  ┌───────────────────────────────────────────────────────────────────┐  │
│  │                     Repository Layer (GORM)                        │  │
│  │  MemeRepo │ MemeVectorRepo │ MemeDescriptionRepo │ QdrantRepo     │  │
│  └───────────────────────────────────────────────────────────────────┘  │
└─────────────────────┬─────────────────────┬─────────────────────────────┘
                      │                     │
         ┌────────────┼─────────────────────┼────────────────┐
         ▼            ▼                     ▼                ▼
┌─────────────┐ ┌──────────────┐ ┌─────────────────┐ ┌─────────────────┐
│  SQLite/    │ │   Qdrant     │ │   S3/R2         │ │  VLM/Embedding  │
│  PostgreSQL │ │  (向量库)    │ │  (对象存储)      │ │  (外部 API)     │
└─────────────┘ └──────────────┘ └─────────────────┘ └─────────────────┘
```

---

## 三、后端架构 (`emomo/`)

### 3.1 目录结构

```
emomo/
├── cmd/
│   ├── api/main.go           # API 服务入口
│   └── ingest/main.go        # 数据导入 CLI 入口
├── internal/
│   ├── api/                  # API 层
│   │   ├── router.go         # 路由配置
│   │   ├── handler/          # HTTP Handlers
│   │   └── middleware/       # 中间件 (CORS, Logger)
│   ├── config/               # 配置管理 (Viper)
│   ├── domain/               # 领域模型
│   │   ├── meme.go           # Meme 实体
│   │   ├── meme_vector.go    # 向量关联实体
│   │   └── meme_description.go # 描述实体
│   ├── repository/           # 数据访问层
│   │   ├── meme_repo.go      # Meme CRUD
│   │   ├── meme_vector_repo.go
│   │   └── qdrant_repo.go    # Qdrant 操作
│   ├── service/              # 业务逻辑层
│   │   ├── search.go         # 搜索服务
│   │   ├── ingest.go         # 数据导入服务
│   │   ├── vlm.go            # VLM 图片描述
│   │   ├── embedding_registry.go  # 多 Embedding 管理
│   │   └── query_understanding.go # 查询理解
│   ├── source/               # 数据源适配器
│   │   ├── chinesebqb/       # ChineseBQB 仓库
│   │   └── staging/          # 爬虫 staging 数据
│   ├── storage/              # 对象存储抽象
│   │   └── s3.go             # S3/R2 实现
│   ├── logger/               # 结构化日志
│   └── prompts/              # LLM Prompt 模板
├── configs/config.yaml       # 配置文件
├── migrations/               # 数据库迁移文件
├── scripts/                  # 辅助脚本
└── crawler/                  # Python 爬虫
```

### 3.2 分层架构

```
┌─────────────────────────────────────────────────────────────────┐
│  Handler Layer (internal/api/handler/)                          │
│  - 处理 HTTP 请求/响应                                           │
│  - 参数验证、错误处理                                             │
│  - 调用 Service 层                                               │
└──────────────────────────────┬──────────────────────────────────┘
                               │
┌──────────────────────────────▼──────────────────────────────────┐
│  Service Layer (internal/service/)                              │
│  - 业务逻辑编排                                                   │
│  - 跨 Repository 事务协调                                        │
│  - 外部 API 调用 (VLM, Embedding)                                │
└──────────────────────────────┬──────────────────────────────────┘
                               │
┌──────────────────────────────▼──────────────────────────────────┐
│  Repository Layer (internal/repository/)                        │
│  - 数据库 CRUD 操作 (GORM)                                       │
│  - Qdrant 向量操作 (gRPC)                                        │
│  - S3/R2 存储操作                                                │
└─────────────────────────────────────────────────────────────────┘
```

### 3.3 核心 API 端点

| 端点 | 方法 | 功能 |
|------|------|------|
| `POST /api/v1/search` | 普通搜索 | 语义搜索表情包 |
| `POST /api/v1/search/stream` | SSE 流式搜索 | 带进度的流式搜索 |
| `GET /api/v1/memes` | 列表 | 分页获取表情包 |
| `GET /api/v1/memes/:id` | 详情 | 获取单个表情包 |
| `GET /api/v1/categories` | 分类 | 获取所有分类 |
| `GET /api/v1/stats` | 统计 | 系统统计信息 |
| `POST /api/v1/ingest` | 导入 | 触发数据导入 |
| `GET /api/v1/ingest/status` | 状态 | 导入任务状态 |

### 3.4 搜索流程 (核心)

```
用户输入查询 "开心的猫"
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 1. Query Understanding (可选)                                  │
│    - LLM 分析用户意图                                          │
│    - 生成 QueryPlan: 语义查询、关键词、同义词、过滤条件         │
│    输出: "表达开心情绪的猫咪表情包" + keywords: [开心, 猫]      │
└───────────────────────────────┬───────────────────────────────┘
                                │
                                ▼
┌───────────────────────────────────────────────────────────────┐
│ 2. Embedding Generation                                        │
│    - 调用 Jina/OpenAI Embedding API                            │
│    - 生成 1024/4096 维向量                                     │
└───────────────────────────────┬───────────────────────────────┘
                                │
                                ▼
┌───────────────────────────────────────────────────────────────┐
│ 3. Hybrid Search (Qdrant)                                      │
│    - Dense Search: 向量余弦相似度                               │
│    - Sparse Search: BM25 关键词匹配                            │
│    - RRF 融合排序                                              │
└───────────────────────────────┬───────────────────────────────┘
                                │
                                ▼
┌───────────────────────────────────────────────────────────────┐
│ 4. Result Enrichment                                           │
│    - 从 PostgreSQL/SQLite 获取 width, height 等元数据          │
│    - 生成 storage_url                                          │
└───────────────────────────────┬───────────────────────────────┘
                                │
                                ▼
                          返回搜索结果
```

### 3.5 数据导入流程 (Ingest)

```
数据源 (ChineseBQB / Crawler Staging)
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ Worker Pool (并发处理)                                         │
│ ┌─────────────────────────────────────────────────────────┐   │
│ │ processItem() 单个 item 处理流程:                        │   │
│ │                                                          │   │
│ │ 1. 读取图片 → 计算 MD5                                   │   │
│ │ 2. 检查 meme_vectors(MD5, Collection) 是否存在 → 去重    │   │
│ │ 3. 检查 memes(MD5) 是否存在 → 复用 S3 路径和 VLM 描述    │   │
│ │ 4. [新数据] 上传到 S3/R2                                 │   │
│ │ 5. [新数据] 调用 VLM 生成图片描述                        │   │
│ │ 6. 调用 Embedding API 生成向量                           │   │
│ │ 7. Upsert 到 Qdrant                                      │   │
│ │ 8. 保存 meme + meme_vectors 记录                         │   │
│ └─────────────────────────────────────────────────────────┘   │
└───────────────────────────────────────────────────────────────┘
```

> 详细导入架构请参考 [DATA_INGEST_ARCHITECTURE.md](./DATA_INGEST_ARCHITECTURE.md)

---

## 四、数据模型

### 4.1 双数据库架构

```
┌─────────────────────────────────────────────────────────────────┐
│                     关系型数据库 (SQLite/PostgreSQL)             │
│                                                                  │
│  ┌────────────────────┐     ┌────────────────────┐              │
│  │       memes        │     │   meme_descriptions │              │
│  ├────────────────────┤     ├────────────────────┤              │
│  │ id (PK)            │     │ id (PK)            │              │
│  │ source_type        │────▶│ meme_id (FK)       │              │
│  │ source_id          │     │ md5_hash           │              │
│  │ storage_key        │     │ vlm_model          │              │
│  │ md5_hash (UK)      │     │ description        │              │
│  │ width, height      │     │ ocr_text           │              │
│  │ format, is_animated│     └────────────────────┘              │
│  │ category, tags     │                                         │
│  │ status             │     ┌────────────────────┐              │
│  └────────┬───────────┘     │    meme_vectors    │              │
│           │                 ├────────────────────┤              │
│           └────────────────▶│ id (PK)            │              │
│                             │ meme_id (FK)       │              │
│                             │ md5_hash           │              │
│                             │ collection (UK)    │              │
│                             │ embedding_model    │              │
│                             │ qdrant_point_id    │              │
│                             └─────────┬──────────┘              │
└───────────────────────────────────────┼─────────────────────────┘
                                        │
                                        ▼
┌───────────────────────────────────────────────────────────────┐
│                     Qdrant (向量数据库)                        │
│                                                                │
│  Collection: emomo (dim: 1024)                                 │
│  Collection: emomo-qwen3-embedding-8b (dim: 4096)             │
│                                                                │
│  Point Structure:                                              │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │ point_id: UUID                                            │ │
│  │ vector: [float32 x 1024/4096]                            │ │
│  │ payload: {                                                │ │
│  │   meme_id, source_type, category,                        │ │
│  │   is_animated, tags, vlm_description,                    │ │
│  │   ocr_text, storage_url                                  │ │
│  │ }                                                         │ │
│  └──────────────────────────────────────────────────────────┘ │
└───────────────────────────────────────────────────────────────┘
```

### 4.2 核心表结构

#### memes 表
| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | TEXT (PK) | UUID 主键 |
| `source_type` | TEXT | 数据来源类型 |
| `source_id` | TEXT | 来源内唯一 ID |
| `storage_key` | TEXT | S3/R2 存储路径 |
| `md5_hash` | TEXT (UK) | 图片内容 MD5 |
| `width`, `height` | INT | 图片尺寸 |
| `format` | TEXT | 图片格式 |
| `is_animated` | BOOL | 是否动图 |
| `category` | TEXT | 分类 |
| `tags` | TEXT (JSON) | 标签数组 |
| `status` | TEXT | 状态: pending/active/failed |

#### meme_vectors 表
| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | TEXT (PK) | UUID 主键 |
| `meme_id` | TEXT (FK) | 关联 meme |
| `md5_hash` | TEXT | 图片 MD5 |
| `collection` | TEXT | Qdrant collection 名 |
| `embedding_model` | TEXT | Embedding 模型名 |
| `qdrant_point_id` | TEXT | Qdrant 点 ID |

> 详细数据库设计请参考 [DATABASE_SCHEMA.md](./DATABASE_SCHEMA.md)

### 4.3 多 Embedding 模型支持

系统支持配置多个 Embedding 模型，每个模型对应独立的 Qdrant Collection：

```yaml
# configs/config.yaml
embeddings:
  - name: jina
    provider: jina
    model: jina-embeddings-v3
    dimensions: 1024
    collection: emomo
    is_default: true
  
  - name: qwen3
    provider: modelscope
    model: Qwen/Qwen3-Embedding-8B
    dimensions: 4096
    collection: emomo-qwen3-embedding-8b
```

> 详细多 Embedding 设计请参考 [MULTI_EMBEDDING.md](./MULTI_EMBEDDING.md)

---

## 五、前端架构 (`emomo-frontend/`)

### 5.1 目录结构

```
emomo-frontend/
├── src/
│   ├── main.tsx              # 入口文件
│   ├── App.tsx               # 根组件
│   ├── api/index.ts          # API 客户端
│   ├── components/           # UI 组件
│   │   ├── Header.tsx        # 顶部导航
│   │   ├── SearchHero.tsx    # 搜索框
│   │   ├── MemeGrid.tsx      # 表情包网格
│   │   ├── MemeCard.tsx      # 表情包卡片
│   │   ├── MemeModal.tsx     # 表情包详情弹窗
│   │   └── SearchProgress.tsx # 搜索进度显示
│   ├── types/index.ts        # TypeScript 类型定义
│   └── data/curatedMemes.ts  # 预置推荐数据
├── e2e/                      # Playwright E2E 测试
└── vite.config.ts            # Vite 配置
```

### 5.2 组件架构

```
App.tsx
├── Header                    # Logo + 品牌
├── SearchHero                # 搜索输入框
├── SearchProgress            # 流式搜索进度 (AnimatePresence)
│   ├── Thinking 动画         # LLM 思考过程
│   └── Stage 提示            # 当前阶段
└── MemeGrid                  # 搜索结果 / 推荐表情
    ├── MemeCard[]            # 单个表情包卡片
    └── MemeModal             # 点击放大查看
```

### 5.3 流式搜索 (SSE)

前端支持 SSE 流式搜索，实时显示 LLM 思考过程：

```typescript
// api/index.ts
export async function searchMemesStream(
  query: string,
  topK: number,
  onProgress: (event: SearchProgressEvent) => void,
  signal?: AbortSignal
): Promise<void> {
  const response = await fetch(`${API_BASE}/search/stream`, {
    method: 'POST',
    body: JSON.stringify({ query, top_k: topK }),
  });
  
  const reader = response.body?.getReader();
  // 解析 SSE 事件流...
}
```

### 5.4 搜索阶段

| 阶段 | 说明 |
|------|------|
| `query_expansion_start` | 开始理解查询 |
| `thinking` | LLM 思考中 (流式输出) |
| `query_expansion_done` | 查询扩展完成 |
| `embedding` | 生成向量 |
| `searching` | Qdrant 搜索 |
| `enriching` | 加载详情 |
| `complete` | 搜索完成 |

### 5.5 技术栈

- **框架**: React 18 + TypeScript
- **构建**: Vite
- **动画**: Framer Motion
- **测试**: Playwright (E2E)
- **样式**: CSS Modules

---

## 六、Python 爬虫 (`crawler/`)

### 6.1 架构

```
crawler/
├── src/emomo_crawler/
│   ├── cli.py           # CLI 入口 (Click)
│   ├── base.py          # BaseCrawler 抽象类
│   ├── staging.py       # StagingManager (manifest + images)
│   └── sources/
│       └── fabiaoqing.py # 发表情爬虫实现
└── pyproject.toml       # uv 依赖管理
```

### 6.2 数据流

```
fabiaoqing.com
      │
      ▼ (爬取)
┌─────────────────────────────────┐
│  data/staging/fabiaoqing/       │
│  ├── manifest.jsonl             │  ← 元数据 (JSON Lines)
│  └── images/                    │
│      ├── abc123.jpg             │
│      └── def456.gif             │
└─────────────────────────────────┘
      │
      ▼ (Go Ingest)
      数据库 + Qdrant + S3
```

### 6.3 Manifest 格式

```json
{
  "id": "abc123",
  "filename": "abc123.jpg",
  "category": "emoji",
  "tags": ["开心"],
  "source_url": "https://...",
  "is_animated": false,
  "format": "jpg"
}
```

### 6.4 CLI 命令

```bash
# 爬取
uv run emomo-crawler crawl --source fabiaoqing --limit 100

# 查看状态
uv run emomo-crawler staging list
uv run emomo-crawler staging stats --source fabiaoqing

# 清理
uv run emomo-crawler staging clean --source fabiaoqing
```

> 详细爬虫和导入流程请参考 [crawler-and-ingest.md](./crawler-and-ingest.md)

---

## 七、外部依赖

| 服务 | 用途 | 配置环境变量 |
|------|------|--------------|
| **Qdrant** | 向量数据库 | `QDRANT_HOST`, `QDRANT_API_KEY`, `QDRANT_USE_TLS` |
| **S3/R2** | 图片存储 | `STORAGE_TYPE`, `STORAGE_ENDPOINT`, `STORAGE_BUCKET` |
| **OpenAI-compatible API** | VLM 描述生成 | `OPENAI_API_KEY`, `OPENAI_BASE_URL`, `VLM_MODEL` |
| **Jina Embeddings** | 向量生成 | `JINA_API_KEY` |
| **PostgreSQL/SQLite** | 元数据存储 | `DATABASE_DRIVER`, `DATABASE_URL` |

---

## 八、部署架构

### 8.1 生产环境

```
┌───────────────────────────────────────────────────────────────┐
│                     Production Environment                     │
│                                                                │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────────────┐│
│  │   Frontend  │    │   Backend   │    │   Background Jobs    ││
│  │   (Vercel/  │───▶│  (Railway/  │───▶│   (Ingest CLI)       ││
│  │   Netlify)  │    │   Render)   │    │                      ││
│  └─────────────┘    └──────┬──────┘    └─────────────────────┘│
│                            │                                   │
│         ┌──────────────────┼──────────────────┐               │
│         ▼                  ▼                  ▼               │
│  ┌─────────────┐  ┌─────────────────┐  ┌─────────────────┐   │
│  │ Qdrant Cloud │  │ Cloudflare R2  │  │ PostgreSQL/     │   │
│  │ (向量数据库) │  │ (对象存储)     │  │ Supabase        │   │
│  └─────────────┘  └─────────────────┘  └─────────────────┘   │
└───────────────────────────────────────────────────────────────┘
```

### 8.2 部署选项

| 组件 | 推荐平台 |
|------|----------|
| 后端 API | Railway, Render, Fly.io, HuggingFace Spaces |
| 前端 | Vercel, Netlify, Cloudflare Pages |
| 向量数据库 | Qdrant Cloud |
| 对象存储 | Cloudflare R2, AWS S3 |
| 关系数据库 | Supabase, Railway PostgreSQL |

> 详细部署指南请参考 [DEPLOYMENT.md](./DEPLOYMENT.md)

---

## 九、关键设计决策

### 9.1 多 Embedding 模型支持

- 通过 `meme_vectors` 表解耦 meme 和向量
- 同一图片可以用不同模型生成多个向量
- 每个模型对应独立的 Qdrant Collection
- 支持运行时切换搜索所用的 Collection

### 9.2 资源复用机制

- 基于 MD5 哈希去重
- 相同图片复用 S3 存储和 VLM 描述
- 只生成新的 Embedding 向量
- 大幅节省 VLM API 调用成本

### 9.3 混合搜索 (Hybrid Search)

- **Dense Search**: 语义向量相似度 (Cosine)
- **Sparse Search**: BM25 关键词匹配
- **RRF** (Reciprocal Rank Fusion): 融合排序
- 支持根据查询意图动态调整权重

### 9.4 查询理解 (Query Understanding)

- LLM 分析用户意图
- 生成结构化 QueryPlan
- 包含：语义查询、关键词、同义词、过滤条件
- 支持流式输出思考过程

> 详细查询理解设计请参考 [QUERY_UNDERSTANDING_DESIGN.md](./QUERY_UNDERSTANDING_DESIGN.md)

### 9.5 分层回滚策略

导入流程在每个步骤失败时会尝试回滚已完成的操作：

| 失败阶段 | 回滚操作 |
|----------|----------|
| VLM 描述生成失败 | 删除 S3 文件 |
| 数据库保存失败 | 删除 S3 文件 |
| Embedding 生成失败 | 删除 meme 记录 + S3 文件 |
| Qdrant 写入失败 | 删除 meme 记录 + S3 文件 |
| meme_vectors 保存失败 | 删除 Qdrant 点 + meme 记录 + S3 文件 |

---

## 十、相关文档

| 文档 | 说明 |
|------|------|
| [QUICK_START.md](./QUICK_START.md) | 快速开始指南 |
| [DATABASE_SCHEMA.md](./DATABASE_SCHEMA.md) | 数据库结构详解 |
| [DATA_INGEST_ARCHITECTURE.md](./DATA_INGEST_ARCHITECTURE.md) | 数据导入架构 |
| [MULTI_EMBEDDING.md](./MULTI_EMBEDDING.md) | 多 Embedding 支持 |
| [QUERY_UNDERSTANDING_DESIGN.md](./QUERY_UNDERSTANDING_DESIGN.md) | 查询理解设计 |
| [CLOUDFLARE_R2_SETUP.md](./CLOUDFLARE_R2_SETUP.md) | R2 存储配置 |
| [DEPLOYMENT.md](./DEPLOYMENT.md) | 部署指南 |
| [crawler-and-ingest.md](./crawler-and-ingest.md) | 爬虫与导入 |

---

*文档更新时间: 2026-01-23*
