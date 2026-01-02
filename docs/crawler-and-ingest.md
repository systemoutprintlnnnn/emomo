# 爬虫与数据导入指南

本文档介绍如何使用 Python 爬虫抓取表情包,并导入到 Emomo 系统。

## 整体流程

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   爬虫抓取   │ ──▶ │  Staging    │ ──▶ │   导入系统   │
│  (Python)   │     │  暂存区     │     │    (Go)     │
└─────────────┘     └─────────────┘     └─────────────┘
                          │
                          ▼
                    data/staging/
                    └── fabiaoqing/
                        ├── manifest.jsonl  # 元数据
                        └── images/         # 图片文件
```

## 1. 爬虫设置

### 1.1 安装依赖

```bash
cd crawler
uv sync
```

### 1.2 查看可用命令

```bash
uv run emomo-crawler --help
```

## 2. 爬取表情包

### 2.1 基础用法

```bash
# 从 fabiaoqing.com 爬取 100 张表情包
uv run emomo-crawler crawl --source fabiaoqing --limit 100
```

### 2.2 从指定页码开始

```bash
# 从第 10 页开始爬取
uv run emomo-crawler crawl --source fabiaoqing --limit 100 --cursor 10
```

### 2.3 调整并发线程数

```bash
# 使用 3 个线程下载 (默认 5)
uv run emomo-crawler crawl --source fabiaoqing --limit 100 --threads 3
```

### 2.4 爬取参数说明

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `--source`, `-s` | 数据源名称 | fabiaoqing |
| `--limit`, `-l` | 最大爬取数量 | 100 |
| `--cursor`, `-c` | 起始页码 | 1 |
| `--threads`, `-t` | 下载线程数 | 5 |

## 3. 查看 Staging 状态

### 3.1 列出所有数据源

```bash
uv run emomo-crawler staging list
```

输出示例:
```
Available staging sources:
  fabiaoqing: 150 items
```

### 3.2 查看详细统计

```bash
uv run emomo-crawler staging stats --source fabiaoqing
```

输出示例:
```
Staging statistics for fabiaoqing:
  Total images: 150
  Total size: 12.5 MB
  Categories:
    表情包: 120
    熊猫头: 30
  Formats:
    gif: 80
    jpg: 50
    png: 20
```

### 3.3 清理 Staging 数据

```bash
# 清理指定来源
uv run emomo-crawler staging clean --source fabiaoqing

# 清理所有来源
uv run emomo-crawler staging clean-all
```

## 4. 导入到系统

### 4.1 前置条件

确保以下服务已启动:
- 数据库（SQLite 或 PostgreSQL）
- Qdrant 向量数据库
- S3/R2 对象存储已配置

### 4.2 执行导入

使用导入脚本（推荐，无需预先编译）:

```bash
# 导入 50 张表情包
./scripts/import-data.sh -s staging:fabiaoqing -l 50

# 或使用 go run 直接运行
go run ./cmd/ingest --source=staging:fabiaoqing --limit=50
```

### 4.3 导入参数说明

**脚本参数 (`import-data.sh`):**

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `-s, --source` | 数据源 (`staging:fabiaoqing`, `chinesebqb`) | - |
| `-l, --limit` | 最大导入数量 | 100 |
| `-e, --embedding` | 使用的 embedding 配置名称 | 默认配置 |
| `-f, --force` | 强制重新处理 (跳过去重检查) | false |
| `-r, --retry` | 重试 pending 状态的项目 | false |
| `-h, --help` | 显示帮助信息 | - |

**go run 参数:**

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `--source` | 数据源 (`staging:fabiaoqing`) | chinesebqb |
| `--limit` | 最大导入数量 | 100 |
| `--force` | 强制重新处理 (跳过去重检查) | false |
| `--retry` | 重试失败的项目 | false |

### 4.4 导入流程详解

每张图片的处理流程:

```
1. 读取本地图片文件
        ↓
2. 计算 MD5 (去重检查)
        ↓
3. 调用 VLM 生成中文描述
   例如: "一只熊猫头表情包,表情夸张"
        ↓
4. 调用 Jina 生成 1024 维向量
        ↓
5. 上传图片到 S3/R2
   Key: {md5前2位}/{md5}.{格式}
   例如: ab/abc123def456.gif
        ↓
6. 写入 Qdrant (向量 + 元数据)
        ↓
7. 写入数据库 (SQLite/PostgreSQL)
```

## 5. 完整示例

### 5.1 首次爬取并导入

```bash
# Step 1: 爬取 200 张表情包
cd crawler
uv run emomo-crawler crawl -s fabiaoqing -l 200

# Step 2: 查看爬取结果
uv run emomo-crawler staging stats -s fabiaoqing

# Step 3: 导入到系统（使用脚本，推荐）
cd ..
./scripts/import-data.sh -s staging:fabiaoqing -l 200
```

### 5.2 增量爬取

```bash
# 查看上次爬取到哪一页 (从日志中获取 next_cursor)
# 假设上次到第 5 页

# 从第 5 页继续爬取
cd crawler
uv run emomo-crawler crawl -s fabiaoqing -l 100 -c 5

# 导入新数据 (自动跳过已存在的)
cd ..
./scripts/import-data.sh -s staging:fabiaoqing -l 100
```

### 5.3 重试失败的导入

```bash
# 重试状态为 pending 的项目
./scripts/import-data.sh -r -l 50
```

## 6. Staging 目录结构

```
data/staging/
└── fabiaoqing/
    ├── manifest.jsonl      # 元数据文件 (JSON Lines 格式)
    └── images/
        ├── abc123.gif
        ├── def456.jpg
        └── ...
```

### manifest.jsonl 格式

每行一个 JSON 对象:

```json
{"id":"abc123","filename":"abc123.gif","category":"表情包","tags":["熊猫头","搞笑"],"source_url":"https://...","is_animated":true,"format":"gif","crawled_at":"2024-01-15T10:30:00Z"}
```

| 字段 | 说明 |
|------|------|
| id | 唯一标识 (MD5 前 16 位) |
| filename | 图片文件名 |
| category | 分类 |
| tags | 标签列表 |
| source_url | 原始 URL |
| is_animated | 是否动图 |
| format | 图片格式 |
| crawled_at | 爬取时间 |

## 7. 按来源删除数据

如需删除某个来源的所有数据:

```sql
-- 1. 查询该来源的所有 storage_key
SELECT id, storage_key FROM memes WHERE source_type = 'staging:fabiaoqing';

-- 2. 手动删除 S3 文件 (根据 storage_key)

-- 3. 删除数据库记录
DELETE FROM memes WHERE source_type = 'staging:fabiaoqing';

-- 4. 删除 Qdrant 中的向量 (通过 API 或管理界面)
```

## 8. 常见问题

### Q: 爬取速度太慢?

调整线程数和请求间隔:
```bash
uv run emomo-crawler crawl -s fabiaoqing -l 100 --threads 10
```

### Q: 导入时 VLM 报错?

检查 API 配置:
```bash
# 确保环境变量已设置
echo $OPENAI_API_KEY
echo $OPENAI_BASE_URL
```

### Q: 如何查看导入进度?

导入工具会输出日志:
```
INFO Starting ingestion source=staging:fabiaoqing limit=50
INFO Ingestion completed total=50 processed=48 skipped=2 failed=0
```

### Q: 图片去重机制?

系统通过两种方式去重:
1. **Source ID**: 同一来源的相同 ID 不会重复导入
2. **MD5 Hash**: 相同内容的图片不会重复上传到 S3
