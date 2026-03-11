# v1 清理台账（首轮盘点）

日期：2026-03-11

## 目标口径

本台账按当前你的决策记录：

- 后续主线按“当前仓库里的 `internal/v2/*` + `/api/v2` + 新页面壳”推进。
- `v1` 指当前仍挂在 `/api/v1`、旧 issue/run/chat 模型、以及围绕它形成的兼容层与遗留资产。
- `docs/other/v3/*` 目前视为历史/另案设想，不作为这次清理目标实现依据。

## 结论摘要

当前项目不是“仓库里残留一些旧文件”，而是存在四层交织：

1. 运行时双栈：主进程同时启动旧 runtime 和 `v2` runtime，并同时挂 `/api/v1` 与 `/api/v2`。
2. 配置兼容：`v2` agent/profile 仍可从 `v1` 的 `agents.profiles + roles` 推导。
3. 前端双模型：新 `pages/` 路由壳已上线，但大量真实 API 集成仍停留在旧 `apiClient.ts` / `views/*` / `v2/*` 旧视图体系。
4. 文档口径冲突：README、CLAUDE、`docs/other/v3` 对 “v1/v2/v3” 的命名语义互相冲突。

## 最新核查结果（2026-03-11）

- 前端现行主路由 `web/src/main.tsx -> web/src/App.tsx -> web/src/pages/*` 已不再静态引用 `@/views/*`、`@/v2/*`、`@/v3/*`。
- `web/src/lib/apiClient.ts` 目前在主源码树中未发现现行业务入口 import，主要保留为 legacy API client 与相关单测资产。
- `/api/v1` 的活跃依赖已明显收敛到后端兼容链：
  - `internal/web/server.go` 仍挂整组 `/api/v1`
  - `internal/web/handlers_v1_routes.go` 已不再默认挂载 legacy business routes，只保留 `ws + admin ops`
  - 当前默认链路只保留 `ws + admin ops`；`/api/v1/mcp` 已从默认 server/runtime/assistant/executor 接线中摘除
  - `internal/teamleader/mcp_tools.go`、`internal/configruntime/manager.go` 仍保留 legacy MCP URL 生成逻辑，但不再接入现行默认运行链
  - `internal/mcpserver/tools_dev.go`、`internal/mcpserver/preflight.go` 仍调用 `/api/v1/admin/ops/*`
  - `src-tauri/src/main.rs` 仍把桌面端 `ws_base_url` 指向 `/api/v1`
  - `cmd/ai-flow/server.go` 启动提示仍把 WebSocket 口径写成 `/api/v1/ws`
- 当前“先迁再删”的关键事实已经比较明确：前端入口已经基本脱钩，下一阶段主要阻力在后端兼容出口、桌面壳、MCP/A2A 和相应测试面。
- A2A 已从前后端主运行链摘除：服务端不再注册 A2A endpoint / agent card，桌面 bootstrap 不再暴露 `a2a_base_url`，前端 `a2aClient/types` 已迁入归档目录。

### 当前 `/api/v1` 白名单候选

建议先按下面三类理解，而不是继续把所有 v1 资产混在一起：

#### 1. 暂时必须保留的兼容协议出口

- WebSocket：`/api/v1/ws`
  - 证据：`internal/web/handlers_v1_routes.go` 仍注册 `r.Get("/ws", hub.HandleWS)`；`cmd/ai-flow/server.go` 启动提示已改为 `legacy ws`
- Dev admin ops：`/api/v1/admin/ops/*`
  - 证据：`internal/mcpserver/tools_dev.go`、`internal/mcpserver/preflight.go` 仍直接调用 restart / system-event 相关入口

判断：当前 compat 白名单已缩到 `ws + admin ops`。MCP 不再属于默认保留白名单，只剩 legacy 源码与独立实现残留待后续继续归档。

当前代码结构上已完成第一步收口：

- `internal/web/handlers_v1_routes.go` 中，compat 白名单已单独收敛到 `registerV1CompatRoutes(...)`
- legacy 业务路由已单独落在 `registerV1LegacyBusinessRoutes(...)`，且已从默认 server runtime 摘除
- 当前 compat 白名单范围：`ws + admin ops`

#### 2. 已基本退出现行入口、可继续归档/替换的 legacy client / 协议

- `web/src/lib/apiClient.ts`
- A2A（前后端）
- MCP（默认运行链）

覆盖范围：

- `projects / issues / runs / chat / workflow-profiles / repo / admin config / audit / gates / decisions`
- `a2a client / a2a types / a2a bridge / a2a http route / agent card`
- `mcp http route / mcp-serve 命令分发 / assistant & executor & v2 runtime 的默认 MCP 注入`

