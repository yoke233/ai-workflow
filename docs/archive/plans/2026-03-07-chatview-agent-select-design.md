# ChatView Agent 选择 + 全面样式优化 设计

## 目标

1. ChatView 启动会话时可选择 ACP 运行时 agent（claude/codex），会话锁定后不可变
2. 全面优化 ChatView 界面样式，统一终端风格

## 设计决策

| 问题 | 决策 |
|------|------|
| 选择粒度 | 选择 Agent（role 不变，只换底层 agent 进程） |
| 交互方式 | 输入框旁下拉选择，新会话可选，已有会话只读 |
| Agent 列表来源 | 后端 `GET /api/v1/agents` 接口 |
| 样式优化范围 | 全面：左面板、聊天区、右 sidebar、输入区、整体布局 |

## 一、Agent 选择功能

### 后端

#### 新 API：`GET /api/v1/agents`

返回配置中可用的 agent profiles。

```json
{
  "agents": [
    { "name": "claude" },
    { "name": "codex" }
  ]
}
```

#### ChatSession 模型扩展

`internal/core/chat.go` 新增字段：

```go
type ChatSession struct {
    // ... 现有字段 ...
    AgentName string `json:"agent_name,omitempty"`
}
```

创建时写入，之后不可变。

#### 创建会话 API 扩展

`POST /api/v1/projects/{projectId}/chat` 请求新增可选字段：

```json
{ "message": "...", "agent_name": "codex" }
```

- 新会话：写入 `agent_name`，不指定则用默认（config 中 team_leader role 绑定的 agent）
- 追加消息（session_id 已存在）：忽略 `agent_name`，沿用会话的 agent

#### ChatAssistant 改动

`chat_assistant_acp.go` 的 `Reply` 方法：

- `ChatAssistantRequest` 新增 `AgentOverride string`
- 如果指定了 AgentOverride，直接从 RoleResolver 的 agent 列表中查找该 agent profile，跳过 role 默认的 agent 绑定
- pooled session 的 key 要包含 agent name，避免不同 agent 复用同一个进程

### 前端

#### apiClient 扩展

```typescript
// types/api.ts
export interface AgentInfo {
  name: string;
}
export interface AgentListResponse {
  agents: AgentInfo[];
}
export interface CreateChatRequest {
  message: string;
  session_id?: string;
  agent_name?: string;  // 新增
}
```

#### ChatView 输入区

- 输入框上方添加 agent 下拉选择器
- 新会话时可选（enabled），已有会话时显示为只读标签
- 默认选中第一个（通常是 claude）
- 会话详情（右侧 sidebar）显示当前 agent name

## 二、全面样式优化

### 整体布局

- 三栏改为全高度 `h-screen` flex 布局，不再内部固定 `h-[30rem]`
- 全局字体 `font-mono`

### 左侧面板

- 收窄到 `w-56`
- 文件树和 Git 面板字体缩小
- Tab 切换改为紧凑 pill 样式

### 中央聊天区

- 消息容器 `flex-1` 填满可用空间
- 时间戳行内右侧浮动
- 分隔线更细 `border-slate-100`
- 流式渲染添加 cursor 闪烁动画

### 右侧 sidebar

- 去圆角、减小 padding，终端紧凑风格
- 会话列表项高度压缩，状态点改为行内 badge
- Issue 面板等宽字体
- 运行事件列表 TUI 日志风格

### 输入区

- 高度自适应（min 2 行，max 6 行）
- 发送按钮和 agent 选择器同一行
- 更紧凑

## 涉及文件

| 层 | 文件 | 改动 |
|----|------|------|
| 后端 model | `internal/core/chat.go` | ChatSession 加 AgentName |
| 后端 store | `internal/plugins/store-sqlite/chat.go` | 存取 agent_name |
| 后端 migration | `internal/plugins/store-sqlite/migrations.go` | ALTER TABLE |
| 后端 handler | `internal/web/handlers_chat.go` | 接受 agent_name 参数 |
| 后端 handler | `internal/web/handlers_agents.go` | 新增 GET /api/v1/agents |
| 后端 assistant | `internal/web/chat_assistant_acp.go` | 支持 agent override |
| 后端 server | `cmd/ai-flow/server.go` | 注册新路由 |
| 前端 types | `web/src/types/api.ts` | 新增类型 |
| 前端 apiClient | `web/src/lib/apiClient.ts` | 新增 listAgents |
| 前端 view | `web/src/views/ChatView.tsx` | agent 选择器 + 全面样式 |
| 前端 test | `web/src/views/ChatView.test.tsx` | 适配新 DOM |
