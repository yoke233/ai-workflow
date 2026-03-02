# Spec 重写计划 — Secretary 流程重设计

## 背景

将 Secretary 从"一次性 JSON 调用"改为"持久交互式 Agent session"，计划文件从 DB-only 改为文件驱动，新增 git clone 项目创建、Secretary 查询工具、Admin UI。

## 一、spec-overview.md 改动

### 1.1 架构图更新

Secretary Agent 不再是内部组件，而是一个持久运行的 Agent session：

```
接入层：Web Workbench (主) │ TUI (备选) │ GitHub Webhook (可选)
         │
Secretary Layer (持久 session)
  ├── Secretary Agent (Claude/可切换, 工作目录=项目目录, 有文件读写权限)
  │   ├── 多轮对话
  │   ├── 生成计划文件 (格式自由, 写入项目目录)
  │   └── 查询工具 (项目进度/Pipeline状态/日志)
  ├── Plan Parser (从用户选定的文件解析为结构化 TaskPlan)
  ├── Multi-Agent Review Panel (审核+修正)
  └── DAG Scheduler (依赖并行调度)
         │
Orchestrator Core (不变)
  ├── Pipeline Engine
  ├── Scheduler
  └── Plugin 层
```

### 1.2 核心数据流更新

```
1. 创建项目 (选本地目录 / 输入 git URL → clone 到 ~/.ai-workflow/repos/)
2. 进入 Workbench → Chat View
3. 多轮对话 (Secretary = 持久 session, 工作目录=项目目录)
   - 读写项目文件、运行命令、理解代码
   - 通过工具查询项目进度、Pipeline 状态
4. 用户指示 Secretary 生成计划文件
   - Secretary 在项目目录生成文件 (格式自由: .md/.json/.yaml/混合)
   - 写入约定路径如 .ai-workflow/plans/ 或任意位置
5. 前端展示新增/变更文件列表 → 用户勾选
6. 提交勾选文件 → 后端调用 AI 解析为结构化 TaskPlan + TaskItems
7. Multi-Agent 审核 → 修正循环 → 人工确认
8. DAG Scheduler 拆任务 → 并行执行 Pipeline
9. 收尾 (merge, cleanup)
10. 全程审计日志

直接模式 (兼容 P0): 跳过 Secretary, 直接创建 Pipeline
```

### 1.3 项目创建方式

新增两种项目创建方式：
- **选择本地目录** — 指向已有的 git 仓库
- **Git Clone** — 提供 URL, 系统 clone 到 `~/.ai-workflow/repos/{project-id}/`

### 1.4 插件槽位更新

无新增槽位。Secretary Agent 复用 Agent + Runtime 插件。

---

## 二、spec-secretary-layer.md 改动 (最大)

### 2.1 Secretary Agent — 持久 Session 模式

**删除**：
- 一次性 `-p` 调用 + JSON 输出模式
- 自动生成 TaskPlan 的默认行为

**新增**：
- 持久 session 交互模式：
  - 项目打开时启动 Secretary Agent 进程 (通过 Runtime.Create)
  - 用户每条消息通过 Runtime.Send() 发送到 stdin
  - Agent 回复通过 StreamParser 从 stdout 读取
  - Session 保持直到用户关闭项目或显式结束
- 工作目录 = 项目目录 (有文件读写权限)
- Agent 可切换 (默认 Claude, 可配置为其他)
- AllowedTools 配置：
  - Read(*), Write(*), Edit(*) — 文件操作
  - Bash(git *), Bash(ls *), Bash(cat *), Bash(find *) — 项目探索
  - Bash(go *), Bash(npm *) 等 — 按项目技术栈配置
  - 自定义查询工具 — 见 2.4

**保留**：
- 对话历史持久化到 SQLite (ChatSession 表)
- 对话管理规则 (多 session, 历史截断等)

### 2.2 计划文件 — 文件驱动模式

**删除**：
- Secretary 输出 JSON → 直接解析为 TaskPlan 的流程
- 严格 JSON schema 输出要求

