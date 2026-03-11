# V2 Architecture

## 1. 定位

以流程编排为核心的 AI 自动化工作平台。

引擎负责：编排 Step 依赖、调度 Agent 执行、组装任务书、校验交付物、推动 Gate 流转。

引擎不负责：Agent 内部思考方式、具体 prompt 措辞、具体工具调用策略。

核心原则：**引擎管约束，Agent 管执行。**

---

## 2. 实体模型

7 个核心实体：

```
Project
 └─ Flow
      └─ Step
           ├─ Briefing
           ├─ Execution[]
           │    └─ Artifact
           └─ AgentContext
```

加上横切的 Event。

### 2.1 Project

工作目标。承载多个 Flow 和共享资料。

```
Project
  id              int64
  name            string
  description     string
  status          active | archived
  metadata        map[string]string
  created_at      time
  updated_at      time
```

### 2.2 Flow

一条可执行的 Step DAG。入口 Step = DependsOn 为空，由引擎自动推导。

```
Flow
  id              int64
  project_id      int64
  name            string
  status          pending | running | blocked | failed | done | cancelled
  parent_step_id  *int64            — 子 Flow 指向父 composite Step
  metadata        map[string]string
  created_at      time
  updated_at      time
```

### 2.3 Step

Flow 中唯一被编排的单元。

```
Step
  id                      int64
  flow_id                 int64
  name                    string
  type                    exec | gate | composite
  status                  pending | ready | running | waiting_gate | blocked | failed | done | cancelled
  depends_on              []int64
  sub_flow_id             *int64          — composite → 子 Flow
  agent_role              string          — lead | worker | gate | support
  required_capabilities   []string        — 能力 tag，选人用
  acceptance_criteria     []string        — 验收标准，Gate 评判用
  timeout                 duration
  config                  map[string]any
  max_retries             int
  retry_count             int
  created_at              time
  updated_at              time
```

Step 类型说明：
- **exec** — 执行型，Agent 干活并提交 Artifact
- **gate** — 门禁型，Agent 审核上游 Artifact 并判定 pass/reject
- **composite** — 展开为子 Flow，子 Flow 全绿后父 Step done

### 2.4 Briefing

引擎给 Agent 的任务书。不是 prompt，是 prompt 的素材。

```
Briefing
  id              int64
  step_id         int64
  objective       string              — 做什么
  context_refs    []ContextRef        — 可访问的上下文引用
  constraints     []string            — 限制条件
  created_at      time
```

ContextRef 类型：
```
ContextRef
  type            flow_summary | project_brief | upstream_artifact | agent_memory
  ref_id          int64
  label           string
```

引擎组装 Briefing 时按 prefix cache 友好顺序排列上下文：
1. Agent 身份（几乎不变）
2. Project 摘要（项目周期内稳定）
3. Flow 摘要 + 验收标准（Flow 周期内稳定）
4. 上游 Artifact 摘要（偶尔变）
5. 当前 Step Briefing（每次执行都变）

### 2.5 Execution

Step 的一次执行尝试。一个 Step 可有多次 Execution（重试、返工）。

```
Execution
  id                int64
  step_id           int64
  flow_id           int64
  status            created | running | succeeded | failed | cancelled
  agent_id          string
  agent_context_id  *int64
  briefing_snapshot string            — 执行时的 Briefing 快照
  artifact_id     *int64            — 指向 Artifact
  action_log        []ActionEntry     — Agent 调用了哪些 action
  error_message     string
  error_kind        transient | permanent | need_help
  attempt           int
  started_at        *time
  finished_at       *time
  created_at        time
```

ActionEntry：
```
ActionEntry
  action          string              — read_context | search_files | fs_write | terminal | submit | mark_blocked | ...
  timestamp       time
  detail          string
```

Agent 只能宣布"本次 Execution 已完成"，是否继续流转由引擎决定。

### 2.6 Artifact

统一交付包。每次 Execution 提交一个 Artifact。

