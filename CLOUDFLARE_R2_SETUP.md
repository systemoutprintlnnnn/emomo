# Cloudflare R2 配置指南

## 获取访问密钥（Access Key）和秘密密钥（Secret Key）

### 步骤 1：登录 Cloudflare 仪表板

1. 访问 [Cloudflare 仪表板](https://dash.cloudflare.com/)
2. 使用您的账户登录

### 步骤 2：进入 R2 对象存储

1. 在左侧菜单中，点击 **"R2"** 进入对象存储管理页面
2. 如果还没有创建存储桶，先创建一个存储桶（Bucket）

### 步骤 3：创建 API 令牌

1. 在 R2 页面中，找到并点击 **"管理 API 令牌"**（Manage API Tokens）
2. 点击 **"创建 API 令牌"**（Create API Token）
3. 填写令牌信息：
   - **令牌名称**：例如 `emomo-storage`（便于识别）
   - **权限**：选择 **"对象读取和写入"**（Object Read and Write）
   - **资源**（可选）：可以限制到特定存储桶，或选择所有存储桶
4. 点击 **"创建 API 令牌"**

### 步骤 4：保存访问密钥

创建成功后，会显示以下信息：

- **访问密钥 ID（Access Key ID）**：这就是 `STORAGE_ACCESS_KEY`
- **秘密访问密钥（Secret Access Key）**：这就是 `STORAGE_SECRET_KEY`

⚠️ **重要提示**：
- **秘密访问密钥只会显示一次**
- 请立即复制并安全保存
- 如果丢失，需要重新创建 API 令牌

### 步骤 5：获取账户 ID 和端点信息

1. 在 R2 页面右上角，可以看到您的 **账户 ID**（Account ID）
2. R2 的端点格式为：`https://<ACCOUNT_ID>.r2.cloudflarestorage.com`
   - 例如：`https://abc123def456.r2.cloudflarestorage.com`

### 步骤 6：获取公共 URL（可选）

如果您想使用 Cloudflare R2 的公共访问域名：

1. 在存储桶设置中，找到 **"公共访问"**（Public Access）
2. 启用公共访问后，会生成一个公共 URL
3. 格式通常为：`https://pub-<random-id>.r2.dev`
4. 或者使用自定义域名（需要配置）

## 环境变量配置

在 Hugging Face Spaces 的 Settings → Secrets and variables → Variables 中添加：

```bash
# 存储类型
STORAGE_TYPE=r2

# R2 端点（使用您的账户 ID）
STORAGE_ENDPOINT=https://<YOUR_ACCOUNT_ID>.r2.cloudflarestorage.com

# 访问密钥（从步骤 4 获取）
STORAGE_ACCESS_KEY=<YOUR_ACCESS_KEY_ID>

# 秘密密钥（从步骤 4 获取）
STORAGE_SECRET_KEY=<YOUR_SECRET_ACCESS_KEY>

# 使用 HTTPS
STORAGE_USE_SSL=true

# 存储桶名称
STORAGE_BUCKET=<YOUR_BUCKET_NAME>

# 公共 URL（可选，用于生成图片访问链接）
STORAGE_PUBLIC_URL=https://pub-<random-id>.r2.dev
```

## 配置示例

假设您的账户 ID 是 `abc123def456`，存储桶名称是 `emomo-memes`：

```bash
STORAGE_TYPE=r2
STORAGE_ENDPOINT=https://abc123def456.r2.cloudflarestorage.com
STORAGE_ACCESS_KEY=your_access_key_id_here
STORAGE_SECRET_KEY=your_secret_access_key_here
STORAGE_USE_SSL=true
STORAGE_BUCKET=emomo-memes
STORAGE_PUBLIC_URL=https://pub-xyz123.r2.dev
```

## 验证配置

配置完成后，应用启动时会尝试连接 R2。检查日志确认连接成功：

```
INFO    Storage connection established
        endpoint=https://abc123def456.r2.cloudflarestorage.com
        bucket=emomo-memes
```

**注意**：日志可能显示 "MinIO connection"，这是因为代码使用 minio-go 库作为 S3 兼容客户端，这是正常的。

## 注意事项

1. **免费额度**：Cloudflare R2 提供每月 10GB 存储和 1000 万次读取操作免费
2. **出站流量**：R2 没有出站流量费用（这是相比其他云存储的优势）
3. **区域**：R2 是全局服务，不需要指定区域（Region）
4. **安全性**：请妥善保管访问密钥，不要提交到代码仓库

## 故障排查

### 连接失败

- 检查端点格式是否正确（必须包含 `https://` 和账户 ID）
- 确认访问密钥和秘密密钥是否正确复制（没有多余空格）
- 检查存储桶名称是否正确

### 权限错误

- 确认 API 令牌权限包含"对象读取和写入"
- 如果限制了资源，确认存储桶在允许列表中

### 公共访问问题

- 如果使用 `STORAGE_PUBLIC_URL`，确保存储桶已启用公共访问
- 或者使用 Cloudflare Workers 或自定义域名提供公共访问