判断：

- 当前主源码树里未见现行业务页面 import
- 主要承担 legacy UI 与测试资产的兼容 client 职责
- A2A 已从保留白名单移除，运行链开始摘除
- MCP 已从默认 server/runtime/assistant/executor 接线中摘除，`handlers_mcp.go` 已退化为归档 stub，CLI 默认入口也不再暴露 `mcp-serve`
- 后续可考虑把 `apiClient.ts` 改名为 `apiClientLegacy.ts`，并把 A2A / MCP 后端源码继续整体迁入 compat/legacy 目录

#### 3. 主要是测试与提示口径残留

- `internal/web/*_test.go` 中大量 `/api/v1/*` 集成测试
- `web/src/lib/wsClient.test.ts`
- `cmd/acp-probe/main.go` 中示例 MCP 地址
- 少量文档、脚本和提示文案中的 `/api/v1/*`

判断：

- 这批不构成主运行链阻力，但会持续制造“v1 仍是默认主链路”的错觉
- 适合在后续波次中跟随白名单逐项同步收敛

## 分层盘点

### A. 后端运行时与接口

| 项目 | 当前状态 | 判断 | 建议动作 |
|---|---|---|---|
| `cmd/ai-flow/server.go` | 主启动流程先起旧 runtime，再额外挂 `bootstrapV2(...)` | 双栈核心入口 | 最后清；先作为总控点，后续逐步把旧依赖摘掉 |
| `cmd/ai-flow/v2_bootstrap.go` | `v2` 运行时单独建库、事件总线、调度器、handler | 目标主线 | 保留，后续作为唯一目标运行时 |
| `internal/web/server.go` | 同时挂 `/api/v1` 和 `/api/v2` | 双栈出口 | 待迁移完成后收敛为 `/api/v2` 主入口 |
| `internal/web/handlers_v1_routes.go` | `/api/v1` 路由注册函数 `registerV1Routes` | legacy compat 主注册点 | 默认仅挂 `ws + admin ops`，旧业务路由已退成 archive-only 实现 |
| `internal/web/handlers_a2a.go`、`internal/teamleader/a2a_bridge.go` | A2A 源码仍在仓库，但已从主运行链摘除 | 已归档候选的 legacy 协议实现 | 后续可整体迁入 compat/archive 目录，或在确认无回滚需求后删除 |
| `internal/teamleader/*` | 含 legacy review path、compatibility interface、legacy field names | 兼容壳仍在运行链路 | 第二波处理；先明确默认流量是否还落旧评审链 |
| `internal/mcpserver/*`、`internal/teamleader/mcp_tools.go`、`internal/configruntime/manager.go` | MCP 默认接线已摘除；仍保留 `/api/v1/mcp`、`/api/v1/admin/ops/*` 的 legacy URL 生成与工具源码 | 已转为归档候选的 legacy MCP 实现 | 后续继续迁入 compat/archive 或直接删除 |
| `src-tauri/src/main.rs` | 暴露 `api_v1_base_url` 和 `api_v2_base_url`，当前仅 WS 仍走 `/api/v1` | 桌面壳仍双栈 | 后续需要统一 desktop bootstrap 契约 |
| `cmd/ai-flow/server.go` | 启动日志仍提示 `ws: /api/v1/ws` | 用户可见兼容口径残留 | 可较早改成“legacy ws”或双栈说明，避免继续误导 |

### B. 配置与协议

| 项目 | 当前状态 | 判断 | 建议动作 |
|---|---|---|---|
| `internal/config/defaults.toml` | 明写“`v2.agents` 为空时从 v1 推导” | `v2` 对 `v1` 的关键兼容依赖 | 第二波处理；先补齐纯 v2 配置样例与加载路径 |
| `internal/configruntime/materialize.go` | `BuildV2Agents` 支持从旧 `roles` / `agents` 转换 | 兼容桥核心 | 后续要么删除 fallback，要么下沉成一次性迁移工具 |
| `internal/config/types.go` | 含 legacy YAML dual-format 支持 | 老配置兼容层 | 可放到后段清理，先统计真实使用情况 |
| `configs/prompts/team_leader.tmpl` | 仍写死 `"contract_version": "v1"` | 旧协议标记仍活跃 | 第一波修正项，至少先改命名和说明 |

### C. 前端代码