```
Artifact
  id              int64
  execution_id    int64
  step_id         int64
  flow_id         int64
  result_markdown string              — Agent 自然语言输出（result.md 内容）
  metadata        map[string]any      — 引擎 Collect 阶段用小模型提取的结构化数据
  assets          []Asset             — 附件列表
  created_at      time
```

Asset：
```
Asset
  name            string
  uri             string
  media_type      string
```

result.md 必须包含：做了什么、结果是什么、交付清单、给下游的说明。

metadata 约定字段：
```yaml
status: completed | partial | blocked
verdict: pass | reject          # Gate 专用
reject_targets: [step_id, ...]  # Gate reject 时指定打回哪些
deliverables:
  - type: branch
    ref: "feat/login"
issues: []
next_steps: []
```

Gate Step 的 Artifact.metadata.verdict 决定流转：
- pass → Gate done → 下游 promote
- reject → 上游 reset → 审核意见注入下一次 Briefing

### 2.7 AgentContext

Agent 在 Flow 内的私有上下文。同 Agent 跨 Execution 复用，不同 Agent 不共享。

```
AgentContext
  id              int64
  agent_id        string
  flow_id         int64
  system_prompt   string
  session_id      string
  summary         string
  turn_count      int
  created_at      time
  updated_at      time
```

### 2.8 Event

领域事件。可持久化也可只广播。

```
Event
  id              int64
  type            EventType
  flow_id         int64
  step_id         int64
  exec_id         int64
  data            map[string]any
  timestamp       time
```

EventType 枚举：
```
flow.started / flow.completed / flow.failed / flow.cancelled
step.ready / step.started / step.completed / step.failed / step.blocked
exec.created / exec.started / exec.succeeded / exec.failed
gate.passed / gate.rejected
```

---

## 3. Agent 模型

### 3.1 四种角色

| 角色 | 职责 |
|------|------|
| **lead** | 拆分规划、维护 Flow、展开 composite Step |
| **worker** | 执行具体 Step，提交 Artifact |
| **gate** | 审核上游 Artifact，判定 pass/reject |
| **support** | 通用辅助（检索、格式化、摘要） |

### 3.2 Agent Profile

```
AgentProfile
  id                  string
  role                lead | worker | gate | support
  driver              claude | codex | human
  launch_command      string
  launch_args         []string
  capabilities        []string        — 能力 tag（dev.backend / test.qa / ...）
  actions_allowed     []string        — 允许的动作
  capabilities_max    ActionSet       — 驱动级上限
```

### 3.3 Action 白名单

| Action | lead | worker | gate | support |
|--------|------|--------|------|---------|
| read_context | Y | Y | Y | Y |
| search_files | Y | Y | Y | Y |
| fs_write | Y | Y | N | N |
| terminal | Y | Y | N | N |
| submit | Y | Y | Y | Y |
| mark_blocked | Y | Y | Y | Y |
| request_help | Y | Y | Y | Y |
| approve / reject | N | N | Y | N |
| create_step | Y | N | N | N |
| expand_flow | Y | N | N | N |

引擎强制执行。Agent 调用未授权 action → 引擎拒绝。

### 3.4 能力匹配

Step.required_capabilities 与 AgentProfile.capabilities 做交集匹配。引擎选人时：
1. 过滤 role 匹配的 agent
2. 检查 capabilities 覆盖
3. 校验 actions_allowed ⊇ step 需要的动作

---

## 4. 引擎执行管道

每个 Step 执行走 3 阶段管道（engine/pipeline.go）：

```
1. prepare    — 选人 + 组卷 + 校验能力 + 加 timeout
                Resolver.Resolve → BriefingBuilder.Build → guard check (inline)
2. execute    — 注入的 executor callback，引擎不管内部实现
3. finalize   — 分类错误 + 重试/阻塞/失败 + 收 artifact + gate 自动判定
                handleFailure (error_kind 三分类) / handleSuccess (gate/done)
```

