# AI Workflow

本项目为 Go 后端 + React 前端协作系统。

当前主线已经收敛到单一路径：

- 后端 API 基线：`/api/*`
- 后端主实现：`internal/backend`、`internal/engine`、`internal/core`、`internal/support`
- 历史设计与旧实现：统一归档在 `archive-src/` 与 `docs/archive/`

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

这会创建 `./.ai-workflow/config.toml`，后续按需修改该文件即可。

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

## 桌面版（Tauri）

已提供 **Tauri + Go sidecar + React** 的桌面打包骨架：

- 前端：仍使用 `web/`（Vite 构建 `web/dist`）
- 后端：`cmd/ai-flow` 作为 **sidecar** 随桌面应用发布，并在启动时自动拉起
- Token：桌面版会通过 Tauri bridge 自动读取应用数据目录下的 `secrets.toml` 并注入到前端（首次启动不需要手动 `?token=...`）

开发运行（Windows）：

```powershell
npm install
npm run tauri:icons
npm run tauri:dev
```

构建打包（Windows）：

```powershell
npm install
npm run tauri:build
```

更多说明见：`docs/spec/tauri-desktop.md`。

## 当前接口状态

当前后端统一使用 `/api/*`，以 Flow / Step / Execution 为核心模型。

## 测试与 Smoke

基线脚本：

```powershell
pwsh -NoProfile -File .\scripts\test\smoke.ps1
```

聚合入口：

```powershell
pwsh -NoProfile -File .\scripts\test\p3-integration.ps1
```

## 文档入口

- 当前计划入口：`docs/plans/README.md`
- 历史计划归档：`docs/archive/plans/README.md`
- 历史 `v3` 设计归档：`docs/archive/v3/README.md`
- 规格与学习资料：`docs/spec/`、`docs/thinking/`、`docs/learning/`