| 项目 | 当前状态 | 判断 | 建议动作 |
|---|---|---|---|
| `web/src/App.tsx` | 已切到新的 `pages/` + `react-router` | 当前 UI 外壳 | 保留 |
| `web/src/pages/*` | 现行入口页已切到 `V2WorkbenchContext` + `apiClientV2` 主链路 | 当前工作台主线 | 保留，并继续补齐 `/api/v2` 覆盖缺口 |
| `web/src/lib/apiClient.ts` | 完整绑定 `/api/v1` 的 issue/run/chat/repo/admin API；当前主源码树里未见现行业务入口 import | 旧业务 client 已基本退到 legacy/测试资产 | 暂不删除；后续可按“legacy-only client” 继续归档或拆出兼容层 |
| `web/src/lib/apiClientV2.ts` | 绑定 `/api/v2` Flow/Step/Execution 模型 | 目标 client | 保留，并逐步扩展覆盖缺口 |
| `web/src/v2/*` | 已迁出到 `web/archive-src/legacy-ui/src/v2/*` | 已脱离主源码树 | 后续只在迁移对照或最终删除时再处理 |
| `web/src/v3/*` | 已迁出到 `web/archive-src/legacy-ui/src/v3/*` | 已脱离主源码树 | 后续只在迁移对照或最终删除时再处理 |
| `web/src/views/*` | 已迁出到 `web/archive-src/legacy-ui/src/views/*` | 已隔离的 legacy `/api/v1` UI 子树 | 不再作为现行工作台入口 |
| `web/src/archive/legacy/*`、`web/src/_archived/*` | 原先存在两层前端归档副本；其中 `_archived` 已删除，`archive/legacy` 已统一迁到 `web/archive-src/legacy-ui/src/archive/legacy/*` | 已完成一轮归档瘦身 | 后续统一在 `web/archive-src/legacy-ui/*` 下维护历史留档 |
| `web/src/stores/*` | 旧 `chat/projects/runs` stores 已迁出；当前 `src` 仅保留仍在主链路使用的 `settingsStore` | 主源码树已收敛 | 后续继续避免把 legacy 状态模型带回现行入口 |
| `web/src/lib/a2aClient.ts`、`web/src/types/a2a.ts` | 已迁出到 `web/archive-src/legacy-ui/src/lib`、`web/archive-src/legacy-ui/src/types` | A2A 前端资产已归档 | 后续无需再作为现行工作台能力维护 |

### D. 测试、脚本、文档

| 项目 | 当前状态 | 判断 | 建议动作 |
|---|---|---|---|
| `scripts/dev.sh` | 仍默认把 `VITE_API_BASE_URL` 指向 `/api/v1` | 本地开发入口仍偏旧 | 第一波修正项 |
| `scripts/test/v2-smoke.ps1`、`v2-pr-flow-smoke.ps1` | 已有 `/api/v2` 冒烟 | 目标链路验证基础 | 保留并扩充 |
| `internal/web/*_test.go` | 大量测试仍直接打 `/api/v1/*` | 活跃测试依赖 | 不能直接删；需先建立对应 `/api/v2` 测试面 |
| `README.md` | 把 `/api/v1` 称为 “V2 API 主链路” | 最容易误导协作 | 第一波修正项 |
| `CLAUDE.md` | 仍描述 `VITE_UI_VERSION` 和旧前端代际关系 | 过时协作文档 | 第一波修正项 |
| `docs/other/v3/*` | 明确说仓库里真正要推进的是 v3，不是 v2 | 与当前决策冲突 | 先标注“历史方案，不作为现行主线” |

## 优先级分组

### P0：先改，不改会持续制造误判

- `README.md`
- `CLAUDE.md`
- `configs/prompts/team_leader.tmpl`
- `internal/web/handlers_v1_routes.go` 的后续拆分
- `scripts/dev.sh`

目标：先统一语言，避免团队继续把 `/api/v1` 当“V2 主链路”。

### P1：先迁再删的活跃依赖

- `internal/web/server.go` 下的 `/api/v1` 路由组
- `web/src/lib/apiClient.ts`
- `internal/teamleader/*` 的 legacy review compatibility
- `internal/configruntime/materialize.go` 的 v1 -> v2 agent/profile fallback
- MCP/A2A/桌面壳里硬编码的 `/api/v1/*`

目标：这部分都在运行链路上，必须先找替代路径，不能粗暴删除。

### P2：可较早瘦身的历史资产

- `web/archive-src/legacy-ui/*`
- 重复测试副本
- 历史计划/设计中明显过时的版本切换描述

目标：降低认知噪音，减少后续误引用。

已完成：