3 个可注入接口：
- **Resolver** — `Resolve(ctx, step) → agentID`
- **BriefingBuilder** — `Build(ctx, step) → Briefing`
- **Collector** — `Extract(ctx, stepType, markdown) → metadata`（小模型提取）

### Collect 阶段：小模型提取

Agent 只输出自然语言 markdown（result_markdown），不需要感知 metadata schema。
引擎在 Collect 阶段用小型快速模型（如 Haiku）自动提取结构化 metadata。

```
Agent (主模型)            Engine Collect 阶段
┌──────────────┐         ┌──────────────────────────┐
│ 专注任务输出   │  ──→   │ 1. 存 result_markdown     │
│ 自然语言 MD   │        │ 2. 调小模型 + tool_use     │
│              │        │    提取 metadata (JSON)    │
│              │        │ 3. 组装 Artifact → 写入 DB │
└──────────────┘         └──────────────────────────┘
```

按 Step.Type 使用不同 extraction schema：
- **exec**: `{files_changed, tests_passed, summary}`
- **gate**: `{verdict: "pass"|"reject", reasons: [...]}`（固定 schema）
- **composite**: `{sub_tasks: [...]}`

优势：
- Agent 不分心，输出质量更高
- 提取是确定性任务，小模型 + tool_use 近乎 100% 准确
- 成本可忽略（几百 token，主模型的 ~1/50）
- Schema 变化不影响 agent prompt

### 4.1 调度循环

```
Run(flowID):
  1. 加载 Flow + Steps
  2. DAG 无环验证（Kahn 拓扑排序）
  3. 推导入口 Steps → 标记 ready
  4. 循环:
     a. 扫描 pending steps，deps 全 done → promote 到 ready
     b. 扫描 ready steps → 并发执行（semaphore 限流）
     c. exec succeeded → step done → 检查下游
     d. exec failed → 检查 error_kind:
        - transient + retry 未超限 → step 回 pending
        - permanent / 超限 → step failed
        - need_help → step blocked
     e. gate step: 读 artifact.metadata.verdict
        - pass → step done
        - reject → 上游 reset + 审核意见注入下一次 briefing
     f. composite step: expand → 创建子 flow → 递归 run → 子 flow done → 父 step done
  5. 全部 step done → flow done
  6. 不可恢复失败 → flow failed
```

### 4.2 Gate 自动化

```
Gate Step 执行:
  1. 引擎收集上游 Artifact（DependsOn 的最新 Artifact）
  2. 组装 Gate Briefing:
     - 待审物: 上游 Artifact 列表
     - 验收标准: 上游 Step 的 acceptance_criteria
  3. 分配 role=gate 的 Agent
  4. Agent 审核后提交 Artifact:
     - metadata.verdict: pass | reject
     - metadata.reject_targets: [step_id, ...]
     - result.md: 审核意见
  5. 引擎读 verdict → pass 继续 / reject 打回
```

### 4.3 Composite 展开

```
Composite Step 执行:
  1. 分配 role=lead 的 Agent
  2. Agent 提交 Artifact:
     - metadata.sub_steps: [{name, type, depends_on, capabilities_required}, ...]
  3. 引擎读 metadata.sub_steps → 创建子 Flow + 子 Steps
  4. 递归 Run 子 Flow
  5. 子 Flow done → 父 Step done
  6. 子 Flow failed → 父 Step 可重试（Lead 重新拆分）
```

### 4.4 超时与取消

- 每个 Step 执行用 context.WithTimeout 包裹
- Flow.Cancel → 取消所有 running execution 的 context
- 超时 → execution failed (error_kind: transient) → 按重试逻辑处理

---

## 5. 上下文管理

### 5.1 隔离边界

上下文隔离单位 = AgentID × FlowID。

### 5.2 三层记忆

