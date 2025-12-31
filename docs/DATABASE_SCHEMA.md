# Emomo 数据库结构文档

本文档详细描述了 Emomo 项目中的数据库结构设计、表关系及其在代码中的使用情况。

## 目录

- [概述](#概述)
- [数据库配置](#数据库配置)
- [数据库初始化](#数据库初始化)
- [表结构详解](#表结构详解)
  - [memes 表](#memes-表)
  - [meme_vectors 表](#meme_vectors-表)
  - [data_sources 表](#data_sources-表)
  - [ingest_jobs 表](#ingest_jobs-表)
- [表关系图](#表关系图)
- [向量数据库 Qdrant](#向量数据库-qdrant)
- [Repository 层使用详解](#repository-层使用详解)
- [Service 层数据库操作](#service-层数据库操作)
- [API 端点与数据库交互](#api-端点与数据库交互)

---

## 概述

Emomo 项目采用 **双数据库架构**：

| 数据库类型 | 用途 | 支持的驱动 |
|-----------|------|-----------|
| **关系型数据库** | 存储表情包元数据、向量记录、数据源配置、任务状态 | SQLite / PostgreSQL |
| **向量数据库 (Qdrant)** | 存储向量嵌入，支持语义相似度搜索 | Qdrant (gRPC) |

### 技术栈

- **ORM**: GORM (Go Object Relational Mapping)
- **SQLite**: 本地开发默认数据库
- **PostgreSQL**: 生产环境推荐
- **Qdrant**: 高性能向量搜索引擎

---

## 数据库配置

### 配置结构 (`internal/config/config.go`)

```go
type DatabaseConfig struct {
    Driver          string        // 数据库驱动: sqlite, postgres
    URL             string        // PostgreSQL 连接 URL (优先级最高)
    Path            string        // SQLite 文件路径
    Host            string        // PostgreSQL 主机
    Port            int           // PostgreSQL 端口
    User            string        // PostgreSQL 用户名
    Password        string        // PostgreSQL 密码
    DBName          string        // PostgreSQL 数据库名
    SSLMode         string        // PostgreSQL SSL 模式
    MaxIdleConns    int           // 最大空闲连接数
    MaxOpenConns    int           // 最大打开连接数
    ConnMaxLifetime time.Duration // 连接最大生命周期
}
```

### 默认配置值

| 配置项 | 默认值 |
|--------|--------|
| `driver` | `sqlite` |
| `path` | `./data/memes.db` |
| `host` | `localhost` |
| `port` | `5432` |
| `dbname` | `emomo` |
| `sslmode` | `disable` |
| `max_idle_conns` | `10` |
| `max_open_conns` | `100` |
| `conn_max_lifetime` | `1h` |

### 环境变量绑定

| 环境变量 | 配置字段 |
|----------|----------|
| `DATABASE_DRIVER` | `database.driver` |
| `DATABASE_URL` | `database.url` |
| `DATABASE_PATH` | `database.path` |
| `DATABASE_HOST` | `database.host` |
| `DATABASE_PORT` | `database.port` |
| `DATABASE_USER` | `database.user` |
| `DATABASE_PASSWORD` | `database.password` |
| `DATABASE_DBNAME` | `database.dbname` |
| `DATABASE_SSLMODE` | `database.sslmode` |

### config.yaml 示例

```yaml
database:
  driver: postgres
  path: ./data/memes.db
  host: localhost
  port: 6543
  user: postgres
  dbname: emomo
  sslmode: disable
  max_idle_conns: 10
  max_open_conns: 100
  conn_max_lifetime: 1h
```

---

## 数据库初始化

### 初始化流程 (`internal/repository/db.go`)

```go
func InitDB(cfg *config.DatabaseConfig) (*gorm.DB, error) {
    // 1. 根据 driver 选择数据库类型
    switch cfg.Driver {
    case "postgres":
        db, err = initPostgres(cfg, gormConfig)
    case "sqlite":
        db, err = initSQLite(cfg, gormConfig)
    default:
        db, err = initSQLite(cfg, gormConfig)  // 默认使用 SQLite
    }

    // 2. 配置连接池
    sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
    sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
    sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

    // 3. 自动迁移所有表
    db.AutoMigrate(
        &domain.Meme{},
        &domain.MemeVector{},
        &domain.DataSource{},
        &domain.IngestJob{},
    )
}
```

### SQLite 特殊配置

SQLite 初始化时启用了以下 PRAGMA 优化：

```go
db.Exec("PRAGMA journal_mode=WAL")  // 启用 WAL 模式提高并发
db.Exec("PRAGMA foreign_keys=ON")   // 启用外键约束
```

### PostgreSQL 特殊配置

PostgreSQL 使用 `PreferSimpleProtocol: true` 以支持事务池（如 Supabase）：

```go
gorm.Open(postgres.New(postgres.Config{
    DSN:                  dsn,
    PreferSimpleProtocol: true,  // 禁用隐式预处理语句
}), gormConfig)
```

---

## 表结构详解

### memes 表

**文件位置**: `internal/domain/meme.go`

存储表情包的核心元数据。

#### 字段定义

| 字段 | 类型 | 约束 | 描述 |
|------|------|------|------|
| `id` | TEXT | PRIMARY KEY | UUID 格式主键 |
| `source_type` | TEXT | NOT NULL, UNIQUE (with source_id) | 数据来源类型 (如 `chinesebqb`, `fabiaoqing`) |
| `source_id` | TEXT | NOT NULL, UNIQUE (with source_type) | 在来源中的唯一标识 |
| `storage_key` | TEXT | - | S3/R2 存储路径 |
| `local_path` | TEXT | - | 本地文件路径 (用于调试) |
| `width` | INT | - | 图片宽度 (像素) |
| `height` | INT | - | 图片高度 (像素) |
| `format` | TEXT | - | 图片格式 (jpeg, png, gif, webp) |
| `is_animated` | BOOL | - | 是否为动态图 |
| `file_size` | BIGINT | - | 文件大小 (字节) |
| `md5_hash` | TEXT | UNIQUE INDEX | 图片内容的 MD5 哈希 (用于去重) |
| `perceptual_hash` | TEXT | - | 感知哈希 (预留，未使用) |
| `qdrant_point_id` | TEXT | - | Qdrant 中的 Point ID (向后兼容) |
| `vlm_description` | TEXT | - | VLM 生成的图片描述 |
| `vlm_model` | TEXT | - | 生成描述的 VLM 模型名称 |
| `embedding_model` | TEXT | - | 使用的 Embedding 模型 (向后兼容) |
| `tags` | TEXT (JSON) | - | 标签数组 (JSON 序列化) |
| `category` | TEXT | INDEX | 分类名称 |
| `status` | TEXT | INDEX, DEFAULT 'pending' | 处理状态: `pending`, `active`, `failed` |
| `created_at` | TIMESTAMP | - | 创建时间 |
| `updated_at` | TIMESTAMP | - | 更新时间 |

#### 索引

```sql
CREATE UNIQUE INDEX idx_memes_source ON memes(source_type, source_id);
CREATE UNIQUE INDEX idx_memes_md5 ON memes(md5_hash);
CREATE INDEX idx_memes_category ON memes(category);
CREATE INDEX idx_memes_status ON memes(status);
```

#### Go 结构体定义

```go
type Meme struct {
    ID             string      `gorm:"type:text;primaryKey" json:"id"`
    SourceType     string      `gorm:"type:text;not null;index:idx_memes_source,unique" json:"source_type"`
    SourceID       string      `gorm:"type:text;not null;index:idx_memes_source,unique" json:"source_id"`
    StorageKey     string      `gorm:"type:text" json:"storage_key"`
    LocalPath      string      `gorm:"column:local_path" json:"local_path,omitempty"`
    Width          int         `json:"width"`
    Height         int         `json:"height"`
    Format         string      `json:"format"`
    IsAnimated     bool        `json:"is_animated"`
    FileSize       int64       `json:"file_size"`
    MD5Hash        string      `gorm:"uniqueIndex:idx_memes_md5" json:"md5_hash"`
    PerceptualHash string      `gorm:"type:text" json:"perceptual_hash,omitempty"`
    QdrantPointID  string      `gorm:"type:text" json:"qdrant_point_id,omitempty"`
    VLMDescription string      `gorm:"type:text" json:"vlm_description,omitempty"`
    VLMModel       string      `gorm:"type:text" json:"vlm_model,omitempty"`
    EmbeddingModel string      `gorm:"type:text" json:"embedding_model,omitempty"`
    Tags           StringArray `gorm:"type:text" json:"tags"`
    Category       string      `gorm:"type:text;index:idx_memes_category" json:"category"`
    Status         MemeStatus  `gorm:"type:text;index:idx_memes_status;default:pending" json:"status"`
    CreatedAt      time.Time   `json:"created_at"`
    UpdatedAt      time.Time   `json:"updated_at"`
}
```

#### MemeStatus 枚举

```go
const (
    MemeStatusPending MemeStatus = "pending"  // 待处理
    MemeStatusActive  MemeStatus = "active"   // 已激活，可搜索
    MemeStatusFailed  MemeStatus = "failed"   // 处理失败
)
```

#### StringArray 自定义类型

由于 SQLite 不原生支持数组类型，`Tags` 字段使用自定义 `StringArray` 类型：

```go
type StringArray []string

// 写入数据库时转换为 JSON 字符串
func (a StringArray) Value() (driver.Value, error) {
    if a == nil {
        return "[]", nil
    }
    b, err := json.Marshal(a)
    return string(b), nil
}

// 从数据库读取时解析 JSON
func (a *StringArray) Scan(value interface{}) error {
    bytes := value.([]byte)
    return json.Unmarshal(bytes, a)
}
```

---

### meme_vectors 表

**文件位置**: `internal/domain/meme_vector.go`

记录 meme 与向量嵌入的关联关系，支持同一 meme 使用不同 Embedding 模型。

#### 字段定义

| 字段 | 类型 | 约束 | 描述 |
|------|------|------|------|
| `id` | TEXT | PRIMARY KEY | UUID 格式主键 |
| `meme_id` | TEXT | NOT NULL, INDEX | 关联的 meme ID |
| `md5_hash` | TEXT | NOT NULL, UNIQUE (with collection) | 图片 MD5 哈希 (冗余存储加速查询) |
| `collection` | TEXT | NOT NULL, UNIQUE (with md5_hash) | Qdrant collection 名称 |
| `embedding_model` | TEXT | NOT NULL | 使用的 Embedding 模型名称 |
| `qdrant_point_id` | TEXT | NOT NULL | Qdrant 中的 Point ID |
| `status` | TEXT | DEFAULT 'active' | 状态: `active`, `deleted` |
| `created_at` | TIMESTAMP | - | 创建时间 |

#### 索引

```sql
CREATE UNIQUE INDEX idx_meme_vectors_md5_collection ON meme_vectors(md5_hash, collection);
CREATE INDEX idx_meme_vectors_meme ON meme_vectors(meme_id);
```

#### Go 结构体定义

```go
type MemeVector struct {
    ID             string    `gorm:"type:text;primaryKey" json:"id"`
    MemeID         string    `gorm:"type:text;not null;index:idx_meme_vectors_meme" json:"meme_id"`
    MD5Hash        string    `gorm:"type:text;not null;uniqueIndex:idx_meme_vectors_md5_collection" json:"md5_hash"`
    Collection     string    `gorm:"type:text;not null;uniqueIndex:idx_meme_vectors_md5_collection" json:"collection"`
    EmbeddingModel string    `gorm:"type:text;not null" json:"embedding_model"`
    QdrantPointID  string    `gorm:"type:text;not null" json:"qdrant_point_id"`
    Status         string    `gorm:"type:text;default:active" json:"status"`
    CreatedAt      time.Time `json:"created_at"`
}
```

#### 设计说明

此表是多 Embedding 模型支持的核心：

1. **资源复用**: 同一图片（相同 MD5）可以使用不同 Embedding 模型生成多个向量
2. **去重机制**: `(md5_hash, collection)` 唯一索引确保同一图片在同一 collection 只有一条向量记录
3. **独立 Point ID**: 每个向量有独立的 `qdrant_point_id`，与 `meme_id` 解耦

---

### data_sources 表

**文件位置**: `internal/domain/source.go`

存储数据源配置信息（预留扩展用，当前主要通过配置文件管理）。

#### 字段定义

| 字段 | 类型 | 约束 | 描述 |
|------|------|------|------|
| `id` | TEXT | PRIMARY KEY | UUID 格式主键 |
| `name` | TEXT | NOT NULL | 数据源名称 |
| `type` | TEXT | NOT NULL | 数据源类型: `static`, `api`, `crawler` |
| `config` | TEXT (JSON) | - | 数据源配置 (JSON 格式) |
| `last_sync_at` | TIMESTAMP | - | 上次同步时间 |
| `last_sync_cursor` | TEXT | - | 上次同步位置 (用于增量同步) |
| `is_enabled` | BOOL | DEFAULT TRUE | 是否启用 |
| `priority` | INT | DEFAULT 0 | 优先级 |
| `created_at` | TIMESTAMP | - | 创建时间 |
| `updated_at` | TIMESTAMP | - | 更新时间 |

#### SourceType 枚举

```go
const (
    SourceTypeStatic  SourceType = "static"   // 静态文件系统
    SourceTypeAPI     SourceType = "api"      // API 数据源
    SourceTypeCrawler SourceType = "crawler"  // 爬虫数据源
)
```

#### SourceConfig 自定义类型

```go
type SourceConfig map[string]interface{}

// 写入数据库时转换为 JSON 字符串
func (c SourceConfig) Value() (driver.Value, error) {
    if c == nil {
        return "{}", nil
    }
    return json.Marshal(c)
}
```

---

### ingest_jobs 表

**文件位置**: `internal/domain/job.go`

记录数据导入任务的执行状态和统计信息。

#### 字段定义

| 字段 | 类型 | 约束 | 描述 |
|------|------|------|------|
| `id` | TEXT | PRIMARY KEY | UUID 格式主键 |
| `source_id` | TEXT | NOT NULL, INDEX | 关联的数据源 ID |
| `status` | TEXT | DEFAULT 'pending' | 任务状态 |
| `total_items` | INT | DEFAULT 0 | 总项目数 |
| `processed_items` | INT | DEFAULT 0 | 已处理数 |
| `failed_items` | INT | DEFAULT 0 | 失败数 |
| `started_at` | TIMESTAMP | - | 开始时间 |
| `completed_at` | TIMESTAMP | - | 完成时间 |
| `error_log` | TEXT | - | 错误日志 |
| `created_at` | TIMESTAMP | - | 创建时间 |
| `updated_at` | TIMESTAMP | - | 更新时间 |

#### JobStatus 枚举

```go
const (
    JobStatusPending   JobStatus = "pending"    // 待执行
    JobStatusRunning   JobStatus = "running"    // 执行中
    JobStatusCompleted JobStatus = "completed"  // 已完成
    JobStatusFailed    JobStatus = "failed"     // 失败
)
```

---

## 表关系图

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              data_sources                                │
│  (数据源配置表 - 预留扩展)                                                │
├─────────────────────────────────────────────────────────────────────────┤
│  id │ name │ type │ config │ last_sync_at │ is_enabled │ priority       │
└──────────┬──────────────────────────────────────────────────────────────┘
           │ (source_id 外键关联)
           ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                               ingest_jobs                                │
│  (导入任务表)                                                             │
├─────────────────────────────────────────────────────────────────────────┤
│  id │ source_id │ status │ total_items │ processed_items │ failed_items  │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────┐
│                                  memes                                   │
│  (表情包元数据表 - 核心表)                                                 │
├─────────────────────────────────────────────────────────────────────────┤
│  id (PK) │ source_type │ source_id │ md5_hash (UK) │ storage_key         │
│  width │ height │ format │ is_animated │ vlm_description │ status        │
│  tags (JSON) │ category │ created_at │ updated_at                        │
└──────────┬──────────────────────────────────────────────────────────────┘
           │
           │ (meme_id 外键关联)
           │
           ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                              meme_vectors                                │
│  (向量关联表 - 多 Embedding 模型支持)                                      │
├─────────────────────────────────────────────────────────────────────────┤
│  id (PK) │ meme_id (FK) │ md5_hash │ collection │ embedding_model         │
│  qdrant_point_id │ status │ created_at                                   │
│                                                                          │
│  UNIQUE (md5_hash, collection)                                           │
└──────────┬──────────────────────────────────────────────────────────────┘
           │
           │ (qdrant_point_id 关联)
           │
           ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                          Qdrant Collections                              │
│  (向量数据库)                                                            │
├─────────────────────────────────────────────────────────────────────────┤
│  Collection: emomo (dim: 1024)                                          │
│    └── Points: [id, vector[1024], payload{meme_id, source_type, ...}]   │
│                                                                          │
│  Collection: emomo-qwen3-embedding-8b (dim: 4096)                       │
│    └── Points: [id, vector[4096], payload{meme_id, source_type, ...}]   │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 向量数据库 Qdrant

### 配置 (`internal/config/config.go`)

```go
type QdrantConfig struct {
    Host       string  // Qdrant 主机地址
    Port       int     // gRPC 端口 (默认 6334)
    Collection string  // 默认 collection 名称
    APIKey     string  // Qdrant Cloud API Key
    UseTLS     bool    // 是否使用 TLS
}
```

### 连接管理 (`internal/repository/qdrant_repo.go`)

```go
type QdrantConnectionConfig struct {
    Host            string
    Port            int
    Collection      string
    APIKey          string  // 设置后自动启用 TLS
    UseTLS          bool
    VectorDimension int     // 向量维度 (默认 1024)
}
```

### Collection 结构

每个 Qdrant collection 存储以下数据：

#### Vector 配置

- **Distance Metric**: Cosine (余弦相似度)
- **HNSW Config**: 
  - M: 16
  - EF Construct: 128
  - Full Scan Threshold: 10000

#### Point 结构

```go
type MemePayload struct {
    MemeID         string   `json:"meme_id"`         // 关联的 meme ID
    SourceType     string   `json:"source_type"`     // 数据来源类型
    Category       string   `json:"category"`        // 分类
    IsAnimated     bool     `json:"is_animated"`     // 是否动态图
    Tags           []string `json:"tags"`            // 标签数组
    VLMDescription string   `json:"vlm_description"` // VLM 描述
    StorageURL     string   `json:"storage_url"`     // 图片 URL
}
```

### 多 Collection 支持

| Collection 名称 | Embedding 模型 | 向量维度 |
|----------------|----------------|----------|
| `emomo` | jina-embeddings-v3 | 1024 |
| `emomo-qwen3-embedding-8b` | Qwen/Qwen3-Embedding-8B | 4096 |

---

## Repository 层使用详解

### MemeRepository (`internal/repository/meme_repo.go`)

#### 核心方法

| 方法 | 功能 | 使用场景 |
|------|------|----------|
| `Create(meme)` | 创建新记录 | 首次导入 |
| `Upsert(meme)` | 创建或更新 (基于 source_type + source_id) | 增量导入 |
| `Update(meme)` | 更新记录 | 重试处理 |
| `GetByID(id)` | 按 ID 查询 | API 获取详情 |
| `GetByMD5Hash(md5)` | 按 MD5 查询 | 去重检查、资源复用 |
| `ExistsByMD5Hash(md5)` | 检查 MD5 是否存在 | 快速去重 |
| `GetBySourceID(type, id)` | 按来源查询 | 来源去重 |
| `ExistsBySourceID(type, id)` | 检查来源是否存在 | 快速来源去重 |
| `ListByStatus(status, limit, offset)` | 按状态分页查询 | 重试 pending |
| `ListByCategory(category, limit, offset)` | 按分类分页查询 | 浏览列表 |
| `GetCategories()` | 获取所有分类 | 分类筛选 |
| `CountByStatus(status)` | 按状态统计数量 | 统计报表 |
| `GetByIDs(ids)` | 批量按 ID 查询 | 搜索结果丰富 |
| `Delete(id)` | 删除记录 | 清理数据 |

#### Upsert 实现 (冲突处理)

```go
func (r *MemeRepository) Upsert(ctx context.Context, meme *domain.Meme) error {
    return r.db.WithContext(ctx).Clauses(clause.OnConflict{
        Columns:   []clause.Column{{Name: "source_type"}, {Name: "source_id"}},
        UpdateAll: true,
    }).Create(meme).Error
}
```

---

### MemeVectorRepository (`internal/repository/meme_vector_repo.go`)

#### 核心方法

| 方法 | 功能 | 使用场景 |
|------|------|----------|
| `Create(vector)` | 创建向量记录 | 导入完成后记录 |
| `ExistsByMD5AndCollection(md5, collection)` | 检查是否存在 | 多 Embedding 去重 |
| `GetByMD5AndCollection(md5, collection)` | 按 MD5+Collection 查询 | 获取已有向量 |
| `GetByMemeID(memeID)` | 获取 meme 的所有向量 | 查看向量分布 |
| `GetByCollection(collection, limit, offset)` | 按 collection 分页 | Collection 管理 |
| `CountByCollection(collection)` | 统计 collection 数量 | 统计报表 |
| `Delete(id)` | 删除记录 | 清理 |
| `DeleteByMemeIDAndCollection(memeID, collection)` | 按关联删除 | 清理特定向量 |

---

### QdrantRepository (`internal/repository/qdrant_repo.go`)

#### 核心方法

| 方法 | 功能 | 使用场景 |
|------|------|----------|
| `EnsureCollection()` | 确保 collection 存在 | 启动初始化 |
| `GetCollectionName()` | 获取 collection 名称 | 日志记录 |
| `GetVectorDimension()` | 获取向量维度 | 验证 |
| `Upsert(pointID, vector, payload)` | 写入/更新向量 | 导入 |
| `Search(vector, topK, filters)` | 向量相似度搜索 | 语义搜索 |
| `PointExists(pointID)` | 检查点是否存在 | 去重 |
| `Delete(pointID)` | 删除向量点 | 清理 |

#### 搜索过滤器

```go
type SearchFilters struct {
    Category   *string  // 分类过滤
    IsAnimated *bool    // 动态图过滤
    SourceType *string  // 来源类型过滤
}
```

---

## Service 层数据库操作

### IngestService (`internal/service/ingest.go`)

数据导入服务，协调多个 Repository 完成完整的导入流程。

#### 导入流程

```
1. 读取图片数据
2. 计算 MD5 哈希
3. 检查 meme_vectors 是否已有该 MD5+Collection 记录
   ├── 已存在 → 跳过 (返回 errSkipDuplicate)
   └── 不存在 → 继续
4. 检查 memes 是否已有该 MD5 记录
   ├── 已存在 → 复用 storage_key 和 vlm_description
   └── 不存在 → 完整处理:
       a. 生成 VLM 描述
       b. 上传 S3
       c. 创建 meme 记录
5. 生成 Embedding 向量
6. 写入 Qdrant
7. 创建 meme_vectors 记录
```

#### 使用的 Repository

```go
type IngestService struct {
    memeRepo   *repository.MemeRepository       // memes 表操作
    vectorRepo *repository.MemeVectorRepository // meme_vectors 表操作
    qdrantRepo *repository.QdrantRepository     // Qdrant 操作
    // ...
}
```

#### 事务与回滚

导入流程在每个步骤失败时会尝试回滚已完成的操作：

```go
// 数据库保存失败 → 回滚 S3 上传
if err := s.memeRepo.Upsert(ctx, meme); err != nil {
    if uploaded {
        s.storage.Delete(ctx, storageKey)
    }
    return err
}

// meme_vectors 保存失败 → 回滚 Qdrant
if err := s.vectorRepo.Create(ctx, vectorRecord); err != nil {
    s.qdrantRepo.Delete(ctx, pointID)
    return err
}
```

---

### SearchService (`internal/service/search.go`)

搜索服务，提供语义搜索和列表功能。

#### 搜索流程

```
1. 确定使用的 collection 和 embedding provider
2. (可选) 查询扩展 (Query Expansion)
3. 生成查询向量
4. 调用 Qdrant 搜索
5. 应用分数阈值过滤
6. 从 memes 表丰富结果数据 (如 width, height)
7. 返回结果
```

#### 多 Collection 支持

```go
type SearchService struct {
    memeRepo          *repository.MemeRepository
    defaultQdrantRepo *repository.QdrantRepository
    defaultEmbedding  EmbeddingProvider
    collections       map[string]*CollectionConfig  // 额外的 collections
    // ...
}

// 注册额外的 collection
func (s *SearchService) RegisterCollection(name string, 
    qdrantRepo *repository.QdrantRepository, 
    embedding EmbeddingProvider) {
    s.collections[name] = &CollectionConfig{
        QdrantRepo: qdrantRepo,
        Embedding:  embedding,
    }
}
```

---

## API 端点与数据库交互

### 端点与数据库操作映射

| 端点 | 方法 | 数据库操作 |
|------|------|-----------|
| `GET /health` | - | 无数据库操作 |
| `POST /api/v1/search` | `SearchService.TextSearch` | Qdrant 搜索 + memes 表查询 |
| `GET /api/v1/search` | `SearchService.TextSearch` | Qdrant 搜索 + memes 表查询 |
| `GET /api/v1/categories` | `MemeRepository.GetCategories` | memes 表查询 |
| `GET /api/v1/memes` | `MemeRepository.ListByCategory` | memes 表分页查询 |
| `GET /api/v1/memes/:id` | `MemeRepository.GetByID` | memes 表单条查询 |
| `GET /api/v1/stats` | `MemeRepository.CountByStatus` + `GetCategories` | memes 表统计 |
| `POST /api/v1/ingest` | `IngestService.IngestFromSource` | memes + meme_vectors + Qdrant |
| `GET /api/v1/ingest/status` | - | 内存状态 (无数据库操作) |

### 搜索请求流程详解

```
POST /api/v1/search
    │
    ▼
SearchHandler.TextSearch
    │
    ├── 解析请求 (query, top_k, collection, category, ...)
    │
    ▼
SearchService.TextSearch
    │
    ├── 选择 collection 和 embedding provider
    │
    ├── (可选) QueryExpansion 扩展查询
    │
    ├── EmbeddingProvider.EmbedQuery(query) → 生成查询向量
    │
    ├── QdrantRepository.Search(vector, topK, filters) → Qdrant 搜索
    │
    ├── 过滤低分结果 (score < threshold)
    │
    ├── MemeRepository.GetByIDs(ids) → 丰富 width/height
    │
    └── 返回 SearchResponse
```

---

## 总结

Emomo 项目的数据库架构设计具有以下特点：

1. **灵活的数据库支持**: 同时支持 SQLite (开发) 和 PostgreSQL (生产)
2. **双数据库架构**: 关系型数据库存储元数据，Qdrant 存储向量
3. **多 Embedding 模型支持**: 通过 `meme_vectors` 表实现同一图片多向量
4. **资源复用**: 相同 MD5 的图片复用 S3 存储和 VLM 描述
5. **清晰的分层**: Domain → Repository → Service → Handler
6. **完善的去重机制**: 基于 MD5 和 source_type+source_id 双重去重

---

*文档更新时间: 2025-12-31*

