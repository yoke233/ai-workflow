# ChatView TUI 终端风格改造设计

## 目标

将 ChatView 从卡片式聊天界面改为类似 Claude Code CLI 的 TUI 终端风格，提升信息密度和阅读体验。

## 设计决策

| 问题 | 决策 |
|------|------|
| 改造范围 | 仅 ChatView，A2AChatView 不动 |
| 语法高亮 | 引入 `react-syntax-highlighter`（Prism），完整语法高亮 |
| 右侧导航条 | 细长条标记 + hover tooltip 摘要 |
| 用户输入标记 | 图标前缀（`👤`），视觉上稍突出 |
| 活动事件 | 灰色背景缩进折叠块，默认折叠显示摘要 |

## 视觉规范

### 整体布局

```
┌──────────┬────────────────────────────┬──┬──────────┐
│ 左侧面板  │    TUI 风格消息流           │条│ 右侧 Bar │
│ FileTree │  👤 用户输入...             │  │ 会话列表  │
│ GitStatus│  • 助手回复...             │  │ Issue 等  │
│          │  ┌─ Ran command... ──────┐ │  │          │
│          │  │  (折叠的工具调用块)     │ │  │          │
│          │  └──────────────────────┘ │  │          │
│          │  ────────────────────────  │  │          │
│          │  👤 用户输入...             │  │          │
└──────────┴────────────────────────────┴──┴──────────┘
                                        ↑
                                    导航标记条
```

- 左侧面板（FileTree / GitStatus）**不变**
- 右侧 Sidebar（会话列表、Issue 管理）**不变**
- 中央聊天区改为 TUI 平铺风格
- 导航标记条插入在聊天区和右侧 Sidebar 之间

### 消息样式

**用户消息：**
- `👤` 图标前缀 + 用户内容
- 浅灰背景条 `bg-slate-50` 区分
- 字体略加粗

**助手消息：**
- `•` bullet 前缀
- 白色背景，普通文本平铺
- 支持完整 Markdown 渲染

**消息分隔：**
- 消息之间用 `<hr>` 水平分隔线隔开
- 分隔线颜色 `border-slate-200`

### 活动事件（tool_call / agent_thought / plan）

- 灰色背景 `bg-slate-100` + 左侧 border `border-l-2 border-slate-300`
- 缩进显示（`ml-4`）
- 默认折叠：只显示摘要行（如 `Ran rg -n "..."` 或 `Thinking...`）
- 点击展开显示完整输出
- 折叠时显示 `… +N lines` 提示
- tool_call_group 合并为一个折叠块

### 代码块 & 语法高亮

- 使用 `react-syntax-highlighter` 的 Prism 版本
- 主题选用 `oneDark` 或 `vscDarkPlus`
- 代码块带语言标签和复制按钮
- 行内代码保持 `bg-slate-100 rounded px-1` 样式

### 右侧导航标记条

- 宽度 `12px`，固定在聊天滚动区右侧
- 高度与聊天区等高，背景 `bg-slate-50`
- 每个用户消息位置对应一个标记点（`bg-blue-400 rounded-full w-2 h-2`）
- 标记点位置 = `(该消息在总内容中的位置比例) * 导航条高度`
- 点击标记 → `scrollIntoView` 跳转到对应用户消息
- Hover 标记 → tooltip 显示用户输入前 30 字

## 新增依赖

```
react-syntax-highlighter
@types/react-syntax-highlighter
```

## 涉及文件

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `web/package.json` | 修改 | 添加依赖 |
| `web/src/views/ChatView.tsx` | 重构 | 消息渲染逻辑重写 |
| `web/src/components/TuiMessage.tsx` | 新建 | TUI 风格消息组件（用户/助手） |
| `web/src/components/TuiActivityBlock.tsx` | 新建 | 折叠式活动事件块 |
| `web/src/components/TuiCodeBlock.tsx` | 新建 | 带语法高亮的代码块 |
| `web/src/components/TuiMarkdown.tsx` | 新建 | 增强版 Markdown 渲染器 |
| `web/src/components/ScrollNavBar.tsx` | 新建 | 右侧导航标记条 |

## 不变的部分

- 左侧面板组件
- 右侧 Sidebar 组件
- Store / 类型定义 / API 通信层
- 输入框 textarea（仅样式微调匹配终端风格）
- CommandPalette / ConfigSelector
