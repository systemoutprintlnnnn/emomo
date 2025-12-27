# Emomo 免费资源部署指南

本文档介绍如何使用免费资源完整部署 Emomo 项目。

## 架构概览

项目包含以下组件：
- **前端**：React + Vite（已部署在 Vercel）
- **后端 API**：Go + Gin（需要部署）
- **向量数据库**：Qdrant（需要部署）
- **对象存储**：MinIO（需要部署或使用云存储）
- **元数据数据库**：SQLite（文件存储）
- **外部 API**：OpenAI（VLM）、Jina（Embedding）

## 方案一：Oracle Cloud 免费 VPS（推荐）

### 优势
- ✅ 永久免费（2核 ARM CPU，4GB RAM，200GB 存储）
- ✅ 性能稳定，不会休眠
- ✅ 可以运行所有服务（Qdrant、MinIO、后端 API）

### 步骤

#### 1. 创建 Oracle Cloud 账户
1. 访问 [Oracle Cloud](https://www.oracle.com/cloud/free/)
2. 注册账户（需要信用卡验证，但不会扣费）
3. 创建免费 ARM 实例（Ampere A1）

#### 2. 配置服务器
```bash
# SSH 连接到服务器
ssh opc@<your-server-ip>

# 更新系统
sudo yum update -y

# 安装 Docker 和 Docker Compose
sudo yum install -y docker docker-compose
sudo systemctl start docker
sudo systemctl enable docker
sudo usermod -aG docker opc

# 安装 Go（如果需要从源码构建）
sudo yum install -y golang git

# 重新登录以应用组权限
exit
```

#### 3. 部署基础设施（Qdrant + MinIO）
```bash
# 克隆项目
git clone <your-repo-url> emomo
cd emomo

# 启动 Qdrant 和 MinIO
cd deployments
docker-compose up -d

# 验证服务
docker ps
curl http://localhost:6333/health  # Qdrant
curl http://localhost:9000/minio/health/live  # MinIO
```

#### 4. 配置防火墙
在 Oracle Cloud 控制台配置安全规则，开放端口：
- `8080`：后端 API
- `6333`：Qdrant REST API（可选，如果不需要外部访问）
- `9000`：MinIO API（可选）
- `9001`：MinIO Console（可选）

#### 5. 配置环境变量
```bash
# 创建 .env 文件
cd /home/opc/emomo
cat > .env << EOF
# MinIO 配置
MINIO_ACCESS_KEY=your-access-key
MINIO_SECRET_KEY=your-secret-key

# OpenAI API
OPENAI_API_KEY=your-openai-key

# Jina API
JINA_API_KEY=your-jina-key

# 可选配置
VLM_MODEL=gpt-4o-mini
QUERY_EXPANSION_MODEL=gpt-4o-mini
SEARCH_SCORE_THRESHOLD=0.0
EOF
```

#### 6. 初始化 MinIO
```bash
# 访问 MinIO Console（需要配置安全组开放 9001 端口）
# http://<your-server-ip>:9001
# 使用 .env 中的 MINIO_ACCESS_KEY 和 MINIO_SECRET_KEY 登录
# 创建名为 "memes" 的 bucket，并设置为公开读取
```

#### 7. 构建并运行后端 API
```bash
cd /home/opc/emomo

# 构建
go build -o api ./cmd/api

# 创建 systemd 服务（推荐）
sudo tee /etc/systemd/system/emomo-api.service > /dev/null << EOF
[Unit]
Description=Emomo API Service
After=network.target

[Service]
Type=simple
User=opc
WorkingDirectory=/home/opc/emomo
EnvironmentFile=/home/opc/emomo/.env
ExecStart=/home/opc/emomo/api
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# 启动服务
sudo systemctl daemon-reload
sudo systemctl enable emomo-api
sudo systemctl start emomo-api
sudo systemctl status emomo-api
```

#### 8. 配置 Nginx 反向代理（可选，推荐）
```bash
# 安装 Nginx
sudo yum install -y nginx

# 配置 Nginx
sudo tee /etc/nginx/conf.d/emomo.conf > /dev/null << EOF
server {
    listen 80;
    server_name <your-domain-or-ip>;

    location /api/ {
        proxy_pass http://localhost:8080;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }
}
EOF

# 启动 Nginx
sudo systemctl enable nginx
sudo systemctl start nginx
```

#### 9. 配置 Vercel 前端环境变量
在 Vercel 项目设置中添加：
```
VITE_API_BASE=https://<your-domain-or-ip>/api/v1
```

如果没有域名，使用 IP：
```
VITE_API_BASE=http://<your-server-ip>:8080/api/v1
```

## 方案二：Railway（简单但需要信用卡）

### 优势
- ✅ 部署简单，支持 Docker
- ✅ 自动 HTTPS
- ✅ 可以部署多个服务

### 限制
- ⚠️ 需要信用卡（免费额度 $5/月）
- ⚠️ 超出免费额度会收费

### 步骤

#### 1. 部署 Qdrant
1. 访问 [Railway](https://railway.app/)
2. 创建新项目
3. 添加服务 → Deploy from GitHub
4. 使用 Qdrant 官方 Docker 镜像：`qdrant/qdrant:latest`
5. 配置端口：`6333`（REST API）

#### 2. 部署 MinIO
1. 在同一个项目中添加新服务
2. 使用 MinIO Docker 镜像：`minio/minio:latest`
3. 配置环境变量：
   - `MINIO_ROOT_USER=minioadmin`
   - `MINIO_ROOT_PASSWORD=your-password`
   - `MINIO_SERVER_URL=https://your-minio.railway.app`
4. 命令：`server /data --console-address ":9001"`

#### 3. 部署后端 API
1. 添加新服务，连接到 GitHub 仓库
2. 配置环境变量（参考方案一的步骤 5）
3. 修改 `configs/config.yaml` 中的 Qdrant 和 MinIO 地址为 Railway 提供的内部地址
4. Railway 会自动检测 Go 项目并构建

#### 4. 配置前端
在 Vercel 中设置：
```
VITE_API_BASE=https://your-api.railway.app/api/v1
```

## 方案三：混合方案（最经济）

### 架构
- **Qdrant**：使用 [Qdrant Cloud 免费 tier](https://cloud.qdrant.io/)（1GB 免费）
- **对象存储**：使用 [Cloudflare R2](https://www.cloudflare.com/products/r2/)（10GB 免费）
- **后端 API**：Railway 或 Render 免费 tier
- **前端**：Vercel（已部署）

### 步骤

#### 1. 设置 Qdrant Cloud
1. 注册 [Qdrant Cloud](https://cloud.qdrant.io/)
2. 创建免费集群
3. 获取 API 密钥和集群 URL

#### 2. 设置 Cloudflare R2
1. 注册 Cloudflare 账户
2. 创建 R2 bucket（名为 `memes`）
3. 获取 Access Key ID 和 Secret Access Key
4. 配置 CORS 和公开访问策略

#### 3. 修改代码以支持 R2
需要修改 `internal/storage/minio.go` 以支持 S3 兼容的 API（R2 兼容 S3）。

创建新文件 `internal/storage/r2.go`：
```go
package storage

import (
    "context"
    "fmt"
    "io"
    
    "github.com/minio/minio-go/v7"
    "github.com/minio/minio-go/v7/pkg/credentials"
)

// R2Storage implements ObjectStorage using Cloudflare R2 (S3-compatible)
type R2Storage struct {
    client   *minio.Client
    bucket   string
    endpoint string
}

// R2Config holds configuration for R2 client
type R2Config struct {
    Endpoint  string
    AccessKey string
    SecretKey string
    Bucket    string
}

// NewR2Storage creates a new R2 storage client
func NewR2Storage(cfg *R2Config) (*R2Storage, error) {
    client, err := minio.New(cfg.Endpoint, &minio.Options{
        Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
        Secure: true, // R2 uses HTTPS
        Region: "auto", // R2 uses "auto" region
    })
    if err != nil {
        return nil, fmt.Errorf("failed to create R2 client: %w", err)
    }

    storage := &R2Storage{
        client:   client,
        bucket:   cfg.Bucket,
        endpoint: cfg.Endpoint,
    }

    return storage, nil
}

// 实现 ObjectStorage 接口的所有方法（与 MinIOStorage 类似）
// ... (参考 minio.go 的实现)
```

#### 4. 部署后端 API
使用 Railway 或 Render：
- Railway：连接 GitHub，自动构建
- Render：创建 Web Service，连接 GitHub

环境变量配置：
```bash
# Qdrant Cloud
QDRANT_HOST=your-cluster.qdrant.io
QDRANT_PORT=443
QDRANT_COLLECTION=memes
QDRANT_API_KEY=your-api-key

# Cloudflare R2
MINIO_ENDPOINT=your-account-id.r2.cloudflarestorage.com
MINIO_ACCESS_KEY=your-r2-access-key
MINIO_SECRET_KEY=your-r2-secret-key
MINIO_USE_SSL=true
MINIO_BUCKET=memes

# OpenAI & Jina
OPENAI_API_KEY=your-key
JINA_API_KEY=your-key
```

## 方案四：Render（免费但会休眠）

### 优势
- ✅ 完全免费（免费 tier）
- ✅ 支持 Docker

### 限制
- ⚠️ 免费服务会在 15 分钟无活动后休眠
- ⚠️ 唤醒需要 30-60 秒

### 步骤
1. 访问 [Render](https://render.com/)
2. 创建 Web Service，连接 GitHub
3. 配置环境变量
4. 使用 Dockerfile 或直接构建 Go 应用

## 数据摄入

部署完成后，需要摄入数据：

```bash
# 在服务器上或本地
cd emomo

# 克隆数据源
git clone https://github.com/zhaoolee/ChineseBQB.git ./data/ChineseBQB

# 构建摄入工具
go build -o ingest ./cmd/ingest

# 摄入数据（先测试少量）
./ingest --source=chinesebqb --limit=100

# 如果成功，摄入全部
./ingest --source=chinesebqb --limit=10000
```

## 成本对比

| 方案 | 月成本 | 优点 | 缺点 |
|------|--------|------|------|
| Oracle Cloud VPS | $0 | 永久免费，性能好 | 需要信用卡验证 |
| Railway | $0-5 | 部署简单 | 需要信用卡，有额度限制 |
| Qdrant Cloud + R2 + Railway | $0-5 | 服务分离，易扩展 | 需要信用卡 |
| Render | $0 | 完全免费 | 会休眠，响应慢 |

## 推荐配置

对于个人项目，推荐**方案一（Oracle Cloud VPS）**：
- 成本最低（$0）
- 性能稳定
- 可以运行所有服务
- 不会休眠

如果不想管理服务器，推荐**方案三（混合方案）**：
- Qdrant Cloud（免费 1GB）
- Cloudflare R2（免费 10GB）
- Railway/Render（免费 tier）

## 故障排查

### 后端无法连接 Qdrant
- 检查 Qdrant 是否运行：`docker ps`
- 检查端口是否正确：`curl http://localhost:6333/health`
- 检查防火墙规则

### 图片无法加载
- 检查 MinIO/R2 bucket 是否设置为公开读取
- 检查 CORS 配置
- 检查图片 URL 是否正确

### API 请求失败
- 检查 Vercel 环境变量 `VITE_API_BASE` 是否正确
- 检查后端日志：`sudo journalctl -u emomo-api -f`
- 检查 CORS 配置

## 安全建议

1. **使用 HTTPS**：配置域名和 SSL 证书（Let's Encrypt 免费）
2. **保护 API Keys**：不要提交到 Git，使用环境变量
3. **限制访问**：配置防火墙，只开放必要端口
4. **定期备份**：备份 SQLite 数据库和 Qdrant 数据

## 下一步

- [ ] 配置自定义域名
- [ ] 设置 SSL 证书（Let's Encrypt）
- [ ] 配置监控和日志
- [ ] 设置自动备份
- [ ] 优化性能（CDN、缓存等）

