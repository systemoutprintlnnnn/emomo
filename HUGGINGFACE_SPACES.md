# Hugging Face Spaces 部署指南

## 问题说明

Hugging Face Spaces 只运行单个 Docker 容器，**不包含 Qdrant 和对象存储服务**。因此需要配置外部服务。

## 解决方案

### 1. 使用 Qdrant Cloud（推荐）

1. 注册 [Qdrant Cloud](https://cloud.qdrant.io/) 账户（免费套餐可用）
2. 创建集群并获取连接信息：
   - Host: `xxx.qdrant.io`
   - Port: `6333` (HTTPS) 或 `6334` (HTTP)
   - API Key: `your-api-key`

### 2. 使用兼容 S3 的对象存储

选项 A：使用 [Backblaze B2](https://www.backblaze.com/b2/cloud-storage.html)（免费 10GB）
- 兼容 S3 API
- 免费额度充足

选项 B：使用 [Cloudflare R2](https://www.cloudflare.com/products/r2/)（免费 10GB/月）⭐ **推荐**
- 兼容 S3 API
- 无出站流量费用
- 详细配置指南：参见 [CLOUDFLARE_R2_SETUP.md](./CLOUDFLARE_R2_SETUP.md)

选项 C：使用其他云存储（AWS S3、阿里云 OSS 等）

## 环境变量配置

在 Hugging Face Spaces 的 Settings → Secrets and variables → Variables 中添加：

### Qdrant 配置

```bash
QDRANT_HOST=your-cluster.qdrant.io
QDRANT_PORT=6333
```

如果使用 Qdrant Cloud，还需要在代码中添加 API Key 支持（当前版本暂不支持，需要修改代码）。

### 存储配置（使用新的统一配置）

#### Cloudflare R2（推荐）

```bash
STORAGE_TYPE=r2
STORAGE_ENDPOINT=https://<YOUR_ACCOUNT_ID>.r2.cloudflarestorage.com
STORAGE_ACCESS_KEY=<YOUR_ACCESS_KEY_ID>
STORAGE_SECRET_KEY=<YOUR_SECRET_ACCESS_KEY>
STORAGE_USE_SSL=true
STORAGE_BUCKET=<YOUR_BUCKET_NAME>
STORAGE_PUBLIC_URL=https://pub-<random-id>.r2.dev  # 可选
```

**如何获取访问密钥**：参见 [CLOUDFLARE_R2_SETUP.md](./CLOUDFLARE_R2_SETUP.md)

#### Backblaze B2（使用统一配置格式）

```bash
STORAGE_TYPE=s3
STORAGE_ENDPOINT=s3.us-west-000.backblazeb2.com
STORAGE_ACCESS_KEY=your-access-key
STORAGE_SECRET_KEY=your-secret-key
STORAGE_USE_SSL=true
STORAGE_BUCKET=your-bucket-name
```

**注意**：旧版 `MINIO_*` 环境变量仍支持（向后兼容），但推荐使用新的 `STORAGE_*` 格式。

### API Keys

```bash
OPENAI_API_KEY=your-openai-key
OPENAI_BASE_URL=https://openrouter.ai/api/v1
VLM_MODEL=qwen/qwen-2.5-vl-7b-instruct:free
JINA_API_KEY=your-jina-key
```

## 临时解决方案：禁用 Qdrant 和对象存储

如果暂时无法配置外部服务，可以修改代码使应用在服务不可用时仍能启动（但搜索功能将不可用）。

### 修改 `cmd/api/main.go`

将 Qdrant 和对象存储初始化改为可选：

```go
// Initialize Qdrant (optional)
qdrantRepo, err := repository.NewQdrantRepository(...)
if err != nil {
    logger.Warn("Qdrant unavailable, search features disabled", zap.Error(err))
    qdrantRepo = nil
}

// Initialize storage (optional)
objectStorage, err := storage.NewStorage(&storage.S3Config{...})
if err != nil {
    logger.Warn("Storage unavailable, upload features disabled", zap.Error(err))
    objectStorage = nil
}
```

然后修改 `SearchService` 使其在 `qdrantRepo` 为 nil 时返回空结果而不是错误。

## 推荐架构

对于生产环境，建议：

1. **后端 API**: Hugging Face Spaces（免费）
2. **向量数据库**: Qdrant Cloud（免费套餐）
3. **对象存储**: Backblaze B2 或 Cloudflare R2（免费）
4. **前端**: Vercel（免费）

这样所有组件都可以使用免费资源。

## 注意事项

1. **Qdrant API Key**: 当前代码版本不支持 Qdrant Cloud 的 API Key 认证，需要修改 `internal/repository/qdrant_repo.go` 添加认证支持
2. **HTTPS**: Qdrant Cloud 使用 HTTPS，需要修改连接方式
3. **数据持久化**: Hugging Face Spaces 的存储是临时的，重启后会丢失 SQLite 数据，建议使用外部数据库（如 Supabase PostgreSQL）

