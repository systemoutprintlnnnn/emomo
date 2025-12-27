# Emomo - AI 表情包语义搜索

基于 Golang + Qdrant + VLM + Text Embedding 的表情包语义搜索系统。

## 技术栈

- **后端**: Go + Gin + GORM
- **向量数据库**: Qdrant
- **元数据存储**: SQLite (MVP) / PostgreSQL (生产)
- **对象存储**: MinIO
- **VLM**: GPT-4o mini (图片描述生成)
- **Text Embedding**: Jina Embeddings v3 (向量化)

## 快速开始

### 1. 环境准备

```bash
# 复制环境变量配置
cp .env.example .env

# 编辑 .env 填入 API Keys
vim .env
```

### 2. 启动基础设施

```bash
# 启动 Qdrant 和 MinIO
docker-compose -f deployments/docker-compose.yml up -d
```

### 3. 准备数据源

```bash
# Clone ChineseBQB 表情包仓库
git clone https://github.com/zhaoolee/ChineseBQB.git ./data/ChineseBQB
```

### 4. 数据摄入

```bash
# 构建摄入工具
go build -o ingest ./cmd/ingest

# 摄入 100 张表情包（测试）
./ingest --source=chinesebqb --limit=100

# 摄入全部表情包
./ingest --source=chinesebqb --limit=10000
```

### 5. 启动 API 服务

```bash
# 构建 API 服务
go build -o api ./cmd/api

# 启动服务
./api
```

服务默认运行在 `http://localhost:8080`

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

配置文件: `configs/config.yaml`

主要配置项:

| 配置项 | 环境变量 | 说明 |
|--------|----------|------|
| vlm.api_key | OPENAI_API_KEY | OpenAI API Key |
| embedding.api_key | JINA_API_KEY | Jina API Key |
| minio.access_key | MINIO_ACCESS_KEY | MinIO Access Key |
| minio.secret_key | MINIO_SECRET_KEY | MinIO Secret Key |

## 项目结构

```
emomo/
├── cmd/
│   ├── api/          # API 服务入口
│   └── ingest/       # 摄入 CLI 工具
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
