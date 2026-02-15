---
title: Emomo
emoji: 🔥
colorFrom: green
colorTo: indigo
sdk: docker
pinned: true
license: mit
---

---

# Emomo - AI 表情包语义搜索

Emomo 是一个基于 Go + Qdrant + VLM + Text Embedding 的表情包语义搜索系统，当前版本以 **本地文件夹采集（local）** 为主，支持自动描述生成与向量检索。

## 功能概览

- 语义搜索：输入文字描述即可检索相似表情包。
- 数据摄入：支持从任意本地文件夹批量导入。
- 向量管理：支持多 Embedding 模型/多集合管理。
- 存储抽象：兼容 Cloudflare R2、AWS S3 与其他 S3 兼容服务。
- 可扩展：查询扩展、VLM 描述与多模型配置均可开关。

## 技术栈

- **后端**: Go + Gin + GORM
- **向量数据库**: Qdrant (gRPC)
- **元数据存储**: SQLite (本地) / PostgreSQL (生产)
- **对象存储**: S3 兼容存储（Cloudflare R2、AWS S3 等）
- **VLM**: OpenAI-compatible API (图片描述生成)
- **Text Embedding**: Jina Embeddings / OpenAI-compatible Embeddings

## 环境要求

- Go 1.24.6（见 `go.mod`）
- Python >= 3.10（用于 crawler）
- uv（crawler 依赖管理）
- Docker（可选，用于本地 Qdrant/MinIO 或日志采集）

## 快速开始（本地开发）

### 1) 准备环境变量

```bash
cp .env.example .env
# 编辑 .env 填入 API Keys 和服务地址
```

### 2) 准备依赖服务

本项目不会自动启动 Qdrant 或对象存储，你可以选择云服务或本地服务。

> 说明：当前版本的数据导入仅支持 `local`（从本地文件夹扫描图片导入）。

**推荐：云服务（Qdrant Cloud + Cloudflare R2）**

```bash
# Qdrant Cloud (gRPC)
QDRANT_HOST=your-cluster.qdrant.io
QDRANT_PORT=6334
QDRANT_API_KEY=your-qdrant-api-key
QDRANT_USE_TLS=true

# Cloudflare R2
STORAGE_TYPE=r2
STORAGE_ENDPOINT=your-account-id.r2.cloudflarestorage.com
STORAGE_ACCESS_KEY=your-r2-access-key
STORAGE_SECRET_KEY=your-r2-secret-key
STORAGE_BUCKET=memes
STORAGE_REGION=auto
STORAGE_USE_SSL=true
STORAGE_PUBLIC_URL=https://pub-xxx.r2.dev
```

**本地体验：Qdrant + MinIO（S3 兼容）**

```bash
# Qdrant
docker run -d --name qdrant -p 6333:6333 -p 6334:6334 qdrant/qdrant:latest

# 本地 Qdrant 配置
QDRANT_HOST=localhost
QDRANT_PORT=6334
QDRANT_USE_TLS=false

# MinIO
docker run -d --name minio -p 9000:9000 -p 9001:9001 \
  -e MINIO_ROOT_USER=accesskey -e MINIO_ROOT_PASSWORD=secretkey \
  quay.io/minio/minio server /data --console-address ":9001"

# 本地存储配置
STORAGE_TYPE=s3compatible
STORAGE_ENDPOINT=localhost:9000
STORAGE_ACCESS_KEY=accesskey
STORAGE_SECRET_KEY=secretkey
STORAGE_BUCKET=memes
STORAGE_USE_SSL=false
```

**可选：使用 Docker Compose 启动 API + 日志采集（Grafana Alloy）**

```bash
docker-compose -f deployments/docker-compose.yml up -d
```

### 3) 准备数据源

**方式 A：从任意本地文件夹采集（推荐）**

将你的表情包放入任意本地文件夹，系统会自动扫描并按目录结构分类：

```bash
# 配置文件夹路径（编辑 configs/config.yaml 或设置环境变量）
# 示例：将表情包放在 ./data/memes 文件夹
# 目录结构示例：
# ./data/memes/
# ├── 猫猫表情/
# │   ├── cat1.jpg
# │   └── cat2.png
# ├── 熊猫头/
# │   └── panda.gif
# └── 未分类/
#     └── misc.webp
```

在 `configs/config.yaml` 中配置路径：

```yaml
sources:
  local:
    path: ./data/memes  # 你的表情包文件夹路径
```

然后摄入（脚本支持零参数运行）：