**新增**：
- Secretary 按用户指令生成计划文件 (格式自由)
- 文件写入项目目录 (推荐但不强制 .ai-workflow/plans/)
- 前端监测文件变更:
  - WebSocket 事件 `secretary_files_changed` 推送新增/修改的文件列表
  - 前端展示变更文件, 用户勾选哪些作为 Plan 输入
- 用户提交勾选 → 后端 Plan Parser:
  - 调用 AI (Claude Driver) 读取选中文件内容
  - 解析为结构化 TaskPlan + TaskItems
  - TaskPlan 引用源文件路径列表 (source_files 字段)
  - 写入 Store, 状态 draft

### 2.3 TaskPlan 数据模型更新

```go
type TaskPlan struct {
    // ... 现有字段保留 ...
    SourceFiles []string  // 新增: 计划来源文件路径列表
}
```

TaskItem 结构不变。

### 2.4 Secretary 查询工具

Secretary Agent 在对话中可通过特殊工具查询系统状态。实现方式：通过 prompt 注入工具描述 + 解析 Agent 的 tool_use 事件 + 后端执行查询 + 将结果注入回 Agent stdin。

可用工具：

| 工具名 | 功能 | 返回 |
|--------|------|------|
| `query_plans` | 列出当前项目所有 TaskPlan | ID, name, status, task count |
| `query_plan_detail` | 查看某个 Plan 详情 | tasks, DAG, review status |
| `query_pipelines` | 列出项目下活跃 Pipeline | ID, status, current stage, progress |
| `query_pipeline_logs` | 查看某 Pipeline 的日志 | 最近 N 条日志 |
| `query_project_stats` | 项目统计 | 总 pipeline 数, 成功率, token 消耗 |

实现架构：
```
用户消息 → Secretary Agent (持久 session)
  ↓
Agent 输出 tool_use 事件 (如 query_plans)
  ↓
后端拦截 → 执行查询 (调 Store 接口)
  ↓
结果注入 Agent stdin (作为 tool_result)
  ↓
Agent 继续生成回复
```

### 2.5 Multi-Agent 审核 — 输入变更

审核流程基本不变，但输入从"Secretary 直接输出的 JSON"变为"Plan Parser 解析后的 TaskPlan"。

Reviewer prompt 中额外注入源文件内容 (SourceFiles), 使审核 Agent 能理解原始计划意图。

修正循环中 Aggregator 的 fix 行为：输出修正后的 TaskItems JSON, 直接替换 (不修改源文件)。

### 2.6 Workbench UI 更新

#### Chat View 改动:
- 移除"生成任务清单"按钮
- 新增: 文件变更提示区 (Secretary 生成文件后弹出)
- 新增: 文件勾选 + "创建计划" 按钮
- 流式输出改为持久 session 模式 (非一次性)

#### 新增: Admin View (/admin)
- 全局视角, 跨项目
- 所有项目列表 + 活跃 Pipeline 概览
- 全局 Pipeline 列表 (可按项目/状态/时间过滤)
- 全局 TaskPlan 列表
- 审计日志浏览器 (按时间/项目/操作类型过滤)
- 系统资源监控 (Agent 并发数, 信号量使用)

#### Workbench 项目级管理视图:
- 在侧栏增加"管理"Tab
- 项目下所有 Pipeline 的状态面板
- 项目下所有 Plan 的概览
- 项目级审计日志

---

## 三、spec-agent-drivers.md 改动

### 3.1 Runtime 接口 — Send() 方法重要性提升

Send() 不再只用于"人工 inject"，现在是 Secretary 每条消息的主通道。需要明确：
- Send() 写入 Session.Stdin
- 消息格式: 纯文本 (由 Agent prompt 协议决定)
- 消息边界: 以换行符分隔

### 3.2 Secretary 的 Agent × Runtime 协作

```
项目打开时:
  → cmd := Agent.BuildCommand(opts)   // opts.WorkDir = 项目目录
  → session := Runtime.Create(ctx, RuntimeOpts{Command: cmd})
  → parser := Agent.NewStreamParser(session.Stdout)

用户每条消息:
  → Runtime.Send(sessionID, message)
  → parser.Next() 逐事件读取回复
  → 遇到 tool_use 事件 → 后端处理 → 结果写回 stdin

项目关闭时:
  → Runtime.Kill(sessionID)
```