| 层 | 内容 | 变化频率 | 缓存策略 |
|----|------|---------|---------|
| 冷 | Agent 历史经验摘要 | 极少变 | Compact 后失效 |
| 温 | 父 Flow / 兄弟 Step 摘要 | 偶尔变 | 版本号控制 |
| 热 | 当前 Step 的最近 Execution 和 Artifact | 每次变 | 不缓存 |

### 5.3 上下文传递

Step 间不共享私有上下文。信息通过 Artifact 流转：
- 上游产出 Artifact
- 引擎读 Artifact → 组装下游 Briefing
- 下游 Agent 通过 Briefing 获取所需信息

### 5.4 retrieval-first

Agent 遇到问题默认先查资料（read_context / search_files），不默认升级。
只有查不到、资料冲突、需要新决策时触发 request_help → step blocked。

---

## 6. 状态机

### 6.1 Flow

```
pending → running → done
                  → failed
                  → cancelled
         → blocked → running
                   → failed
                   → cancelled
```

### 6.2 Step

```
pending → ready → running → done
                          → failed → pending (retry)
                                   → cancelled
                          → waiting_gate → done
                                        → blocked → ready
                                                  → pending
                                                  → cancelled
                          → blocked → ready
                                    → pending
                                    → cancelled
                          → cancelled
        → cancelled
```

### 6.3 Execution

```
created → running → succeeded
                  → failed
                  → cancelled
```

---

## 7. Store 接口

```
Store
  FlowStore
    CreateFlow / GetFlow / ListFlows / UpdateFlowStatus
  StepStore
    CreateStep / GetStep / ListStepsByFlow / UpdateStepStatus / UpdateStep
  ExecutionStore
    CreateExecution / GetExecution / ListExecutionsByStep / UpdateExecution
  ArtifactStore
    CreateArtifact / GetArtifact / GetLatestByStep / ListByExecution
  AgentContextStore
    CreateAgentContext / GetAgentContext / FindAgentContext / UpdateAgentContext
  EventStore
    CreateEvent / ListEvents
  ProjectStore
    CreateProject / GetProject / ListProjects / UpdateProject
  BriefingStore
    CreateBriefing / GetBriefing / GetByStep
  Close
```

---

## 8. 实现状态

### 已完成 ✅

| 内容 | 位置 | 说明 |
|------|------|------|
| Flow 模型 + 状态机 | `internal/v2/core/flow.go` | FlowStatus 6 态 |
| Step 模型 + 约束字段 | `internal/v2/core/step.go` | 3 类型，8 态，required_capabilities / acceptance_criteria / timeout |
| Execution 模型 + 错误分类 | `internal/v2/core/execution.go` | 5 态，ErrorKind 三分类，BriefingSnapshot |
| Artifact 模型 | `internal/v2/core/artifact.go` | result_markdown + metadata（小模型提取）+ assets |
| Briefing 模型 | `internal/v2/core/briefing.go` | objective + context_refs + constraints |
| AgentProfile + 能力匹配 | `internal/v2/core/agent.go` | 4 角色，11 Action，能力 tag 匹配 |
| AgentContext 模型 | `internal/v2/core/agent_context.go` | |
| Event + EventBus 接口 | `internal/v2/core/event.go` | 16 种 EventType |
| Store 接口 | `internal/v2/core/store.go` | 7 个子接口（含 ArtifactStore / BriefingStore） |
| 领域错误 | `internal/v2/core/errors.go` | 9 个错误类型 |
| SQLite Store 全套 CRUD | `internal/v2/store/sqlite/` | 含 Artifact / Briefing Store，9 个测试通过 |
| DAG 验证 + 入口推导 | `internal/v2/engine/dag.go` | Kahn 无环检测 |
| 状态转移校验 | `internal/v2/engine/transition.go` | Flow/Step/Exec 三套 |
| 调度循环（promote → dispatch） | `internal/v2/engine/engine.go` | 两阶段调度 |
| 并发控制 Semaphore | `internal/v2/engine/scheduler.go` | |
| Gate 自动化（读 artifact.verdict） | `internal/v2/engine/gate.go` | reject → 上游 reset + gate pending |
| Composite 展开 + SubFlowID 持久化 | `internal/v2/engine/composite.go` | 创建子 Flow |
| EventBus 内存实现 | `internal/v2/engine/bus.go` | channel fan-out |
| 三阶段管道 (prepare→execute→finalize) | `internal/v2/engine/pipeline.go` | Resolver / BriefingBuilder / Collector 接口 |
| Step 超时 | `internal/v2/engine/engine.go` | context.WithTimeout 包裹执行 |
| 错误三分类 | `internal/v2/engine/pipeline.go` | permanent 跳过重试 / need_help 阻塞 / transient 重试 |
| 引擎测试 | `internal/v2/engine/*_test.go` | 19 个测试通过 |

