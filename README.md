# AI Workflow（Wave4 / V2）

本项目为 Go 后端 + React 前端协作系统。
当前统一模型：**issue / profile / run / Team Leader**。

## 核心术语

- `issue`：最小交付单元，包含目标、上下文、约束与验收标准。
- `profile`：Agent 执行画像（角色与能力组合）。
- `run`：一次执行实例，输入为 `issue + profile`，输出为事件与结果。
- `Team Leader`：统一编排入口，负责 issue 拆分、profile 选择、run 启停与 review 汇总。

## 环境要求

- Go 1.23+
- Node.js 20+
- Git

## 本地启动

### 0) 初始化配置（推荐）

首次进入仓库目录后，先生成项目内配置文件：

```powershell
go run ./cmd/ai-flow config init
```

这会创建 `./.ai-workflow/config.yaml`，后续按需修改该文件即可。

如需覆盖已存在配置：

```powershell
go run ./cmd/ai-flow config init --force
```

> 如果不执行这一步，程序也能启动，会自动使用内置默认配置。

### 1) 启动后端

```powershell
go run ./cmd/ai-flow server --port 8080
```

### 2) 启动前端

首次安装依赖：

```powershell
npm --prefix web install
```

启动开发服务器：

```powershell
npm --prefix web run dev -- --strictPort
```

### 3) 访问地址

- 前端：`http://localhost:5173`
- 后端健康检查：`http://127.0.0.1:8080/health`

## V2 API 主链路

1. 创建项目：`POST /api/v1/projects`
2. 启动 Team Leader 对话（携带 profile）：`POST /api/v1/projects/{projectID}/chat`
3. 生成 issue：`POST /api/v1/projects/{projectID}/issues/from-files`
4. 查询 run 事件：`GET /api/v1/projects/{projectID}/chat/{sessionID}/events`
5. 查询 issue review 事件：`GET /api/v1/projects/{projectID}/issues/{issueID}/timeline?kinds=review`

## 测试与 Smoke

Wave4-T3 基线脚本：

```powershell
pwsh -NoProfile -File .\scripts\test\v2-smoke.ps1
```

聚合入口（当前收敛到 V2 smoke）：

```powershell
pwsh -NoProfile -File .\scripts\test\p3-integration.ps1
```

## 文档入口

- 当前计划入口：`docs/plans/README.md`
- 历史计划归档：`docs/archive/plans/README.md`
- 规格与学习资料：`docs/spec/`、`docs/thinking/`、`docs/learning/`
