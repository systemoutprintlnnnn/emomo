# AI 表情库 MVP 实现计划

## 项目概述

基于 `meme-library-roadmap.md` 设计，实现一个语义搜索表情包系统。

**技术栈**: Golang + Gin + GORM + Qdrant + SQLite + MinIO + GPT-4o mini + Jina Embeddings v3

**MVP 目标**:
- 导入 ChineseBQB 5,791 张表情包
- 实现文本搜索（文搜图）
- 搜索延迟 < 200ms

---

## 实现阶段

### 阶段一：项目骨架搭建

#### 1.1 初始化 Go 模块
```
go mod init github.com/timmy/emomo
```

#### 1.2 创建目录结构
```
emomo/
├── cmd/
│   ├── api/main.go           # API 服务入口
│   └── ingest/main.go        # 摄入 CLI 工具
├── internal/
│   ├── api/
│   │   ├── handler/          # HTTP 处理器
│   │   ├── middleware/       # 中间件
│   │   └── router.go         # 路由配置
│   ├── config/               # 配置管理
│   ├── domain/               # 领域模型
│   ├── repository/           # 数据访问层
│   ├── service/              # 业务逻辑层
│   ├── source/               # 数据源适配器
│   ├── storage/              # 对象存储抽象
│   └── pkg/                  # 工具包
├── configs/
│   └── config.yaml           # 配置文件
├── deployments/
│   ├── docker-compose.yml    # Qdrant + MinIO
│   └── Dockerfile
├── data/                     # 本地数据（.gitignore）
└── scripts/
```

#### 1.3 核心依赖
- `github.com/gin-gonic/gin` - Web 框架
- `gorm.io/gorm` + `gorm.io/driver/sqlite` - ORM
- `github.com/qdrant/go-client` - Qdrant 客户端
- `github.com/minio/minio-go/v7` - MinIO 客户端
- `github.com/spf13/viper` - 配置管理
- `go.uber.org/zap` - 日志
- `github.com/go-resty/resty/v2` - HTTP 客户端

---

### 阶段二：配置与基础设施

#### 2.1 配置管理 (`internal/config/config.go`)
- 使用 Viper 读取 YAML 配置
- 支持环境变量覆盖
- 配置项：数据库路径、Qdrant 地址、MinIO 配置、API Keys

#### 2.2 Docker Compose (`deployments/docker-compose.yml`)
- Qdrant 容器（端口 6333/6334）
- MinIO 容器（端口 9000/9001）

#### 2.3 数据库初始化 (`internal/repository/db.go`)
- GORM 初始化 SQLite
- 自动迁移表结构

---

### 阶段三：领域模型与数据层

#### 3.1 领域模型 (`internal/domain/`)
- `meme.go` - Meme 实体
- `source.go` - DataSource 实体
- `job.go` - IngestJob 实体

#### 3.2 SQLite 表结构
```sql
-- memes 表
id, source_type, source_id, storage_key, original_url,
width, height, format, is_animated, file_size,
md5_hash, qdrant_point_id, vlm_description, vlm_model,
embedding_model, tags, category, status, created_at, updated_at

-- data_sources 表
id, name, type, config, last_sync_at, last_sync_cursor, is_enabled, priority

-- ingest_jobs 表
id, source_id, status, total_items, processed_items,
failed_items, started_at, completed_at, error_log
```

#### 3.3 Repository 层 (`internal/repository/`)
- `meme_repo.go` - Meme CRUD + MD5 去重查询
- `source_repo.go` - 数据源配置
- `qdrant_repo.go` - Qdrant 向量操作

---

### 阶段四：存储服务

#### 4.1 对象存储抽象 (`internal/storage/`)
- `interface.go` - 存储接口定义
- `minio.go` - MinIO 实现
  - `Upload(ctx, key, reader)` - 上传图片
  - `GetURL(key)` - 获取访问 URL
  - `Delete(key)` - 删除图片

---

### 阶段五：外部 AI 服务

#### 5.1 VLM 服务 (`internal/service/vlm.go`)
- 对接 GPT-4o mini API
- Prompt 模板：描述图片内容、表情、文字、情绪
- 支持 Base64 图片输入
- 返回 50-100 字中文描述

#### 5.2 Embedding 服务 (`internal/service/embedding.go`)
- 对接 Jina Embeddings v3 API
- 输入：文本描述
- 输出：1024 维向量

---

### 阶段六：数据源适配器

#### 6.1 数据源接口 (`internal/source/interface.go`)
```go
type Source interface {
    FetchBatch(ctx, cursor, limit) ([]MemeItem, nextCursor, error)
    GetSourceID() string
    SupportsIncremental() bool
}
```