共 28 个测试全绿。

### 未完成 ❌

#### P0 — 接口实现

| 内容 | 说明 |
|------|------|
| Resolver 实现 | 根据 AgentProfile 匹配 step.agent_role + required_capabilities |
| BriefingBuilder 实现 | 读上游 Artifact → 读 Project 摘要 → 组装 Briefing |
| Collector 实现 | 调小模型 + tool_use 按 StepType 提取 metadata |

#### P1 — 流转完善

| 内容 | 说明 |
|------|------|
| Composite 递归 Run | scheduleLoop 自动检测 composite → expand → 递归 run 子 Flow |
| Cancel 传播 | Cancel flow → cancel 所有 running execution 的 context |
| Flow blocked 处理 | 全部 step blocked → flow blocked，支持恢复 |
| 上游 Artifact → 下游 Briefing 数据流 | 引擎自动注入 |

#### P2 — 顶层模型 + 记忆

| 内容 | 说明 |
|------|------|
| Project 模型 + Store | CRUD |
| Flow 增加 project_id | 关联 Project |
| Briefing Store | CRUD |
| 三层记忆实现 | 冷/温/热 Recall + prefix cache 友好排列 |
| Memory Compact | 超阈值压缩历史为摘要 |
| Decision 追溯 | Execution 记录 briefing_snapshot + model 信息 |

#### P3 — API + 前端

| 内容 | 说明 |
|------|------|
| REST API | Project / Flow / Step / Execution / Artifact CRUD |
| WebSocket 事件推送 | Event 实时广播 |
| 进度视图 | Project 级 / Flow 级 / Step 级三层 |
| ACP Agent 对接 | StepHandler 实现 → ACP 协议调用 |

#### P4 — 远期

| 内容 | 说明 |
|------|------|
| Schedule 定时任务 | cron 触发 |
| Agent 动态创建 | 运行时创建新 agent |
| Pattern 模式归纳 | 从成功经验中提炼可复用模板 |
| 授权衰减 | 信任积累后减少人工审批 |
| PostgreSQL 迁移 | 多实例部署 |

---

## 9. 目录结构

```
internal/v2/
├── core/                   领域模型 + 接口
│   ├── flow.go
│   ├── step.go
│   ├── execution.go
│   ├── artifact.go
│   ├── briefing.go
│   ├── agent_context.go
│   ├── agent.go
│   ├── project.go          ← P2 待建
│   ├── event.go
│   ├── store.go
│   └── errors.go
├── store/sqlite/           SQLite 持久化
│   ├── store.go
│   ├── migrations.go
│   ├── flow.go
│   ├── step.go
│   ├── execution.go
│   ├── artifact.go
│   ├── briefing.go
│   ├── agent_context.go
│   ├── event.go
│   ├── json.go
│   └── store_test.go
├── engine/                 Flow 引擎
│   ├── engine.go
│   ├── dag.go
│   ├── scheduler.go
│   ├── transition.go
│   ├── gate.go
│   ├── composite.go
│   ├── bus.go
│   ├── pipeline.go            三阶段管道 + Resolver/BriefingBuilder/Collector 接口
│   └── *_test.go
└── api/                    ← P3 待建
    ├── handler.go
    └── routes.go
```