### 3.3 Secretary AllowedTools 配置

```yaml
secretary:
  agent: claude                     # 可切换
  allowed_tools:
    - "Read(*)"
    - "Write(*)"
    - "Edit(*)"
    - "Bash(git *)"
    - "Bash(ls *)"
    - "Bash(find *)"
    - "Bash(cat *)"
    - "Bash(mkdir *)"
    # 项目技术栈相关 (自动检测或手动配置)
    - "Bash(go *)"
    - "Bash(npm *)"
```

### 3.4 Claude 持久 Session 模式

当前 spec 只描述了 `claude -p` (非交互模式)。Secretary 需要交互模式：

**方案**: 使用 `claude` 不带 `-p`, 通过 stdin/stdout 交互。
- 或者使用 Claude SDK/API 直接调用 (绕过 CLI)
- 具体方案取决于 Claude CLI 是否支持持久 stdin 交互

> 实现时需要验证 Claude CLI 的交互模式是否适合程序化 stdin/stdout 通信。
> 如果不适合, 可能需要直接用 Anthropic API 构建 Secretary Agent (而非通过 CLI)。

---

## 四、spec-api-config.md 改动

### 4.1 项目 API 更新

```
POST /api/v1/projects
  Body: {
    name,
    source: "local" | "git",
    repo_path: "/path/to/repo",        // source=local 时
    git_url: "https://...",            // source=git 时
    git_branch: "main"                 // 可选
  }
  → 201: { id, name, repo_path, ... }

  source=git 时:
  - clone 到 ~/.ai-workflow/repos/{id}/
  - repo_path 设为 clone 后的路径
```

### 4.2 Chat API 更新 — 持久 Session

```
POST /api/v1/projects/:pid/chat/sessions
  → 201: { session_id }
  注: 后端启动 Secretary Agent 进程

POST /api/v1/projects/:pid/chat/sessions/:sid/messages
  Body: { content: "..." }
  → 200: { message_id }
  注: 消息通过 Runtime.Send() 发给 Agent, 回复通过 WS 流式推送

DELETE /api/v1/projects/:pid/chat/sessions/:sid
  → 204
  注: 终止 Secretary Agent 进程
```

### 4.3 文件选择 API (新增)

```
GET /api/v1/projects/:pid/files
  Query: ?path=.ai-workflow/plans/&changed_since=2026-03-01T00:00:00Z
  → 200: { files: [{ path, size, modified_at, is_new }] }
  注: 列出项目目录下指定路径的文件, 支持按修改时间过滤

GET /api/v1/projects/:pid/files/content
  Query: ?paths=file1.md,file2.md
  → 200: { files: [{ path, content }] }
  注: 读取指定文件内容 (前端预览用)

POST /api/v1/projects/:pid/plans/from-files
  Body: { session_id: "...", file_paths: ["plan1.md", "plan2.md"] }
  → 201: { plan_id, name, tasks: [...], status: "draft" }
  注: 后端调用 AI 解析文件为 TaskPlan
```

### 4.4 Admin API (新增)

```
GET /api/v1/admin/overview
  → 200: {
    projects: { total, active },
    pipelines: { running, waiting, done_today, failed_today },
    plans: { executing, reviewing },
    agents: { active, max }
  }

GET /api/v1/admin/pipelines
  Query: ?project_id=&status=&limit=50&offset=0
  → 200: { items: [...], total }

GET /api/v1/admin/plans
  Query: ?project_id=&status=&limit=50&offset=0
  → 200: { items: [...], total }

GET /api/v1/admin/audit-log
  Query: ?project_id=&action=&user=&since=&until=&limit=100&offset=0
  → 200: { items: [{ timestamp, project_id, action, target, user, detail }], total }
```

### 4.5 WebSocket 新增事件

| 事件 | 触发 | 数据 |
|------|------|------|
| `secretary_files_changed` | Secretary 写入/修改文件 | file_paths, session_id |
| `secretary_tool_call` | Secretary 调用查询工具 | tool_name, input |
| `secretary_tool_result` | 查询工具返回结果 | tool_name, output |

### 4.6 数据库 Schema 更新