#### 6.2 ChineseBQB 适配器 (`internal/source/chinesebqb/`)
- 从 GitHub 仓库获取图片列表
- 解析目录结构获取分类
- 下载图片到本地

---

### 阶段七：摄入服务

#### 7.1 摄入编排 (`internal/service/ingest.go`)
完整流程：
1. 从数据源获取图片列表
2. 下载图片 & 计算 MD5
3. MD5 去重检查（查 SQLite）
4. 上传图片到 MinIO
5. 调用 VLM 生成描述
6. 调用 Embedding 生成向量
7. 写入 Qdrant（向量 + Payload）
8. 写入 SQLite 元数据

#### 7.2 并发控制
- Worker Pool 模式
- 可配置并发数（默认 5）
- 失败重试机制

#### 7.3 摄入 CLI (`cmd/ingest/main.go`)
```bash
./ingest --source=chinesebqb --limit=100
```

---

### 阶段八：搜索服务

#### 8.1 搜索核心 (`internal/service/search.go`)
- `TextSearch(query, topK, filters)` - 文本搜索
  1. 调用 Embedding 服务生成查询向量
  2. 在 Qdrant 执行向量相似搜索
  3. 返回结果（含 URL、分数、描述）

#### 8.2 Qdrant 配置
- Collection: `memes`
- 向量维度: 1024
- 距离度量: Cosine
- HNSW 索引: ef=128, m=16

---

### 阶段九：API 层

#### 9.1 路由 (`internal/api/router.go`)
```
GET  /health              # 健康检查
POST /api/v1/search       # 文本搜索
GET  /api/v1/categories   # 分类列表
GET  /api/v1/memes        # 表情包列表（分页）
GET  /api/v1/memes/:id    # 单个表情包详情
```

#### 9.2 Handler (`internal/api/handler/`)
- `search.go` - 搜索接口
- `meme.go` - 表情包 CRUD
- `health.go` - 健康检查

#### 9.3 中间件 (`internal/api/middleware/`)
- CORS 支持
- 请求日志
- 错误处理

---

### 阶段十：API 服务入口

#### 10.1 主入口 (`cmd/api/main.go`)
- 加载配置
- 初始化数据库连接
- 初始化各服务
- 启动 HTTP 服务器

---

## 关键文件清单

| 文件 | 用途 |
|------|------|
| `cmd/api/main.go` | API 服务入口 |
| `cmd/ingest/main.go` | 摄入 CLI 入口 |
| `internal/config/config.go` | 配置管理 |
| `internal/domain/meme.go` | Meme 领域模型 |
| `internal/repository/meme_repo.go` | Meme 数据访问 |
| `internal/repository/qdrant_repo.go` | Qdrant 操作 |
| `internal/service/vlm.go` | VLM 图片描述 |
| `internal/service/embedding.go` | 文本向量化 |
| `internal/service/ingest.go` | 摄入编排 |
| `internal/service/search.go` | 搜索服务 |
| `internal/source/interface.go` | 数据源接口 |
| `internal/source/chinesebqb/adapter.go` | ChineseBQB 适配器 |
| `internal/storage/minio.go` | MinIO 存储 |
| `internal/api/router.go` | 路由配置 |
| `internal/api/handler/search.go` | 搜索 Handler |
| `configs/config.yaml` | 配置文件 |
| `deployments/docker-compose.yml` | 容器编排 |

---

## 实现顺序

1. **项目骨架** - go.mod, 目录结构
2. **配置管理** - Viper 配置
3. **Docker 环境** - docker-compose.yml
4. **领域模型** - 实体定义
5. **数据库层** - GORM + SQLite
6. **存储层** - MinIO 客户端
7. **VLM 服务** - GPT-4o mini 对接
8. **Embedding 服务** - Jina API 对接
9. **数据源适配器** - ChineseBQB
10. **摄入服务** - 完整摄入流程
11. **搜索服务** - 向量搜索
12. **API 层** - HTTP 接口
13. **集成测试** - 端到端验证

---

## 确认的决策

1. **API Key 存储**: 环境变量方式 (`.env` 文件)
2. **图搜图功能**: MVP 阶段不实现，仅支持文搜图
3. **ChineseBQB 获取**: Git Clone 到本地

---

## 环境变量配置

```bash
# .env
OPENAI_API_KEY=sk-xxx           # GPT-4o mini
JINA_API_KEY=jina_xxx           # Jina Embeddings
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=minioadmin
```
