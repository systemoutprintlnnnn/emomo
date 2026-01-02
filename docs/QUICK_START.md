# 快速部署指南

## 前置要求

- OpenAI Compatible API Key（用于 VLM/查询扩展，支持 OpenAI, OpenRouter 等）
- Jina API Key（用于 Embedding）
- Qdrant 服务（本地或云端，使用 gRPC 端口）
- S3 兼容对象存储（Cloudflare R2 / AWS S3 / MinIO 等）
- 前端部署（可选，如 Vercel）

## 方案选择

### 🚀 推荐：Oracle Cloud 免费 VPS

**适合**：想要完全控制，不介意管理服务器

**步骤**：
1. 注册 [Oracle Cloud](https://www.oracle.com/cloud/free/) 免费账户
2. 创建 ARM 实例（2核4GB，永久免费）
3. 按照下面的命令部署

```bash
# 1. 连接到服务器
ssh opc@<your-server-ip>

# 2. 安装 Docker
sudo yum update -y
sudo yum install -y docker docker-compose
sudo systemctl start docker
sudo systemctl enable docker
sudo usermod -aG docker opc
newgrp docker

# 3. 克隆项目
git clone <your-repo-url> emomo
cd emomo

# 4. 创建 .env 文件
cat > .env << EOF
# 对象存储配置（推荐使用 Cloudflare R2）
STORAGE_TYPE=r2
STORAGE_ENDPOINT=<account-id>.r2.cloudflarestorage.com
STORAGE_ACCESS_KEY=your-access-key
STORAGE_SECRET_KEY=your-secret-key
STORAGE_USE_SSL=true
STORAGE_BUCKET=memes
STORAGE_PUBLIC_URL=https://pub-xxx.r2.dev

# 或使用本地 S3 兼容存储（需要先启动 MinIO 等服务）
# STORAGE_TYPE=s3compatible
# STORAGE_ENDPOINT=localhost:9000
# STORAGE_ACCESS_KEY=accesskey
# STORAGE_SECRET_KEY=secretkey
# STORAGE_USE_SSL=false
# STORAGE_BUCKET=memes

# Qdrant Cloud（gRPC）
QDRANT_HOST=your-cluster.qdrant.io
QDRANT_PORT=6334
QDRANT_API_KEY=your-qdrant-api-key
QDRANT_USE_TLS=true

OPENAI_API_KEY=your-openai-key
JINA_API_KEY=your-jina-key
EOF

# 5. 启动服务（API + 日志采集，依赖外部 Qdrant/存储）
cd deployments
docker-compose -f docker-compose.prod.yml up -d

# 6. 检查服务状态
docker ps
curl http://localhost:8080/health

# 7. 配置 Vercel 环境变量
# VITE_API_BASE=http://<your-server-ip>:8080/api/v1
```

### 🎯 最简单：Railway（需要信用卡）

**适合**：想要快速部署，不介意需要信用卡

**步骤**：
1. 访问 [Railway](https://railway.app/)
2. 连接 GitHub 仓库
3. 创建新项目，选择 "Deploy from GitHub repo"
4. 添加环境变量（见下方）
5. Railway 会自动检测并部署

**环境变量**：
```
CONFIG_PATH=./configs/config.prod.yaml
STORAGE_TYPE=r2
STORAGE_ENDPOINT=<account-id>.r2.cloudflarestorage.com
STORAGE_ACCESS_KEY=your-access-key
STORAGE_SECRET_KEY=your-secret-key
STORAGE_USE_SSL=true
STORAGE_BUCKET=memes
STORAGE_PUBLIC_URL=https://pub-xxx.r2.dev
OPENAI_API_KEY=your-openai-key
JINA_API_KEY=your-jina-key
QDRANT_HOST=your-qdrant-host
QDRANT_PORT=6334
QDRANT_API_KEY=your-qdrant-api-key
QDRANT_USE_TLS=true
```

**注意**：需要单独部署 Qdrant 和配置对象存储，或者使用云服务：
- Qdrant Cloud（免费 1GB）
- Cloudflare R2（免费 10GB，S3 兼容）

### 💡 混合方案：云服务 + Railway

**适合**：不想管理基础设施

**步骤**：
1. **Qdrant**：注册 [Qdrant Cloud](https://cloud.qdrant.io/)，创建免费集群
2. **存储**：注册 Cloudflare，创建 R2 bucket
3. **后端**：Railway 部署 API

**环境变量**（Railway）：
```
CONFIG_PATH=./configs/config.prod.yaml
QDRANT_HOST=your-cluster.qdrant.io
QDRANT_PORT=6334
QDRANT_API_KEY=your-qdrant-api-key
QDRANT_USE_TLS=true
STORAGE_TYPE=r2
STORAGE_ENDPOINT=your-account-id.r2.cloudflarestorage.com
STORAGE_ACCESS_KEY=your-r2-access-key
STORAGE_SECRET_KEY=your-r2-secret-key
STORAGE_USE_SSL=true
STORAGE_BUCKET=memes
STORAGE_PUBLIC_URL=https://pub-xxx.r2.dev
OPENAI_API_KEY=your-openai-key
JINA_API_KEY=your-jina-key
```

**注意**：当前代码已支持 Qdrant Cloud API Key（gRPC + TLS）。

## 数据摄入

部署完成后，需要摄入表情包数据：

```bash
# 在服务器上或本地
cd emomo

# 克隆数据源
git clone https://github.com/zhaoolee/ChineseBQB.git ./data/ChineseBQB

# 使用导入脚本（推荐，无需预先编译）
./scripts/import-data.sh -s chinesebqb -l 100

# 如果成功，摄入全部
./scripts/import-data.sh -s chinesebqb -l 10000

# 或使用 go run 直接运行
go run ./cmd/ingest --source=chinesebqb --limit=100
```

## 配置前端

在 Vercel 项目设置中添加环境变量：

```
VITE_API_BASE=https://your-api-domain.com/api/v1
```

如果没有自定义域名：
```
VITE_API_BASE=http://your-server-ip:8080/api/v1
```

## 验证部署

1. **健康检查**：
   ```bash
   curl http://your-api/health
   ```

2. **搜索测试**：
   ```bash
   curl -X POST http://your-api/api/v1/search \
     -H "Content-Type: application/json" \
     -d '{"query": "测试", "top_k": 5}'
   ```

3. **前端测试**：访问 Vercel 部署的前端，尝试搜索

## 常见问题

### Q: 后端无法连接 Qdrant
**A**: 检查 Qdrant 是否运行，端口是否正确，防火墙是否开放

### Q: 图片无法加载
**A**: 检查对象存储 bucket 是否设置为公开读取，CORS 配置是否正确。如果使用 R2，确保配置了 `STORAGE_PUBLIC_URL`

### Q: API 返回 CORS 错误
**A**: 检查后端 CORS 配置，确保允许前端域名

### Q: Railway/Render 部署失败
**A**: 检查环境变量是否全部设置，日志查看具体错误

## 下一步

- [ ] 配置自定义域名和 SSL
- [ ] 设置监控和告警
- [ ] 配置自动备份
- [ ] 优化性能（CDN、缓存）

更多详细信息请参考 [DEPLOYMENT.md](./DEPLOYMENT.md)
