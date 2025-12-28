# Docker Compose 部署指南

本目录包含 Docker Compose 配置文件，支持多种部署模式。

## 文件说明

- `docker-compose.yml` - 开发环境配置（包含 Qdrant 和 S3 兼容存储）
- `docker-compose.prod.yml` - 生产环境配置（支持本地服务和云服务）

## 部署模式

### 模式 1: 仅使用云服务（推荐用于生产环境）

使用 Qdrant Cloud 和 Cloudflare R2，无需本地服务。

**优点**：
- 无需管理本地服务
- 自动扩展
- 高可用性

**步骤**：

1. 配置云服务：
   ```bash
   # 复制示例配置文件
   cp ../configs/config.cloud.yaml.example ../configs/config.prod.yaml
   
   # 编辑配置文件，填入你的云服务凭证
   # - Qdrant Cloud API Key
   # - Cloudflare R2 Access Key 和 Secret Key
   ```

2. 设置环境变量：
   ```bash
   export OPENAI_API_KEY=your-openai-key
   export JINA_API_KEY=your-jina-key
   export QDRANT_API_KEY=your-qdrant-cloud-key
   export QDRANT_HOST=your-cluster.qdrant.io
   export QDRANT_USE_TLS=true
   export STORAGE_TYPE=r2
   export STORAGE_ENDPOINT=your-account-id.r2.cloudflarestorage.com
   export STORAGE_ACCESS_KEY=your-r2-access-key
   export STORAGE_SECRET_KEY=your-r2-secret-key
   export STORAGE_BUCKET=memes
   export STORAGE_REGION=auto
   ```

3. 启动服务：
   ```bash
   docker-compose -f docker-compose.prod.yml up -d
   ```

### 模式 2: 仅使用本地服务

使用本地 Qdrant 和 S3 兼容存储服务。

**步骤**：

1. 启动所有服务（包括本地 Qdrant 和 S3 兼容存储）：
   ```bash
   docker-compose -f docker-compose.prod.yml --profile local up -d
   ```

2. 验证服务：
   ```bash
   # 检查 Qdrant
   curl http://localhost:6333/health

   # 检查 S3 兼容存储
   curl http://localhost:9000/minio/health/live
   ```

### 模式 3: 混合模式 - 本地 Qdrant + 云存储

使用本地 Qdrant，但使用 Cloudflare R2 存储图片。

**步骤**：

1. 启动本地 Qdrant：
   ```bash
   docker-compose -f docker-compose.prod.yml --profile qdrant-local up -d
   ```

2. 配置云存储环境变量（见模式 1）

### 模式 4: 混合模式 - 云 Qdrant + 本地 S3 存储

使用 Qdrant Cloud，但使用本地 S3 兼容存储服务存储图片。

**步骤**：

1. 启动本地 S3 兼容存储：
   ```bash
   docker-compose -f docker-compose.prod.yml --profile s3-local up -d
   ```

2. 配置 Qdrant Cloud 环境变量（见模式 1）

## Profiles 说明

- `local` - 启动所有本地服务（Qdrant + S3 兼容存储）
- `qdrant-local` - 仅启动本地 Qdrant
- `s3-local` - 仅启动本地 S3 兼容存储
- 无 profile - 仅启动 API 服务（使用云服务）

## 环境变量配置

### Qdrant 配置

**本地 Qdrant**：
```bash
export QDRANT_HOST=localhost
export QDRANT_PORT=6334
export QDRANT_USE_TLS=false
```

**Qdrant Cloud**：
```bash
export QDRANT_HOST=your-cluster.qdrant.io
export QDRANT_PORT=443
export QDRANT_API_KEY=your-api-key
export QDRANT_USE_TLS=true
```

### 存储配置

**本地 S3 兼容存储**：
```bash
export STORAGE_TYPE=s3compatible
export STORAGE_ENDPOINT=localhost:9000
export STORAGE_ACCESS_KEY=accesskey
export STORAGE_SECRET_KEY=secretkey
export STORAGE_BUCKET=memes
export STORAGE_USE_SSL=false
```

**Cloudflare R2**：
```bash
export STORAGE_TYPE=r2
export STORAGE_ENDPOINT=your-account-id.r2.cloudflarestorage.com
export STORAGE_ACCESS_KEY=your-r2-access-key
export STORAGE_SECRET_KEY=your-r2-secret-key
export STORAGE_BUCKET=memes
export STORAGE_REGION=auto
export STORAGE_USE_SSL=true
export STORAGE_PUBLIC_URL=https://pub-xxx.r2.dev  # 可选
```

**AWS S3**：
```bash
export STORAGE_TYPE=s3
export STORAGE_ENDPOINT=s3.amazonaws.com
export STORAGE_ACCESS_KEY=your-aws-access-key
export STORAGE_SECRET_KEY=your-aws-secret-key
export STORAGE_BUCKET=your-bucket-name
export STORAGE_REGION=us-east-1
export STORAGE_USE_SSL=true
```

## 常用命令

```bash
# 启动服务
docker-compose -f docker-compose.prod.yml [--profile <profile>] up -d

# 查看日志
docker-compose -f docker-compose.prod.yml logs -f api

# 停止服务
docker-compose -f docker-compose.prod.yml down

# 停止并删除数据卷（谨慎使用）
docker-compose -f docker-compose.prod.yml down -v

# 重启服务
docker-compose -f docker-compose.prod.yml restart api

# 查看运行状态
docker-compose -f docker-compose.prod.yml ps
```

## 故障排查

### API 无法连接 Qdrant

1. 检查 Qdrant 是否运行：
   ```bash
   docker ps | grep qdrant
   ```

2. 检查环境变量：
   ```bash
   docker exec emomo-api env | grep QDRANT
   ```

3. 检查网络连接：
   ```bash
   docker exec emomo-api ping -c 3 localhost  # 本地
   docker exec emomo-api ping -c 3 your-cluster.qdrant.io  # 云服务
   ```

### API 无法连接存储

1. 检查存储服务是否运行（本地 S3 兼容存储）：
   ```bash
   docker ps | grep s3
   ```

2. 检查环境变量：
   ```bash
   docker exec emomo-api env | grep STORAGE
   ```

3. 验证存储配置：
   - 本地 S3 兼容存储：访问 http://localhost:9001 检查 bucket
   - Cloudflare R2：检查 R2 dashboard
   - AWS S3：检查 AWS Console

## 安全建议

1. **不要提交敏感信息**：使用环境变量或 secrets 管理
2. **使用 HTTPS**：生产环境必须使用 TLS
3. **限制访问**：配置防火墙规则
4. **定期备份**：备份数据库和重要数据