```bash
# 直接导入（使用 configs/config.yaml 的 sources.local.path）
./scripts/import-data.sh

# 或手动指定文件夹路径导入
./scripts/import-data.sh -p /path/to/your/memes -l 100

# 或直接 go run
SOURCES_LOCAL_PATH=/path/to/your/memes go run ./cmd/ingest --limit=100 --source=local
```

<!-- 当前版本仅保留 local 本地文件夹导入；如需爬虫/staging 流程，后续可再引入独立脚本与数据源适配器。 -->

### 4) 摄入数据

```bash
# 从本地文件夹导入（推荐，零参数即可运行）
./scripts/import-data.sh

# 手动指定文件夹 + 数量
./scripts/import-data.sh -p /path/to/your/memes -l 100

# 或使用 go run 直接运行（同样支持 env 覆盖路径）
SOURCES_LOCAL_PATH=/path/to/your/memes go run ./cmd/ingest --source=local --limit=100
```

> 注：当前 `import-data.sh` 已简化为仅负责本地文件夹导入；staging/爬虫导入可后续单独脚本化（或恢复多 source 模式）。

### 5) 启动 API 服务

```bash
# 直接运行
go run ./cmd/api

# 或构建二进制
go build -o api ./cmd/api
./api
```

服务默认运行在 `http://localhost:8080`，健康检查 `http://localhost:8080/health`。

## API 示例

### 文本搜索

```bash
curl -X POST http://localhost:8080/api/v1/search \
  -H "Content-Type: application/json" \
  -d '{"query": "无语", "top_k": 20}'
```

### 获取分类列表

```bash
curl http://localhost:8080/api/v1/categories
```

### 获取表情包列表

```bash
curl "http://localhost:8080/api/v1/memes?category=猫猫表情&limit=20"
```

### 获取单个表情包

```bash
curl http://localhost:8080/api/v1/memes/{id}
```

### 获取统计信息

```bash
curl http://localhost:8080/api/v1/stats
```

## 配置说明

- 默认配置文件：`configs/config.yaml`
- 可通过 `CONFIG_PATH` 指定配置文件路径（默认 `configs/config.yaml`）
- `.env` 用于注入 API keys 与运行时环境变量

常用环境变量：

| 配置项 | 环境变量 | 说明 |
|--------|----------|------|
| vlm.api_key | OPENAI_API_KEY | OpenAI-compatible API Key |
| vlm.base_url | OPENAI_BASE_URL | OpenAI-compatible Base URL |
| embedding.api_key | EMBEDDING_API_KEY | Embedding API Key |
| storage.type | STORAGE_TYPE | 存储类型：r2, s3, s3compatible |
| storage.endpoint | STORAGE_ENDPOINT | 存储端点（不含 bucket） |
| storage.bucket | STORAGE_BUCKET | 存储桶名称 |
| storage.region | STORAGE_REGION | 存储区域（R2 使用 `auto`） |
| storage.use_ssl | STORAGE_USE_SSL | 是否使用 HTTPS |
| storage.public_url | STORAGE_PUBLIC_URL | 公开访问 URL（R2 推荐） |
| qdrant.host | QDRANT_HOST | Qdrant 地址 |
| qdrant.port | QDRANT_PORT | Qdrant gRPC 端口（默认 6334） |
| qdrant.api_key | QDRANT_API_KEY | Qdrant Cloud API Key |
| qdrant.use_tls | QDRANT_USE_TLS | Qdrant TLS（Cloud 建议 true） |

## 开发与测试

```bash
# 运行 Go 测试
go test ./...

# 启动 API（热更新自行使用 air/其他工具）
go run ./cmd/api
```

## 项目结构

```
emomo/
├── cmd/                 # Go 入口（api/ingest）
├── crawler/             # Python 爬虫（uv 管理）
├── internal/            # Go 应用核心逻辑
│   ├── api/             # API 层
│   ├── config/          # 配置管理
│   ├── domain/          # 领域模型
│   ├── repository/      # 数据访问层
│   ├── service/         # 业务逻辑层
│   ├── source/          # 数据源适配器
│   └── storage/         # 对象存储
├── configs/             # 配置文件
├── deployments/         # 部署配置
├── data/                # 本地数据目录
├── docs/                # 设计与使用文档
└── scripts/             # 辅助脚本
```

## 更多文档

- `docs/QUICK_START.md`
- `docs/DEPLOYMENT.md`
- `docs/MULTI_EMBEDDING.md`
- `docs/DATABASE_SCHEMA.md`

## License

MIT