- 删除重复前端归档目录 `web/src/_archived/*`
- 将非路由前端页面子树与 legacy-only 组件/store 统一迁到 `web/archive-src/legacy-ui/*`
- `web/src` 现已只保留现行路由链所需的 `pages`、`contexts`、`layouts`、主链路组件与必要 store
- `cmd/ai-flow/server.go` 启动提示已明确为 `api: /api/v2, legacy ws: /api/v1/ws`
- MCP 已从默认 web/server/runtime/assistant/executor 链路摘除，`mcp-serve` 不再作为 CLI 默认入口暴露
- `cmd/ai-flow/commands_mcp.go` 已迁入 `archive-src/legacy-backend/cmd-ai-flow/`，`cmd/ai-flow/adapters.go` 中的 MCP issue adapter 已从主线剥离
- `internal/web` 下的 A2A 实现文件已迁入 `archive-src/legacy-backend/internal-web-a2a/`，默认服务不再保留这些 legacy handler 源码
- `internal/teamleader/a2a_bridge.go`、`internal/teamleader/mcp_tools.go`、`internal/mcpserver/*` 已迁入 `archive-src/legacy-backend/`，主线只保留兼容壳类型与零接线
- `internal/web/handlers_mcp.go` 与 A2A/MCP 相关 legacy 测试文件也已迁入 `archive-src/legacy-backend/tests/`
- `cmd/a2a-smoke/*` 与 `internal/teamleader/a2a_types.go` 也已迁入 `archive-src/legacy-backend/`，主线不再保留 A2A 兼容入口
- `/api/v1` legacy business routes 已从默认 server runtime 摘除，仅保留 compat 白名单 `ws + admin ops`
- 桌面 bootstrap 与 MCP/dev 工具中的 `/api/v1/*` 兼容路径已补充 legacy 语义注释，并集中成常量，便于下一步继续替换

## 建议的三波执行法

### Wave 1：统一命名与协作文档

输出：

- README 改成“当前双栈状态 + 目标收敛方向”
- 明确 `/api/v1 = legacy`，`/api/v2 = target`
- 给 `docs/other/v3/*` 加历史方案标识
- 修正 `team_leader.tmpl` 中的旧 `contract_version` 口径

风险低，收益高，建议立刻做。

### Wave 2：前端先完成真实迁移

输出：

- 新 `pages/` 真正接上 `/api/v2`
- 给旧 `/api/v1` UI 子树建立明确 legacy 边界
- 把已归档的 `web/archive-src/legacy-ui/src/v2/*` 里仍有价值的数据逻辑搬入 `pages/`
- 清空现网入口对 `apiClient.ts` 的直接依赖

完成标志：

- 非归档目录中，不再有活跃入口 import `createApiClient`
- `web/src` 中不再存在 `views/*`、`v2/*`、`v3/*` 这类 legacy 页面目录
- `pages/` 不再依赖 mock data 作为主显示来源

### Wave 3：后端收敛与兼容层拆除

输出：

- `/api/v2` 覆盖现有业务主能力
- `/api/v1` 仅保留明确兼容白名单，或整体下线
- 删除 v1 -> v2 配置推导 fallback
- 清理 legacy review path / legacy token / legacy auth 注释与结构

完成标志：

- 主启动流程不再依赖旧 runtime
- 内部 URL 生成不再默认写 `/api/v1/*`

## 建议的下一步

下一步直接进入 Wave 1，比继续讨论更划算。建议我马上做这一波最小清理：

1. 修正 `README.md` 的版本口径。
2. 修正 `CLAUDE.md` 中过时的 UI 版本说明。
3. 继续拆分 `internal/web/handlers_v1_routes.go`。
4. 标注 `docs/other/v3/*` 为历史设计，不作为当前实施主线。

## 关键证据

- `cmd/ai-flow/server.go`
- `cmd/ai-flow/v2_bootstrap.go`
- `internal/web/server.go`
- `internal/web/handlers_v1_routes.go`
- `internal/config/defaults.toml`
- `internal/configruntime/materialize.go`
- `configs/prompts/team_leader.tmpl`
- `web/src/App.tsx`
- `web/src/pages/DashboardPage.tsx`
- `web/src/pages/ProjectsPage.tsx`
- `web/src/lib/apiClient.ts`
- `web/src/lib/apiClientV2.ts`
- `web/archive-src/legacy-ui/src/v2/AppV2.tsx`
- `web/archive-src/legacy-ui/src/v3/views/OverviewView.tsx`
- `README.md`
- `CLAUDE.md`
- `docs/other/v3/README.md`
