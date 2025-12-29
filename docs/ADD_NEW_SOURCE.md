# 添加新表情包来源指南

本文档指导你如何为 Emomo 添加新的表情包来源。

## 目录
- [快速开始](#快速开始)
- [方案选择](#方案选择)
- [实现步骤](#实现步骤)
- [常见来源示例](#常见来源示例)

---

## 快速开始

### 理解 Source 接口

所有数据源必须实现以下接口（`internal/source/interface.go`）：

```go
type Source interface {
    GetSourceID() string                    // 唯一标识（如 "chinesebqb"）
    GetDisplayName() string                  // 显示名称（如 "ChineseBQB"）
    FetchBatch(ctx, cursor, limit) (items, nextCursor, error)  // 分批获取
    SupportsIncremental() bool               // 是否支持增量更新
}
```

**MemeItem 数据结构**：
```go
type MemeItem struct {
    SourceID    string      // 来源内唯一ID（如 "panda/happy.jpg"）
    URL         string      // 图片URL或本地路径
    Category    string      // 分类/标签
    Tags        []string    // 标签列表
    IsAnimated  bool        // 是否为动图
    Format      string      // jpeg/png/gif/webp
    LocalPath   string      // 本地文件路径（可选）
}
```

---

## 方案选择

### 方案 1：本地文件系统来源 ⭐️ 最简单

**适用场景**：
- 你有现成的表情包文件夹
- 表情包数量不大（< 10万）
- 按目录组织好分类

**优点**：
- 实现简单，复制 ChineseBQB 适配器即可
- 不需要网络请求
- 可以直接使用现有文件

**缺点**：
- 需要手动下载更新
- 占用磁盘空间

**参考实现**：`internal/source/chinesebqb/adapter.go`

---

### 方案 2：在线 API 来源 ⭐️⭐️⭐️ 推荐

**适用场景**：
- 从公开 API 获取（如 Giphy、Tenor、Imgur）
- 需要实时更新
- 表情包数量大

**优点**：
- 自动获取最新内容
- 支持增量更新
- 节省本地存储

**缺点**：
- 需要处理 API 限流
- 依赖网络稳定性
- 可能需要 API Key

**适合的来源**：
- [Giphy API](https://developers.giphy.com/)
- [Tenor API](https://tenor.com/gifapi/documentation)
- [Imgur API](https://api.imgur.com/)
- 自建爬虫

---

### 方案 3：数据库来源

**适用场景**：
- 从现有数据库迁移
- 已有表情包管理系统

**优点**：
- 直接复用现有数据
- 支持复杂查询

**缺点**：
- 需要额外数据库连接
- 实现复杂度较高

---

## 实现步骤

### Step 1: 创建适配器文件

```bash
# 创建新来源目录
mkdir -p internal/source/your_source_name

# 创建适配器文件
touch internal/source/your_source_name/adapter.go
```

### Step 2: 实现 Source 接口

选择以下模板之一：

#### 模板 A：本地文件系统来源

```go
package your_source_name

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "strconv"
    "strings"

    "github.com/timmy/emomo/internal/source"
)

const (
    SourceID   = "your_source_id"        // 如 "emojikitchen"
    SourceName = "Your Source Name"      // 如 "Emoji Kitchen"
)

type Adapter struct {
    repoPath string
    items    []source.MemeItem
    loaded   bool
}

func NewAdapter(repoPath string) *Adapter {
    return &Adapter{repoPath: repoPath}
}

func (a *Adapter) GetSourceID() string {
    return SourceID
}

func (a *Adapter) GetDisplayName() string {
    return SourceName
}

func (a *Adapter) SupportsIncremental() bool {
    return false  // 静态文件系统不支持增量
}

func (a *Adapter) FetchBatch(ctx context.Context, cursor string, limit int) ([]source.MemeItem, string, error) {
    // 首次加载所有文件
    if !a.loaded {
        if err := a.loadItems(); err != nil {
            return nil, "", err
        }
        a.loaded = true
    }

    // 解析游标
    startIndex := 0
    if cursor != "" {
        startIndex, _ = strconv.Atoi(cursor)
    }

    if startIndex >= len(a.items) {
        return []source.MemeItem{}, "", nil
    }

    // 获取批次
    endIndex := startIndex + limit
    if endIndex > len(a.items) {
        endIndex = len(a.items)
    }

    batch := a.items[startIndex:endIndex]

    // 计算下一个游标
    nextCursor := ""
    if endIndex < len(a.items) {
        nextCursor = strconv.Itoa(endIndex)
    }

    return batch, nextCursor, nil
}

func (a *Adapter) loadItems() error {
    // 检查路径存在
    if _, err := os.Stat(a.repoPath); os.IsNotExist(err) {
        return fmt.Errorf("repository path does not exist: %s", a.repoPath)
    }

    a.items = []source.MemeItem{}

    // 遍历目录
    err := filepath.Walk(a.repoPath, func(path string, info os.FileInfo, err error) error {
        if err != nil || info.IsDir() {
            return err
        }

        // 过滤图片文件
        ext := strings.ToLower(filepath.Ext(path))
        format := ""
        switch ext {
        case ".jpg", ".jpeg":
            format = "jpeg"
        case ".png":
            format = "png"
        case ".gif":
            format = "gif"
        case ".webp":
            format = "webp"
        default:
            return nil
        }

        // 提取分类（父目录名）
        category := filepath.Base(filepath.Dir(path))

        // 生成 SourceID
        relPath, _ := filepath.Rel(a.repoPath, path)
        sourceID := strings.ReplaceAll(relPath, string(os.PathSeparator), "_")

        // 构建 MemeItem
        item := source.MemeItem{
            SourceID:   sourceID,
            URL:        path,
            LocalPath:  path,
            Category:   category,
            Format:     format,
            IsAnimated: format == "gif",
            Tags:       extractTags(category, info.Name()),
        }

        a.items = append(a.items, item)
        return nil
    })

    return err
}

// 从分类和文件名提取标签
func extractTags(category, filename string) []string {
    tags := []string{}

    if category != "" {
        tags = append(tags, category)
    }

    // 从文件名提取（按下划线分割）
    name := strings.TrimSuffix(filename, filepath.Ext(filename))
    parts := strings.Split(name, "_")
    for _, part := range parts {
        part = strings.TrimSpace(part)
        if len(part) > 1 {
            tags = append(tags, part)
        }
    }

    return uniqueStrings(tags)
}

func uniqueStrings(strs []string) []string {
    seen := make(map[string]bool)
    result := []string{}
    for _, s := range strs {
        if !seen[s] {
            seen[s] = true
            result = append(result, s)
        }
    }
    return result
}
```

#### 模板 B：在线 API 来源

```go
package your_source_name

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "strconv"

    "github.com/timmy/emomo/internal/source"
)

const (
    SourceID   = "your_api_source"
    SourceName = "Your API Source"
)

type Adapter struct {
    apiURL string
    apiKey string
    client *http.Client
}

func NewAdapter(apiURL, apiKey string) *Adapter {
    return &Adapter{
        apiURL: apiURL,
        apiKey: apiKey,
        client: &http.Client{},
    }
}

func (a *Adapter) GetSourceID() string {
    return SourceID
}

func (a *Adapter) GetDisplayName() string {
    return SourceName
}

func (a *Adapter) SupportsIncremental() bool {
    return true  // API 支持增量更新
}

func (a *Adapter) FetchBatch(ctx context.Context, cursor string, limit int) ([]source.MemeItem, string, error) {
    // 解析游标（页码或偏移量）
    offset := 0
    if cursor != "" {
        offset, _ = strconv.Atoi(cursor)
    }

    // 构建请求
    url := fmt.Sprintf("%s?offset=%d&limit=%d", a.apiURL, offset, limit)
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, "", err
    }

    // 添加认证
    if a.apiKey != "" {
        req.Header.Set("Authorization", "Bearer "+a.apiKey)
    }

    // 发送请求
    resp, err := a.client.Do(req)
    if err != nil {
        return nil, "", err
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        body, _ := io.ReadAll(resp.Body)
        return nil, "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
    }

    // 解析响应
    var apiResp APIResponse
    if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
        return nil, "", err
    }

    // 转换为 MemeItem
    items := make([]source.MemeItem, 0, len(apiResp.Data))
    for _, apiItem := range apiResp.Data {
        items = append(items, source.MemeItem{
            SourceID:   apiItem.ID,
            URL:        apiItem.ImageURL,
            Category:   apiItem.Category,
            Tags:       apiItem.Tags,
            Format:     detectFormat(apiItem.ImageURL),
            IsAnimated: apiItem.IsGIF,
        })
    }

    // 计算下一个游标
    nextCursor := ""
    if len(items) == limit {
        nextCursor = strconv.Itoa(offset + limit)
    }

    return items, nextCursor, nil
}

// API 响应结构（根据实际 API 调整）
type APIResponse struct {
    Data []APIItem `json:"data"`
}

type APIItem struct {
    ID       string   `json:"id"`
    ImageURL string   `json:"image_url"`
    Category string   `json:"category"`
    Tags     []string `json:"tags"`
    IsGIF    bool     `json:"is_gif"`
}

func detectFormat(url string) string {
    if strings.HasSuffix(url, ".gif") {
        return "gif"
    } else if strings.HasSuffix(url, ".png") {
        return "png"
    } else if strings.HasSuffix(url, ".webp") {
        return "webp"
    }
    return "jpeg"
}
```

### Step 3: 配置新来源

编辑 `configs/config.yaml`：

```yaml
sources:
  chinesebqb:
    enabled: true
    repo_path: ./data/ChineseBQB

  your_source_name:  # 添加你的新来源
    enabled: true
    repo_path: ./data/YourSource  # 本地文件系统
    # 或
    api_url: https://api.example.com/memes  # API 来源
    api_key: your_api_key
```

编辑 `internal/config/config.go`：

```go
type Config struct {
    // ... 现有字段

    Sources struct {
        ChineseBQB ChineseBQBConfig `yaml:"chinesebqb"`

        // 添加你的新来源配置
        YourSource YourSourceConfig `yaml:"your_source_name"`
    } `yaml:"sources"`
}

// 添加配置结构
type YourSourceConfig struct {
    Enabled  bool   `yaml:"enabled"`
    RepoPath string `yaml:"repo_path"`
    // 或 API 配置
    APIURL   string `yaml:"api_url"`
    APIKey   string `yaml:"api_key"`
}
```

### Step 4: 注册到 Ingest CLI

编辑 `cmd/ingest/main.go`：

```go
import (
    // ... 现有导入
    "github.com/timmy/emomo/internal/source/your_source_name"
)

func main() {
    // ... 现有代码

    var src source.Source
    switch *sourceType {
    case "chinesebqb":
        src = chinesebqb.NewAdapter(cfg.Sources.ChineseBQB.RepoPath)

    // 添加你的新来源
    case "your_source_name":
        src = your_source_name.NewAdapter(cfg.Sources.YourSource.RepoPath)
        // 或 API 来源
        // src = your_source_name.NewAdapter(cfg.Sources.YourSource.APIURL, cfg.Sources.YourSource.APIKey)

    default:
        logger.Fatal("Unknown source type")
    }

    // ... 其余代码
}
```

### Step 5: 测试运行

```bash
# 重新编译
go build -o ingest ./cmd/ingest

# 测试导入（限制10条）
./ingest --source=your_source_name --limit=10

# 查看导入状态
./ingest --retry --limit=10

# 正式导入（大批量）
./ingest --source=your_source_name --limit=10000
```

---

## 常见来源示例

### 1. Giphy API

```go
package giphy

const (
    SourceID   = "giphy"
    SourceName = "Giphy"
    BaseURL    = "https://api.giphy.com/v1/gifs/trending"
)

type Adapter struct {
    apiKey string
    client *http.Client
}

func NewAdapter(apiKey string) *Adapter {
    return &Adapter{
        apiKey: apiKey,
        client: &http.Client{Timeout: 10 * time.Second},
    }
}

func (a *Adapter) FetchBatch(ctx context.Context, cursor string, limit int) ([]source.MemeItem, string, error) {
    offset := 0
    if cursor != "" {
        offset, _ = strconv.Atoi(cursor)
    }

    url := fmt.Sprintf("%s?api_key=%s&limit=%d&offset=%d", BaseURL, a.apiKey, limit, offset)

    resp, err := a.client.Get(url)
    if err != nil {
        return nil, "", err
    }
    defer resp.Body.Close()

    var giphyResp GiphyResponse
    json.NewDecoder(resp.Body).Decode(&giphyResp)

    items := make([]source.MemeItem, 0)
    for _, gif := range giphyResp.Data {
        items = append(items, source.MemeItem{
            SourceID:   gif.ID,
            URL:        gif.Images.Original.URL,
            Category:   "trending",
            Tags:       []string{"giphy", "trending"},
            Format:     "gif",
            IsAnimated: true,
        })
    }

    nextCursor := ""
    if len(items) == limit {
        nextCursor = strconv.Itoa(offset + limit)
    }

    return items, nextCursor, nil
}

type GiphyResponse struct {
    Data []struct {
        ID     string `json:"id"`
        Images struct {
            Original struct {
                URL string `json:"url"`
            } `json:"original"`
        } `json:"images"`
    } `json:"data"`
}
```

### 2. Telegram Sticker Packs（本地下载）

```bash
# 使用 Telegram API 下载贴纸包
# 假设你已经下载到 ./data/TelegramStickers/

# 目录结构：
# ./data/TelegramStickers/
#   ├── pepe/
#   │   ├── happy.webp
#   │   └── sad.webp
#   └── doge/
#       ├── wow.webp
#       └── such.webp
```

实现与 ChineseBQB 类似，只需修改：
- SourceID: `"telegram"`
- 支持 `.webp` 格式
- 添加 TGS（Telegram 动态贴纸）支持

### 3. 自建爬虫（示例：知乎表情包）

```go
package zhihu

import (
    "github.com/gocolly/colly"
)

type Adapter struct {
    collector *colly.Collector
    items     []source.MemeItem
}

func NewAdapter() *Adapter {
    c := colly.NewCollector()

    a := &Adapter{collector: c}

    // 配置爬虫规则
    c.OnHTML("img.meme", func(e *colly.HTMLElement) {
        item := source.MemeItem{
            SourceID:   e.Attr("data-id"),
            URL:        e.Attr("src"),
            Category:   e.Attr("data-category"),
            Tags:       []string{"zhihu"},
            Format:     "jpeg",
            IsAnimated: false,
        }
        a.items = append(a.items, item)
    })

    return a
}

func (a *Adapter) FetchBatch(ctx context.Context, cursor string, limit int) ([]source.MemeItem, string, error) {
    page := 1
    if cursor != "" {
        page, _ = strconv.Atoi(cursor)
    }

    url := fmt.Sprintf("https://zhihu.com/memes?page=%d", page)
    a.collector.Visit(url)

    // 返回爬取的结果
    return a.items, strconv.Itoa(page+1), nil
}
```

---

## 最佳实践

### 1. 错误处理

```go
func (a *Adapter) FetchBatch(...) ([]source.MemeItem, string, error) {
    // 添加超时控制
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    // 捕获 panic
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("panic: %v", r)
        }
    }()

    // 重试逻辑
    for retry := 0; retry < 3; retry++ {
        items, nextCursor, err := a.fetchWithRetry(ctx, cursor, limit)
        if err == nil {
            return items, nextCursor, nil
        }
        time.Sleep(time.Second * time.Duration(retry+1))
    }

    return nil, "", fmt.Errorf("failed after 3 retries")
}
```

### 2. 限流处理

```go
import "golang.org/x/time/rate"

type Adapter struct {
    limiter *rate.Limiter  // 限制 API 调用频率
}

func NewAdapter(apiKey string) *Adapter {
    return &Adapter{
        limiter: rate.NewLimiter(rate.Every(time.Second), 10), // 每秒最多10次
    }
}

func (a *Adapter) FetchBatch(...) {
    if err := a.limiter.Wait(ctx); err != nil {
        return nil, "", err
    }
    // ... 执行请求
}
```

### 3. 缓存优化

```go
type Adapter struct {
    cache map[string][]source.MemeItem
    mu    sync.RWMutex
}

func (a *Adapter) FetchBatch(ctx context.Context, cursor string, limit int) {
    // 检查缓存
    a.mu.RLock()
    if cached, ok := a.cache[cursor]; ok {
        a.mu.RUnlock()
        return cached, "", nil
    }
    a.mu.RUnlock()

    // 获取数据
    items, nextCursor, err := a.fetchFromAPI(ctx, cursor, limit)

    // 更新缓存
    a.mu.Lock()
    a.cache[cursor] = items
    a.mu.Unlock()

    return items, nextCursor, err
}
```

### 4. 日志记录

```go
import "go.uber.org/zap"

func (a *Adapter) FetchBatch(...) {
    logger.Info("fetching batch",
        zap.String("source", a.GetSourceID()),
        zap.String("cursor", cursor),
        zap.Int("limit", limit),
    )

    items, nextCursor, err := a.fetch(ctx, cursor, limit)

    logger.Info("batch fetched",
        zap.Int("count", len(items)),
        zap.String("next_cursor", nextCursor),
        zap.Error(err),
    )

    return items, nextCursor, err
}
```

---

## 调试技巧

### 1. 单元测试

创建 `adapter_test.go`：

```go
package your_source_name

import (
    "context"
    "testing"
)

func TestFetchBatch(t *testing.T) {
    adapter := NewAdapter("/path/to/test/data")

    items, cursor, err := adapter.FetchBatch(context.Background(), "", 10)

    if err != nil {
        t.Fatalf("FetchBatch failed: %v", err)
    }

    if len(items) == 0 {
        t.Error("Expected items, got none")
    }

    t.Logf("Fetched %d items, next cursor: %s", len(items), cursor)
}
```

运行测试：
```bash
go test ./internal/source/your_source_name/...
```

### 2. 独立测试脚本

创建 `test_source.go`：

```go
package main

import (
    "context"
    "fmt"
    "your_source_name"
)

func main() {
    adapter := your_source_name.NewAdapter("/path/to/data")

    items, cursor, err := adapter.FetchBatch(context.Background(), "", 5)
    if err != nil {
        panic(err)
    }

    for i, item := range items {
        fmt.Printf("[%d] %s - %s (%s)\n", i, item.SourceID, item.Category, item.Format)
        fmt.Printf("     URL: %s\n", item.URL)
        fmt.Printf("     Tags: %v\n", item.Tags)
    }

    fmt.Printf("\nNext cursor: %s\n", cursor)
}
```

运行：
```bash
go run test_source.go
```

---

## 常见问题

### Q1: 如何处理大量文件（>10万）？

使用流式处理而非一次性加载：

```go
func (a *Adapter) FetchBatch(ctx context.Context, cursor string, limit int) {
    // 不要在 loadItems() 中加载所有文件
    // 而是动态扫描目录的一部分
    return a.scanDirectory(cursor, limit)
}
```

### Q2: API 有速率限制怎么办？

添加限流器和退避策略：

```go
import "golang.org/x/time/rate"

limiter := rate.NewLimiter(rate.Every(time.Second), 10)
limiter.Wait(ctx)
```

### Q3: 如何支持增量更新？

实现 `SupportsIncremental() = true`，并使用时间戳游标：

```go
func (a *Adapter) FetchBatch(ctx context.Context, cursor string, limit int) {
    lastUpdate := parseTime(cursor)  // 解析上次更新时间

    // 只获取 lastUpdate 之后的新内容
    items := fetchNewItemsSince(lastUpdate)

    nextCursor := time.Now().Format(time.RFC3339)
    return items, nextCursor, nil
}
```

### Q4: 图片 URL 过期怎么办？

- 方案1：下载到本地后设置 `LocalPath`
- 方案2：使用 S3 URL（Emomo 会自动上传）
- 方案3：实现 URL 刷新逻辑

---

## 推荐的公开表情包来源

1. **Giphy** - https://developers.giphy.com/
2. **Tenor** - https://tenor.com/gifapi
3. **GitHub 表情包仓库**：
   - https://github.com/matheusfelipeog/beautiful-docs (各种表情包)
   - https://github.com/zenorocha/alfred-workflows (Alfred 表情包)
4. **Telegram Sticker Sets** - 使用 Bot API
5. **Reddit** - /r/MemeEconomy, /r/dankmemes (需要爬虫)
6. **自定义爬虫**：
   - 知乎表情包
   - B站表情包
   - 微博表情包

---

## 下一步

1. 选择你想添加的来源类型
2. 复制对应模板创建适配器
3. 配置 `config.yaml` 和注册到 `cmd/ingest/main.go`
4. 测试运行 `./ingest --source=your_source --limit=10`
5. 检查 API 响应或本地文件是否正确导入

如有问题，请查看现有实现 `internal/source/chinesebqb/adapter.go` 作为参考。
