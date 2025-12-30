---
title: Emomo
emoji: 🔥
colorFrom: green
colorTo: indigo
sdk: docker
pinned: false
---

Check out the configuration reference at https://huggingface.co/docs/hub/spaces-config-reference

# Emomo - AI 表情包语义搜索

基于 Golang + Qdrant + VLM + Text Embedding 的表情包语义搜索系统。

## 技术栈

- **后端**: Go + Gin + GORM
- **向量数据库**: Qdrant
- **元数据存储**: SQLite (MVP) / PostgreSQL (生产)
- **对象存储**: S3 兼容存储（Cloudflare R2、AWS S3 等）
- **VLM**: OpenAI-compatible API (e.g., GPT-4o mini, Claude via OpenRouter) (图片描述生成)
- **Text Embedding**: Jina Embeddings v3 (向量化)

## 快速开始

### 1. 环境准备

```bash
# 复制环境变量配置
cp .env.example .env

# 编辑 .env 填入 API Keys 和服务地址
vim .env
```

### 2. 配置基础服务（Qdrant + 对象存储）

本项目不会自动启动 Qdrant 或对象存储，请选择云服务或本地服务。

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

**本地体验：Docker 启动 Qdrant + MinIO（S3 兼容）**

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

### 3. 可选：启动日志采集（Grafana Alloy）

```bash
docker-compose -f deployments/docker-compose.yml up -d
```

### 4. 准备数据源

```bash
# Clone ChineseBQB 表情包仓库
git clone https://github.com/zhaoolee/ChineseBQB.git ./data/ChineseBQB
```

### 5. 数据摄入

```bash
# 构建摄入工具
go build -o ingest ./cmd/ingest

# 摄入 100 张表情包（测试）
./ingest --source=chinesebqb --limit=100

# 摄入全部表情包
./ingest --source=chinesebqb --limit=10000
```

### 6. 启动 API 服务

```bash
# 构建 API 服务
go build -o api ./cmd/api

# 启动服务
./api
```

服务默认运行在 `http://localhost:8080`，健康检查 `http://localhost:8080/health`。

## API 接口

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

配置文件: `configs/config.yaml`（可用 `CONFIG_PATH` 指定）

主要配置项:

| 配置项 | 环境变量 | 说明 |
|--------|----------|------|
| vlm.api_key | OPENAI_API_KEY | OpenAI-compatible API Key |
| vlm.base_url | OPENAI_BASE_URL | OpenAI-compatible Base URL |
| embedding.api_key | JINA_API_KEY | Jina API Key |
| storage.type | STORAGE_TYPE | 存储类型：r2, s3, s3compatible |
| storage.endpoint | STORAGE_ENDPOINT | 存储端点地址（不包含 bucket） |
| storage.bucket | STORAGE_BUCKET | 存储桶名称 |
| storage.region | STORAGE_REGION | 存储区域（R2 使用 `auto`） |
| storage.use_ssl | STORAGE_USE_SSL | 是否使用 HTTPS |
| storage.public_url | STORAGE_PUBLIC_URL | 公开访问 URL（R2 推荐配置） |
| qdrant.host | QDRANT_HOST | Qdrant 地址 |
| qdrant.port | QDRANT_PORT | Qdrant gRPC 端口（默认 6334） |
| qdrant.api_key | QDRANT_API_KEY | Qdrant Cloud API Key |
| qdrant.use_tls | QDRANT_USE_TLS | Qdrant TLS（Cloud 建议 true） |

## 项目结构

```
emomo/
├── cmd/
│   ├── api/          # API 服务入口
│   └── ingest/       # 摄入 CLI 工具
├── crawler/          # Python 爬虫
├── internal/
│   ├── api/          # API 层
│   ├── config/       # 配置管理
│   ├── domain/       # 领域模型
│   ├── repository/   # 数据访问层
│   ├── service/      # 业务逻辑层
│   ├── source/       # 数据源适配器
│   └── storage/      # 对象存储
├── configs/          # 配置文件
├── deployments/      # 部署配置
├── data/             # 本地数据目录
└── scripts/          # 脚本
```

## License

MIT
