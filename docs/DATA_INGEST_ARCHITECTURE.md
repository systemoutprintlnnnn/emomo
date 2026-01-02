# 数据导入架构深度解析

本文档详细介绍 Emomo 系统的数据导入（Ingest）架构，包括数据流、处理管道、并发模型、多 Embedding 支持和错误处理机制。

> 快速上手请参考 [crawler-and-ingest.md](./crawler-and-ingest.md)

## 目录

1. [整体架构](#1-整体架构)
2. [入口点分析](#2-入口点分析)
3. [IngestService 核心实现](#3-ingestservice-核心实现)
4. [Source 接口与数据源适配器](#4-source-接口与数据源适配器)
5. [Python 爬虫系统](#5-python-爬虫系统)
6. [多 Embedding 支持机制](#6-多-embedding-支持机制)
7. [处理管道详解](#7-处理管道详解)
8. [并发模型与性能](#8-并发模型与性能)
9. [错误处理与事务回滚](#9-错误处理与事务回滚)
10. [Qdrant 集成](#10-qdrant-集成)
11. [配置参考](#11-配置参考)

---

## 1. 整体架构

### 1.1 数据流总览

```
┌─────────────────────────────────────────────────────────────┐
│                    Python Crawler                            │
│            fabiaoqing.com → 下载图片 → manifest.jsonl        │
└─────────────────────┬───────────────────────────────────────┘
                      │ staging/{source_id}/
                      │   ├── manifest.jsonl
                      │   └── images/
                      ▼
┌─────────────────────────────────────────────────────────────┐
│              Go Ingest CLI (cmd/ingest/main.go)              │
│       ./ingest --source=staging:fabiaoqing --embedding=jina  │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│                    IngestService                             │
│  ┌────────────────────────────────────────────────────────┐ │
│  │         Worker Pool (Goroutines)                        │ │
│  │                                                         │ │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐              │ │
│  │  │ Worker 0 │  │ Worker 1 │  │ Worker N │   ...        │ │
│  │  └────┬─────┘  └────┬─────┘  └────┬─────┘              │ │
│  │       │             │             │                     │ │
│  │       └─────────────┼─────────────┘                     │ │
│  │                     ▼                                   │ │
│  │              processItem()                              │ │
│  └────────────────────────────────────────────────────────┘ │
└─────────────────────┬───────────────────────────────────────┘
                      │
        ┌─────────────┼─────────────┬──────────────┐
        ▼             ▼             ▼              ▼
   ┌────────┐   ┌──────────┐  ┌────────────┐  ┌─────────┐
   │ VLM    │   │ S3/R2    │  │ Embedding  │  │ Qdrant  │
   │ 生成描述│   │ 上传图片 │  │ 生成向量   │  │ 存储向量│
   └────────┘   └──────────┘  └────────────┘  └─────────┘
        │             │             │              │
        └─────────────┴─────────────┴──────────────┘
                      │
                      ▼
          ┌──────────────────────┐
          │    PostgreSQL/SQLite  │
          │  memes + meme_vectors │
          └──────────────────────┘
```

### 1.2 核心组件

| 组件 | 位置 | 职责 |
|------|------|------|
| Ingest CLI | `cmd/ingest/main.go` | 命令行入口，参数解析，依赖初始化 |
| IngestService | `internal/service/ingest.go` | 核心处理逻辑，Worker Pool |
| Source Interface | `internal/source/source.go` | 数据源抽象接口 |
| ChineseBQB Adapter | `internal/source/chinesebqb/` | 本地静态仓库适配器 |
| Staging Adapter | `internal/source/staging/` | 爬虫输出适配器 |
| EmbeddingRegistry | `internal/service/embedding_registry.go` | 多 Embedding 管理 |
| QdrantRepository | `internal/repository/qdrant_repo.go` | 向量数据库操作 |

---

## 2. 入口点分析

### 2.1 命令行参数

```bash
./ingest [flags]

Flags:
  --source      数据源 ("chinesebqb" 或 "staging:<source_id>")  [默认: chinesebqb]
  --limit       最大处理数量                                    [默认: 100]
  --retry       重试 pending 状态的记录                         [默认: false]
  --force       强制重新处理，跳过去重检查                      [默认: false]
  --embedding   Embedding 配置名称 ("jina", "qwen3" 等)         [默认: 使用 is_default=true]
  --config      配置文件路径                                    [默认: ./configs/config.yaml]
```

### 2.2 初始化流程

```go
// cmd/ingest/main.go

func main() {
    // 1. 日志初始化
    appLogger := logger.New(...)

    // 2. 解析命令行参数
    flag.Parse()

    // 3. 加载配置
    cfg, err := config.Load(*configPath)

    // 4. 确定 Embedding 配置
    var embeddingCfg *config.EmbeddingConfig
    if *embeddingName != "" {
        embeddingCfg = cfg.GetEmbeddingByName(*embeddingName)
    } else {
        embeddingCfg = cfg.GetDefaultEmbedding()
    }
    embeddingCfg.ResolveEnvVars()      // 解析 api_key_env 等
    embeddingCfg.ValidateWithAPIKey()  // 验证配置完整性

    // 5. 获取 Qdrant Collection 名称
    collectionName := embeddingCfg.GetCollection(cfg.Qdrant.Collection)

    // 6. 初始化各组件
    // - MemeRepository (数据库)
    // - MemeVectorRepository (多模型追踪)
    // - QdrantRepository (向量数据库)
    // - ObjectStorage (S3/R2)
    // - VLMService (描述生成)
    // - EmbeddingProvider (向量生成)
    // - IngestService (聚合以上所有)

    // 7. 执行导入或重试
    if *retryPending {
        ingestService.RetryPending(ctx, *limit)
    } else {
        ingestService.IngestFromSource(ctx, src, *limit, opts)
    }
}
```

---

## 3. IngestService 核心实现

### 3.1 结构体定义

```go
// internal/service/ingest.go

type IngestService struct {
    memeRepo    *repository.MemeRepository       // memes 表操作
    vectorRepo  *repository.MemeVectorRepository // meme_vectors 表操作
    qdrantRepo  *repository.QdrantRepository     // Qdrant 向量库
    storage     storage.ObjectStorage            // S3/R2 存储
    vlm         *VLMService                      // 视觉语言模型
    embedding   EmbeddingProvider                // 嵌入模型
    logger      *logger.Logger
    workers     int                              // 并发 Worker 数
    batchSize   int                              // 每批获取数量
    collection  string                           // Qdrant Collection 名
}
```

### 3.2 IngestFromSource 主流程

```go
func (s *IngestService) IngestFromSource(
    ctx context.Context,
    src source.Source,
    limit int,
    opts *IngestOptions,
) (*IngestStats, error) {

    // 1. 创建通道
    itemsChan := make(chan source.MemeItem, s.workers*2)
    resultsChan := make(chan *processResult, s.workers*2)

    // 2. 启动 Worker Pool
    var wg sync.WaitGroup
    for i := 0; i < s.workers; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            s.worker(ctx, workerID, src.GetSourceID(), itemsChan, resultsChan, opts)
        }(i)
    }

    // 3. 分批获取数据并发送到通道
    go func() {
        cursor := ""
        totalFetched := 0

        for totalFetched < limit {
            remaining := limit - totalFetched
            batchLimit := min(s.batchSize, remaining)

            items, nextCursor, err := src.FetchBatch(ctx, cursor, batchLimit)
            if err != nil {
                break
            }

            for _, item := range items {
                select {
                case itemsChan <- item:
                case <-ctx.Done():
                    break
                }
            }

            totalFetched += len(items)
            cursor = nextCursor
            if nextCursor == "" {
                break
            }
        }
        close(itemsChan)
    }()

    // 4. 等待所有 Worker 完成
    go func() {
        wg.Wait()
        close(resultsChan)
    }()

    // 5. 收集结果
    stats := &IngestStats{}
    for result := range resultsChan {
        stats.TotalItems++
        if result.skipped {
            stats.SkippedItems++
        } else if result.err != nil {
            stats.FailedItems++
        } else {
            stats.ProcessedItems++
        }
    }

    return stats, nil
}
```

### 3.3 Worker 实现

```go
func (s *IngestService) worker(
    ctx context.Context,
    workerID int,
    sourceType string,
    items <-chan source.MemeItem,
    results chan<- *processResult,
    opts *IngestOptions,
) {
    for item := range items {
        // 响应 context 取消
        select {
        case <-ctx.Done():
            return
        default:
        }

        result := &processResult{sourceID: item.SourceID}

        // 处理单个 item
        if err := s.processItem(ctx, sourceType, &item, opts); err != nil {
            if err == errSkipDuplicate {
                result.skipped = true
            } else {
                result.err = err
                s.logger.WithError(err).Errorf("Worker %d: failed to process item", workerID)
            }
        }

        results <- result
    }
}
```

---

## 4. Source 接口与数据源适配器

### 4.1 Source 接口

```go
// internal/source/source.go

type Source interface {
    GetSourceID() string
    GetDisplayName() string
    FetchBatch(ctx context.Context, cursor string, limit int) ([]MemeItem, string, error)
    SupportsIncremental() bool
}

type MemeItem struct {
    SourceID   string   // 源内唯一标识
    URL        string   // 原始 URL（可选）
    LocalPath  string   // 本地文件路径
    Category   string   // 分类
    Tags       []string // 标签
    IsAnimated bool     // 是否动图
    Format     string   // 格式：jpg, png, gif, webp
}
```

### 4.2 ChineseBQB 适配器

```go
// internal/source/chinesebqb/adapter.go

type Adapter struct {
    repoPath string           // 仓库根路径
    items    []source.MemeItem
    loaded   bool
}

func (a *Adapter) FetchBatch(ctx context.Context, cursor string, limit int) ([]MemeItem, string, error) {
    // 延迟加载：首次调用时扫描目录
    if !a.loaded {
        a.loadItems()
        a.loaded = true
    }

    // cursor 是数组索引
    startIndex := parseIntOrZero(cursor)
    endIndex := min(startIndex+limit, len(a.items))

    batch := a.items[startIndex:endIndex]

    var nextCursor string
    if endIndex < len(a.items) {
        nextCursor = strconv.Itoa(endIndex)
    }

    return batch, nextCursor, nil
}

func (a *Adapter) loadItems() error {
    // 递归遍历目录
    filepath.Walk(a.repoPath, func(path string, info os.FileInfo, err error) error {
        if info.IsDir() {
            return nil
        }

        // 识别图片格式
        ext := filepath.Ext(path)
        format := parseFormat(ext)  // jpg, png, gif, webp

        // 从目录名获取分类
        category := filepath.Base(filepath.Dir(path))

        // 从文件名提取标签
        tags := extractTags(category, filepath.Base(path))

        a.items = append(a.items, source.MemeItem{
            SourceID:   generateSourceID(path),
            LocalPath:  path,
            Category:   category,
            Format:     format,
            IsAnimated: format == "gif",
            Tags:       tags,
        })
        return nil
    })

    sort.Slice(a.items, ...) // 保证顺序一致性
    return nil
}
```

### 4.3 Staging 适配器

```go
// internal/source/staging/adapter.go

type Adapter struct {
    basePath string // staging 根目录
    sourceID string // 数据源 ID（如 "fabiaoqing"）
    items    []source.MemeItem
    loaded   bool
}

func (a *Adapter) GetSourceID() string {
    return "staging:" + a.sourceID  // 前缀区分
}

func (a *Adapter) loadItems() error {
    manifestPath := filepath.Join(a.basePath, a.sourceID, "manifest.jsonl")

    file, _ := os.Open(manifestPath)
    scanner := bufio.NewScanner(file)

    // 逐行读取 JSONL
    for scanner.Scan() {
        var item ManifestItem
        json.Unmarshal(scanner.Bytes(), &item)
        // {
        //   "id": "abc123",
        //   "filename": "abc123.jpg",
        //   "category": "emoji",
        //   "tags": ["开心"],
        //   "source_url": "https://...",
        //   "is_animated": false,
        //   "format": "jpg"
        // }

        localPath := filepath.Join(a.basePath, a.sourceID, "images", item.Filename)

        // 验证文件存在
        if _, err := os.Stat(localPath); os.IsNotExist(err) {
            continue
        }

        a.items = append(a.items, source.MemeItem{
            SourceID:   fmt.Sprintf("%s_%s", a.sourceID, item.ID),
            URL:        item.SourceURL,
            LocalPath:  localPath,
            Category:   item.Category,
            Tags:       item.Tags,
            IsAnimated: item.IsAnimated,
            Format:     item.Format,
        })
    }

    return nil
}
```

---

## 5. Python 爬虫系统

### 5.1 目录结构

```
crawler/
├── src/emomo_crawler/
│   ├── cli.py           # CLI 入口
│   ├── base.py          # BaseCrawler 抽象类
│   ├── staging.py       # StagingManager
│   └── sources/
│       ├── __init__.py
│       └── fabiaoqing.py
├── pyproject.toml
└── README.md
```

### 5.2 BaseCrawler 抽象类

```python
# crawler/src/emomo_crawler/base.py

class BaseCrawler(ABC):
    def __init__(self, rate_limit=2.0, max_retries=3, timeout=30):
        self.rate_limit = rate_limit
        self.max_retries = max_retries
        self.timeout = timeout

    @property
    @abstractmethod
    def source_id(self) -> str:
        """返回源 ID，如 'fabiaoqing'"""
        pass

    @abstractmethod
    async def crawl(
        self,
        staging: StagingManager,
        limit: int,
        cursor: str | None = None,
    ) -> tuple[int, str | None]:
        """
        爬取数据

        Returns:
            (爬取数量, 下一页游标)
        """
        pass
```

### 5.3 FabiaoqingCrawler 实现

```python
# crawler/src/emomo_crawler/sources/fabiaoqing.py

class FabiaoqingCrawler(BaseCrawler):
    BASE_URL = "https://fabiaoqing.com"
    LIST_URL = "https://fabiaoqing.com/biaoqing/lists/page/{page}.html"

    async def crawl(self, staging, limit, cursor=None):
        page = int(cursor) if cursor else 1
        items_crawled = 0
        existing_ids = await staging.get_existing_ids(self.source_id)

        while items_crawled < limit:
            # 1. 获取列表页
            html = await self._fetch_page(self.LIST_URL.format(page=page))

            # 2. 解析 HTML
            soup = BeautifulSoup(html, 'html.parser')
            images = soup.select('a.bqb img')

            if not images:
                break

            # 3. 并发下载
            for img in images:
                if items_crawled >= limit:
                    break

                img_url = urljoin(self.BASE_URL, img.get('data-src'))
                item_id = generate_id_from_url(img_url)

                if item_id in existing_ids:
                    continue

                await self._download_and_save(staging, img_url, item_id)
                items_crawled += 1

            page += 1

        next_cursor = str(page) if items_crawled > 0 else None
        return items_crawled, next_cursor
```

### 5.4 StagingManager

```python
# crawler/src/emomo_crawler/staging.py

class StagingManager:
    def __init__(self, base_path: Path):
        self.base_path = base_path

    def save_image(self, source_id: str, item_id: str, data: bytes, fmt: str) -> str:
        """保存图片并返回文件名"""
        images_dir = self.base_path / source_id / "images"
        images_dir.mkdir(parents=True, exist_ok=True)

        filename = f"{item_id}.{fmt}"
        (images_dir / filename).write_bytes(data)
        return filename

    def append_manifest(self, source_id: str, item: StagingItem):
        """追加 manifest.jsonl"""
        manifest_path = self.base_path / source_id / "manifest.jsonl"
        with open(manifest_path, 'a') as f:
            f.write(json.dumps(item.to_dict(), ensure_ascii=False) + '\n')

    async def get_existing_ids(self, source_id: str) -> set[str]:
        """读取已存在的 ID 集合（用于去重）"""
        manifest_path = self.base_path / source_id / "manifest.jsonl"
        if not manifest_path.exists():
            return set()

        ids = set()
        for line in manifest_path.read_text().splitlines():
            item = json.loads(line)
            ids.add(item['id'])
        return ids
```

### 5.5 CLI 命令

```bash
# 爬取
uv run emomo-crawler crawl --source fabiaoqing --limit 100 --cursor 5 --threads 5

# 查看状态
uv run emomo-crawler staging list
uv run emomo-crawler staging stats --source fabiaoqing

# 清理
uv run emomo-crawler staging clean --source fabiaoqing
```

---

## 6. 多 Embedding 支持机制

### 6.1 配置结构

```yaml
# configs/config.yaml

embeddings:
  - name: jina
    provider: jina
    model: jina-embeddings-v3
    api_key_env: JINA_API_KEY
    dimensions: 1024
    collection: emomo
    is_default: true

  - name: qwen3
    provider: modelscope
    model: Qwen/Qwen3-Embedding-8B
    api_key_env: MODELSCOPE_API_KEY
    base_url_env: MODELSCOPE_BASE_URL
    dimensions: 4096
    collection: emomo-qwen3-embedding-8b
```

### 6.2 EmbeddingConfig 结构体

```go
// internal/config/embedding.go

type EmbeddingConfig struct {
    Name       string `mapstructure:"name"`
    Provider   string `mapstructure:"provider"`
    Model      string `mapstructure:"model"`
    APIKey     string `mapstructure:"api_key"`
    APIKeyEnv  string `mapstructure:"api_key_env"`   // 环境变量名
    BaseURL    string `mapstructure:"base_url"`
    BaseURLEnv string `mapstructure:"base_url_env"`
    Dimensions int    `mapstructure:"dimensions"`
    Collection string `mapstructure:"collection"`
    IsDefault  bool   `mapstructure:"is_default"`
}

func (c *EmbeddingConfig) ResolveEnvVars() {
    if c.APIKeyEnv != "" && c.APIKey == "" {
        c.APIKey = os.Getenv(c.APIKeyEnv)
    }
    if c.BaseURLEnv != "" && c.BaseURL == "" {
        c.BaseURL = os.Getenv(c.BaseURLEnv)
    }
}

func (c *EmbeddingConfig) ValidateWithAPIKey() error {
    if c.APIKey == "" {
        return fmt.Errorf("embedding %q: api_key is required", c.Name)
    }
    return nil
}
```

### 6.3 meme_vectors 表设计

```sql
CREATE TABLE meme_vectors (
    id TEXT PRIMARY KEY,
    meme_id TEXT NOT NULL,
    md5_hash TEXT NOT NULL,
    collection TEXT NOT NULL,           -- Qdrant collection 名
    embedding_model TEXT NOT NULL,       -- 模型名称
    qdrant_point_id TEXT NOT NULL,       -- Qdrant 点 ID
    status TEXT DEFAULT 'active',
    created_at TIMESTAMP DEFAULT NOW(),

    UNIQUE(md5_hash, collection),        -- 同一图片在同一 collection 只能有一个向量
    FOREIGN KEY (meme_id) REFERENCES memes(id)
);
```

### 6.4 去重逻辑

```go
// 检查 (MD5, Collection) 组合是否存在
if !opts.Force && s.vectorRepo != nil {
    exists, _ := s.vectorRepo.ExistsByMD5AndCollection(ctx, md5Hash, s.collection)
    if exists {
        return errSkipDuplicate
    }
}
```

### 6.5 资源复用

```go
// 检查是否已有 meme 记录
existingMeme, _ := s.memeRepo.GetByMD5Hash(ctx, md5Hash)

if existingMeme != nil {
    // 复用已有资源：
    // - memeID
    // - storageKey (S3 路径)
    // - storageURL
    // - vlmDescription (VLM 生成的描述)
    // - width, height

    // 只需要：
    // - 生成新的向量
    // - 写入对应的 Qdrant collection
    // - 创建 meme_vectors 记录
}
```

---

## 7. 处理管道详解

### 7.1 processItem 完整流程

```
┌─────────────────────────────────────────────────────────────┐
│                      processItem()                           │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ Stage 1: 读取和去重                                   │   │
│  │   1. 读取本地图片文件                                 │   │
│  │   2. 计算 MD5 哈希                                    │   │
│  │   3. 检查 meme_vectors(md5, collection) 是否存在     │   │
│  │      → 存在则 return errSkipDuplicate                │   │
│  └──────────────────────────────────────────────────────┘   │
│                           │                                  │
│                           ▼                                  │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ Stage 2: 资源复用判断                                 │   │
│  │   检查 memes(md5_hash) 是否存在                       │   │
│  │   → 存在: 复用 storageKey, vlmDescription            │   │
│  │   → 不存在: 执行 Stage 3                              │   │
│  └──────────────────────────────────────────────────────┘   │
│                           │                                  │
│              ┌────────────┴────────────┐                    │
│              ▼                         ▼                    │
│  ┌───────────────────────┐  ┌───────────────────────────┐   │
│  │ [复用路径]             │  │ [新建路径]                 │   │
│  │  复用:                 │  │  Stage 3: 新建资源         │   │
│  │  - memeID             │  │   1. 生成 UUID              │   │
│  │  - storageKey         │  │   2. 获取图片尺寸           │   │
│  │  - storageURL         │  │   3. 调用 VLM 生成描述      │   │
│  │  - vlmDescription     │  │   4. 上传到 S3              │   │
│  │  - width, height      │  │   5. 创建 meme 记录         │   │
│  └───────────┬───────────┘  └───────────────┬───────────┘   │
│              │                              │                │
│              └──────────────┬───────────────┘                │
│                             ▼                                │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ Stage 4: 向量生成和存储                               │   │
│  │   1. 调用 EmbeddingProvider.Embed(vlmDescription)    │   │
│  │   2. 生成新的 pointID (UUID)                          │   │
│  │   3. 构建 Payload (包含搜索所需元数据)               │   │
│  │   4. Qdrant.Upsert(pointID, vector, payload)         │   │
│  │   5. 创建 meme_vectors 记录                          │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 7.2 Qdrant Payload 结构

```go
type MemePayload struct {
    MemeID         string   `json:"meme_id"`
    SourceType     string   `json:"source_type"`
    Category       string   `json:"category"`
    IsAnimated     bool     `json:"is_animated"`
    Tags           []string `json:"tags"`
    VLMDescription string   `json:"vlm_description"`
    StorageURL     string   `json:"storage_url"`
}
```

### 7.3 S3 存储路径设计

```
存储 Key 格式: {md5前2位}/{md5}.{格式}
示例: a1/a1b2c3d4e5f6g7h8.jpg

优势:
- 前缀分片，避免热点
- 基于内容哈希，天然去重
- 格式后缀便于 CDN 识别
```

---

## 8. 并发模型与性能

### 8.1 Worker Pool 架构

```
                    ┌─────────────────┐
                    │  FetchBatch()   │
                    │  (生产者)        │
                    └────────┬────────┘
                             │
                             ▼
                  ┌─────────────────────┐
                  │    itemsChan        │
                  │  缓冲: workers * 2  │
                  └──────────┬──────────┘
                             │
         ┌───────────────────┼───────────────────┐
         ▼                   ▼                   ▼
   ┌──────────┐       ┌──────────┐       ┌──────────┐
   │ Worker 0 │       │ Worker 1 │       │ Worker N │
   └────┬─────┘       └────┬─────┘       └────┬─────┘
        │                  │                  │
        └──────────────────┼──────────────────┘
                           ▼
                ┌─────────────────────┐
                │    resultsChan      │
                │  缓冲: workers * 2  │
                └──────────┬──────────┘
                           │
                           ▼
                  ┌─────────────────┐
                  │ 结果收集 (atomic) │
                  └─────────────────┘
```

### 8.2 性能指标

```
配置:
- workers: 8
- batchSize: 100

单个 item 处理时间分布:
┌────────────────────┬──────────┬─────────┐
│ 阶段               │ 时间     │ 占比    │
├────────────────────┼──────────┼─────────┤
│ 读取图片           │ 10ms     │ 0.3%    │
│ MD5 计算           │ 1ms      │ 0.03%   │
│ VLM API 调用       │ 2000ms   │ 63%     │ ← 瓶颈
│ S3 上传            │ 500ms    │ 16%     │
│ 数据库写入         │ 50ms     │ 1.6%    │
│ Embedding 生成     │ 500ms    │ 16%     │
│ Qdrant 写入        │ 100ms    │ 3%      │
├────────────────────┼──────────┼─────────┤
│ 总计               │ ~3161ms  │ 100%    │
└────────────────────┴──────────┴─────────┘

理论吞吐量:
= workers / 平均处理时间
= 8 / 3.161s
≈ 2.5 items/秒

每小时处理量 ≈ 9000 items
```

### 8.3 优化建议

| 优化点 | 方案 | 预期提升 |
|--------|------|----------|
| 增加 Workers | 16-32 (I/O 密集型) | +50-100% |
| VLM 批量调用 | 如果 API 支持 batch | +200% |
| S3 并发上传 | 多部分上传 | +20% |
| Qdrant 批量写入 | batch upsert | +30% |
| 资源复用 | 已实现 | 跳过 VLM 调用 |

---

## 9. 错误处理与事务回滚

### 9.1 分层回滚策略

```go
// 回滚函数定义
rollbackStorage := func() {
    if uploaded {
        s.storage.Delete(ctx, storageKey)
    }
}

rollbackMeme := func() {
    if createdNewMeme && memeID != "" {
        s.memeRepo.Delete(ctx, memeID)
    }
}

// 各阶段的回滚逻辑
┌─────────────────────────────────────────────────────────────┐
│ 阶段                    │ 失败时回滚                         │
├─────────────────────────┼────────────────────────────────────┤
│ VLM 生成描述失败        │ rollbackStorage()                  │
│ 数据库保存 meme 失败    │ rollbackStorage()                  │
│ Embedding 生成失败      │ rollbackMeme() + rollbackStorage() │
│ Qdrant 写入失败         │ rollbackMeme() + rollbackStorage() │
│ meme_vectors 保存失败   │ Qdrant.Delete() +                  │
│                         │ rollbackMeme() + rollbackStorage() │
└─────────────────────────┴────────────────────────────────────┘
```

### 9.2 错误场景与恢复

#### VLM API 超时

```
[场景] OpenAI API 响应超时 (>30s)

[行为]
- item 标记为 failed
- 不创建任何记录

[恢复]
- 重新运行 ingest (自动跳过已处理的)
- 或 --force 强制重新处理
```

#### S3 上传失败

```
[场景] S3 连接中断

[行为]
- 回滚已创建的 meme 记录
- item 标记为 failed

[恢复]
- 修复网络/S3 服务
- 重新运行 ingest
```

#### Qdrant 写入失败

```
[场景] Qdrant 连接丢失

[行为]
- 回滚 meme 和 S3
- item 标记为 failed

[恢复]
- 修复 Qdrant 连接
- 重新运行 ingest
```

### 9.3 RetryPending 机制

```go
func (s *IngestService) RetryPending(ctx context.Context, limit int) (*IngestStats, error) {
    // 查询所有 pending 状态的 meme
    memes, _ := s.memeRepo.ListByStatus(ctx, domain.MemeStatusPending, limit, 0)

    for _, meme := range memes {
        // 1. 从 S3 下载图片
        reader, _ := s.storage.Download(ctx, meme.StorageKey)

        // 2. 重新生成 VLM 描述
        description, _ := s.vlm.DescribeImage(ctx, imageData, meme.Format)

        // 3. 生成向量
        embedding, _ := s.embedding.Embed(ctx, description)

        // 4. 写入 Qdrant
        s.qdrantRepo.Upsert(ctx, meme.ID, embedding, payload)

        // 5. 更新状态为 active
        meme.Status = domain.MemeStatusActive
        s.memeRepo.Update(ctx, &meme)
    }

    return stats, nil
}
```

---

## 10. Qdrant 集成

### 10.1 连接配置

```go
// internal/repository/qdrant_repo.go

type QdrantConnectionConfig struct {
    Host            string  // "localhost" 或 "cloud.qdrant.io"
    Port            int     // 6334 (gRPC)
    Collection      string  // 集合名称
    APIKey          string  // Qdrant Cloud API Key
    UseTLS          bool    // 启用 TLS
    VectorDimension int     // 向量维度
}

func NewQdrantRepository(cfg *QdrantConnectionConfig) (*QdrantRepository, error) {
    // TLS 配置
    useTLS := cfg.UseTLS || cfg.APIKey != ""

    if useTLS {
        // 生产环境: TLS + API Key
        tlsConfig := &tls.Config{MinVersion: tls.VersionTLS13}
        creds := credentials.NewTLS(tlsConfig)
        opts = append(opts, grpc.WithTransportCredentials(creds))

        if cfg.APIKey != "" {
            opts = append(opts, grpc.WithUnaryInterceptor(apiKeyInterceptor(cfg.APIKey)))
        }
    } else {
        // 本地开发: 无 TLS
        opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
    }

    conn, _ := grpc.NewClient(addr, opts...)
    return &QdrantRepository{
        conn:            conn,
        pointsClient:    pb.NewPointsClient(conn),
        collectionsClient: pb.NewCollectionsClient(conn),
        collectionName:  cfg.Collection,
        vectorDimension: cfg.VectorDimension,
    }, nil
}
```

### 10.2 集合管理

```go
func (r *QdrantRepository) EnsureCollection(ctx context.Context) error {
    req := &pb.CreateCollection{
        CollectionName: r.collectionName,
        VectorsConfig: &pb.VectorsConfig{
            Config: &pb.VectorsConfig_Params{
                Params: &pb.VectorParams{
                    Size:     uint64(r.vectorDimension),
                    Distance: pb.Distance_Cosine,
                },
            },
        },
    }

    _, err := r.collectionsClient.Create(ctx, req)
    if err != nil && strings.Contains(err.Error(), "already exists") {
        return nil
    }
    return err
}
```

### 10.3 向量操作

```go
func (r *QdrantRepository) Upsert(
    ctx context.Context,
    pointID string,
    vector []float32,
    payload *MemePayload,
) error {
    point := &pb.PointStruct{
        Id: &pb.PointId{
            PointIdOptions: &pb.PointId_Uuid{Uuid: pointID},
        },
        Vectors: &pb.Vectors{
            VectorsOptions: &pb.Vectors_Vector{
                Vector: &pb.Vector{Data: vector},
            },
        },
        Payload: payloadToQdrant(payload),
    }

    req := &pb.UpsertPoints{
        CollectionName: r.collectionName,
        Points:         []*pb.PointStruct{point},
    }

    _, err := r.pointsClient.Upsert(ctx, req)
    return err
}

func (r *QdrantRepository) Delete(ctx context.Context, pointID string) error {
    req := &pb.DeletePoints{
        CollectionName: r.collectionName,
        Points: &pb.PointsSelector{
            PointsSelectorOneOf: &pb.PointsSelector_Points{
                Points: &pb.PointsIdsList{
                    Ids: []*pb.PointId{{PointIdOptions: &pb.PointId_Uuid{Uuid: pointID}}},
                },
            },
        },
    }

    _, err := r.pointsClient.Delete(ctx, req)
    return err
}
```

---

## 11. 配置参考

### 11.1 完整配置示例

```yaml
# configs/config.yaml

server:
  port: 8080
  mode: debug

database:
  driver: postgres
  port: 5432
  user: postgres
  dbname: emomo

qdrant:
  host: localhost
  port: 6334
  collection: emomo

storage:
  type: r2
  endpoint: ""      # STORAGE_ENDPOINT
  use_ssl: true
  bucket: emomo
  public_url: ""    # STORAGE_PUBLIC_URL

vlm:
  provider: openai
  model: ""         # VLM_MODEL
  base_url: https://openrouter.ai/api/v1

embeddings:
  - name: jina
    provider: jina
    model: jina-embeddings-v3
    api_key_env: JINA_API_KEY
    dimensions: 1024
    collection: emomo
    is_default: true

  - name: qwen3
    provider: modelscope
    model: Qwen/Qwen3-Embedding-8B
    api_key_env: MODELSCOPE_API_KEY
    base_url_env: MODELSCOPE_BASE_URL
    dimensions: 4096
    collection: emomo-qwen3-embedding-8b

ingest:
  workers: 8
  batch_size: 100
  retry_count: 3

sources:
  chinesebqb:
    enabled: true
    repo_path: ./data/ChineseBQB
  staging:
    path: ./data/staging
```

### 11.2 环境变量

```bash
# VLM
VLM_MODEL=gpt-4o-mini
OPENAI_API_KEY=sk-...
OPENAI_BASE_URL=https://openrouter.ai/api/v1

# Embedding
JINA_API_KEY=jina-...
MODELSCOPE_API_KEY=...
MODELSCOPE_BASE_URL=https://api-inference.modelscope.cn/v1

# Storage
STORAGE_ENDPOINT=xxx.r2.cloudflarestorage.com
STORAGE_ACCESS_KEY=...
STORAGE_SECRET_KEY=...
STORAGE_PUBLIC_URL=https://cdn.example.com

# Qdrant
QDRANT_HOST=localhost
QDRANT_API_KEY=...

# Database
DATABASE_URL=postgresql://user:pass@localhost:5432/emomo
```

---

## 总结

| 维度 | 特性 |
|------|------|
| **吞吐量** | 2-3 items/秒 (受限于 VLM API) |
| **并发模型** | Worker Pool + 缓冲通道 |
| **可靠性** | 分层回滚 + RetryPending |
| **去重机制** | MD5 + Collection 组合 |
| **资源复用** | S3 + VLM 描述按 MD5 复用 |
| **多模型支持** | 独立 Collection + EmbeddingConfig |
| **可扩展性** | Source/EmbeddingProvider 接口化 |