#### audit_log 表 (新增)

```sql
CREATE TABLE audit_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id  TEXT,                    -- 可为空 (系统级操作)
    action      TEXT NOT NULL,           -- 操作类型
    target_type TEXT,                    -- project / pipeline / plan / task / chat
    target_id   TEXT,
    user_id     TEXT DEFAULT 'system',
    detail      TEXT DEFAULT '{}',       -- JSON
    timestamp   DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_audit_log_project ON audit_log(project_id);
CREATE INDEX idx_audit_log_action ON audit_log(action);
CREATE INDEX idx_audit_log_timestamp ON audit_log(timestamp);
```

操作类型枚举：
- project_created, project_deleted
- chat_session_started, chat_session_ended, chat_message_sent
- plan_files_generated, plan_created_from_files
- plan_review_started, plan_review_completed, plan_approved, plan_rejected
- task_dispatched, task_completed, task_failed, task_retried, task_skipped
- pipeline_created, pipeline_started, pipeline_completed, pipeline_failed
- human_action (approve/reject/modify/skip/abort)
- agent_session_started, agent_session_ended

#### projects 表更新

```sql
ALTER TABLE projects ADD COLUMN source TEXT DEFAULT 'local';  -- 'local' | 'git'
ALTER TABLE projects ADD COLUMN git_url TEXT;
ALTER TABLE projects ADD COLUMN git_branch TEXT;
```

#### task_plans 表更新

```sql
ALTER TABLE task_plans ADD COLUMN source_files TEXT DEFAULT '[]';  -- JSON array of file paths
```

### 4.7 Store 接口更新

```go
// 新增
AppendAuditLog(entry AuditEntry) error
GetAuditLogs(filter AuditFilter) ([]AuditEntry, int, error)
```

### 4.8 Secretary 配置更新

```yaml
secretary:
  agent: claude                      # Secretary 使用的 Agent (可切换)
  session_idle_timeout: 30m          # 空闲超时自动关闭 session
  allowed_tools: [...]               # 见 3.3
  query_tools_enabled: true          # 启用查询工具
  context_max_tokens: 4000           # 项目上下文 token 预算 (保留)
  # 移除: session_max_messages (持久 session 由 Agent 自行管理上下文)
```

---

## 五、spec-pipeline-engine.md 改动 (最小)

### 5.1 Pipeline 创建来源更新

Pipeline 来源表新增说明：DAG Scheduler 创建 Pipeline 时, TaskItem 的来源是"文件解析后的结构化数据"而非"Secretary 直接输出的 JSON"。

### 5.2 无其他改动

Pipeline Engine 的 Stage 定义、状态机、Reactions Engine 等均不受影响。

---

## 六、spec-github-integration.md 改动 (最小)

### 6.1 无核心改动

GitHub 集成仍为 P3 可选增强。Tracker 同步的数据来源从"Secretary JSON"变为"文件解析后的 TaskPlan"，但 Tracker 接口不变。

---

## 七、执行顺序

1. spec-overview.md — 更新架构图和数据流 (全局上下文)
2. spec-secretary-layer.md — 主要重写 (Section I, II, VIII)
3. spec-agent-drivers.md — 新增持久 session 模式
4. spec-api-config.md — 新增 API、Schema、配置
5. spec-pipeline-engine.md — 微调
6. spec-github-integration.md — 微调

## 八、风险和待定项

1. **Claude CLI 持久交互模式** — 需验证 `claude` (不带 `-p`) 是否支持程序化 stdin/stdout 通信。如不支持, 需改用 Anthropic API 直接调用。
2. **文件变更检测** — Secretary 写文件后如何通知前端？方案: Agent 的 Write/Edit tool_use 事件触发 WS 推送。
3. **Plan Parser 的鲁棒性** — 文件格式自由意味着 Parser 需要足够智能。兜底: 如果解析失败, 返回错误让用户在 Chat 中调整文件格式。
4. **Session 生命周期** — 持久 session 的内存占用、超时清理、崩溃恢复策略需要明确。
5. **审计日志量** — agent_output 级别的日志可能很大, 需要和现有 logs 表区分 (audit_log 只记操作, 不记输出流)。
