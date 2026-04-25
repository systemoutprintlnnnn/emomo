# Emomo

> AI 表情包语义搜索系统 — Go 后端 + React 前端 monorepo

Emomo 让你用自然语言搜表情包。系统由 Go 后端（搜索 + ChineseBQB 摄入）和 React 前端（用户界面）组成。

## 仓库结构

```
emomo/
├── backend/      # Go + Gin + Qdrant + GORM，REST API + 摄入流水线
├── frontend/     # React 19 + Vite + Framer Motion，单页应用
├── deployments/  # 跨服务的 Docker Compose 编排（API + Grafana Alloy）
├── docs/         # 跨服务设计与使用文档
├── scripts/
│   └── start.sh  # 本机一键起后端 + 前端
├── render.yaml   # Render 部署配置（rootDir: backend）
└── railway.json  # Railway 部署配置（dockerfilePath: backend/Dockerfile）
```

每个子项目都有自己的 `README.md` / `AGENTS.md` / `CLAUDE.md` / `GEMINI.md`，说明该子项目的本地开发与约定：

- 后端：[backend/README.md](backend/README.md)
- 前端：[frontend/README.md](frontend/README.md)

## 快速上手

### 一键起前后端

```bash
./scripts/start.sh
```

脚本会先启 backend（`go run ./cmd/api`，端口 8080），再启 frontend（`npm run dev`，端口 5173）。需要先在 [backend/.env](backend/.env.example) 填好 API keys（Qdrant、对象存储、VLM、embedding 等）。

### 单独运行某一块

```bash
# 后端
cd backend
cp .env.example .env   # 首次：填好 API keys
go run ./cmd/api

# 前端
cd frontend
cp .env.example .env   # 首次：默认指向 http://localhost:8080/api/v1
npm install
npm run dev
```

### Jina v4 图像向量摄入（可选）

使用配置中的 Jina embedding 做摄入时，在 `backend/` 下执行，例如：

```bash
cd backend
./scripts/import-data.sh -s chinesebqb -e jina -l 50
# 或: go run ./cmd/ingest --source=chinesebqb --embedding=jina --limit=50
```

详见 [docs/MULTI_EMBEDDING.md](docs/MULTI_EMBEDDING.md) 与 [backend/configs/config.yaml](backend/configs/config.yaml)。

## 技术栈速览

| 子项目 | 关键技术 |
|--------|---------|
| backend | Go 1.24, Gin, GORM, Qdrant (gRPC), S3/R2, OpenAI-compatible VLM, Jina v4 / ModelScope embeddings, BM25 hybrid 检索, Grafana Alloy + Loki |
| frontend | React 19, TypeScript, Vite 7, Framer Motion, Playwright e2e |

## 部署

- **Docker Compose（本机）**：`docker compose -f deployments/docker-compose.yml up -d`，会起 API 容器 + Grafana Alloy 日志采集（Qdrant 与对象存储需自备）。
- **Render**：根的 [render.yaml](render.yaml) 把后端服务的 rootDir 设为 `backend/`。
- **Railway**：根的 [railway.json](railway.json) 指向 `backend/Dockerfile`。
- **Hugging Face Space**：[`.github/workflows/sync_to_hf.yml`](.github/workflows/sync_to_hf.yml) 在每次 push 到 main 时把 `backend/` 子树拆出来 force-push 到 Space 的 `main` 分支，所以 Space 看到的根就是 `backend/`。

## 贡献约定

- 提交信息使用 Conventional Commits（`feat:`、`fix:`、`chore:` 等）；跨子项目的改动在正文里按目录分点说明。
- AI agents 协作约定见各子项目的 [AGENTS.md](backend/AGENTS.md) / [frontend/AGENTS.md](frontend/AGENTS.md) 与本仓库根的 [AGENTS.md](AGENTS.md)。
- 不提交 secrets，使用各子项目下的 `.env` 与 `.env.example`。

## 更多文档

- [docs/QUICK_START.md](docs/QUICK_START.md)
- [docs/INGEST.md](docs/INGEST.md)
- [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md)
- [docs/MULTI_EMBEDDING.md](docs/MULTI_EMBEDDING.md)
- [docs/DATABASE_SCHEMA.md](docs/DATABASE_SCHEMA.md)

## License

[MIT](LICENSE)
