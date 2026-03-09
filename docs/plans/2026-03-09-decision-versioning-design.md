# Decision 版本化设计

日期: 2026-03-09
状态: draft

## 1. 目标

为系统中每个 AI 决策建立结构化记录，包含 prompt/model/reasoning/output，实现决策可追溯。当 Agent 行为异常时，能够回溯到具体的 prompt 和模型版本。

### 设计原则

- **利用已有基础设施** — TaskStep.Input/Output 已有但未使用，优先填充而非新建表
- **渐进式** — 先覆盖关键决策点，不追求一步到位
- **不影响性能** — 决策记录是写入的副作用，不增加 LLM 调用

## 2. 现状分析

### 系统中的 5 个决策点

| 决策点 | 位置 | 当前记录 | 缺失 |
|--------|------|---------|------|
| 审查决策 | review.go `decideSession()` | ReviewRecord (verdict/score) | prompt、model |
| 分解决策 | decompose_handler.go | TaskStep (note only) | Input/Output 为空 |
| Stage 执行 | executor_stages.go | run_events (prompt/done) | model、template 版本 |
| Chat 助手 | handlers_chat.go | 无 | 全部缺失 |
| 权限决策 | acp_handler.go | 无 | 策略选择记录 |

### TaskStep 已有但未使用的字段

```go
type TaskStep struct {
    Input   string  // ← 空，设计用于存储输入摘要
    Output  string  // ← 空，设计用于存储输出摘要
    RefID   string  // ← 部分使用（review_record_id）
    RefType string  // ← 部分使用（"review_record"）
}
```

## 3. 方案选择

### 方案 A：新建 decisions 表

独立的 Decision 模型，完整存储所有决策字段。

- 优点：结构清晰，查询灵活
- 缺点：新增表 + migration + Store 接口 + 大量改造

### 方案 B：扩展 TaskStep（推荐）

利用 TaskStep 已有的 Input/Output/RefID 字段，新增少量字段。

- 优点：改动最小，与事件溯源统一
- 缺点：Input/Output 是 JSON 字符串，不如独立表灵活

### 方案 C：扩展 TaskStep + 独立 decision_meta 表

TaskStep 记录核心决策事实，decision_meta 存储 prompt 快照等大体积数据。

- 优点：兼顾简洁和完整
- 缺点：多一张表，复杂度介于 A 和 B 之间

**选择方案 B**，理由：
1. TaskStep.Input/Output 本身就是为此设计的
2. 初期决策记录的主要用途是追溯，不需要复杂查询
3. v3 设计中 TaskStep.decision_ref 是可选的，证明 TaskStep 本身就能承载决策信息
4. 需要时可以在方案 B 基础上升级到方案 C

## 4. 数据结构

### 4.1 TaskStep 字段使用约定

不新增字段，充分利用已有字段：

```go
// 决策类 TaskStep 的字段使用：
TaskStep{
    Action:  StepReviewApproved,     // 决策动作
    AgentID: "reviewer_code",        // 决策执行者
    Input:   `{"prompt_hash":"...","model":"claude-sonnet-4-20250514","template":"review"}`,
    Output:  `{"decision":"approve","score":85,"reasoning":"代码质量符合标准"}`,
    Note:    "review approved with score 85",  // 人类可读摘要
    RefID:   "review-record-123",    // 关联详细记录
    RefType: "review_record",        // 关联类型
}
```

### 4.2 Input JSON Schema

```json
{
  "prompt_hash": "sha256:abcd1234",
  "model": "claude-sonnet-4-20250514",
  "template": "review",
  "template_version": "v2.1",
  "token_count": 4500
}
```

不存完整 prompt（太大），存 hash。完整 prompt 在 run_events 中已有（type=prompt 的事件）。通过 prompt_hash 可以关联。

### 4.3 Output JSON Schema

```json
{
  "decision": "approve",
  "reasoning": "代码结构清晰，测试覆盖率 85%",
  "score": 85,
  "confidence": 0.9,
  "issues_count": 0,
  "duration_ms": 3200
}
```

### 4.4 各决策点的 Input/Output 规范

#### 审查决策

```go
// review.go — runPhase1 完成后
s.recordTaskStep(issue, StepReviewApproved, reviewerName, note)
// Input: {"prompt_hash":"...","model":"...","template":"review"}
// Output: {"decision":"approve","score":85,"reasoning":"..."}
```

#### 分解决策

```go
// decompose_handler.go — 分解完成后
s.recordTaskStep(issue, StepDecomposed, "team_leader", note)
// Input: {"prompt_hash":"...","model":"...","template":"decompose","user_prompt":"帮我做注册系统"}
// Output: {"decision":"decompose","children_count":5,"summary":"用户注册系统拆为5个子任务"}
```

#### Stage 执行

Stage 执行通过 run_events 已有 prompt/done 事件，不重复在 TaskStep 中记录。但 `stage_completed` 的 TaskStep 可以记录：

```go
// executor.go — stage 完成后
recordTaskStep(issue, StepStageCompleted, agentName, note)
// Input: {"model":"...","template":"implement","stage":"implement"}
// Output: {"files_changed":3,"tests_passed":true,"duration_ms":45000}
```

## 5. 实现路径

### 5.1 辅助函数

```go
// internal/core/decision.go

// DecisionInput 构造决策输入元数据
type DecisionInput struct {
    PromptHash      string `json:"prompt_hash,omitempty"`
    Model           string `json:"model,omitempty"`
    Template        string `json:"template,omitempty"`
    TemplateVersion string `json:"template_version,omitempty"`
    TokenCount      int    `json:"token_count,omitempty"`
    Extra           map[string]string `json:"extra,omitempty"`
}

func (d DecisionInput) JSON() string { ... }

// DecisionOutput 构造决策输出元数据
type DecisionOutput struct {
    Decision    string `json:"decision"`
    Reasoning   string `json:"reasoning,omitempty"`
    Score       *int   `json:"score,omitempty"`
    Confidence  float64 `json:"confidence,omitempty"`
    DurationMs  int64  `json:"duration_ms,omitempty"`
    Extra       map[string]string `json:"extra,omitempty"`
}

func (d DecisionOutput) JSON() string { ... }

// PromptHash 计算 prompt 的 SHA256 前缀
func PromptHash(prompt string) string {
    h := sha256.Sum256([]byte(prompt))
    return "sha256:" + hex.EncodeToString(h[:8])
}
```

### 5.2 改造 recordTaskStep

现有 `recordTaskStep` 签名：

```go
func (s *DepScheduler) recordTaskStep(issue *core.Issue, action core.TaskStepAction, agentID, note string)
```

新增带 Decision 的版本：

```go
func (s *DepScheduler) recordTaskStepWithDecision(
    issue *core.Issue,
    action core.TaskStepAction,
    agentID, note string,
    input, output string,
) {
    // 和 recordTaskStep 相同，但填充 Input/Output
}
```

### 5.3 改造调用方

| 调用方 | 改造内容 |
|--------|---------|
| `review.go` `runReviewSession()` | 审查完成时填充 Input/Output |
| `decompose_handler.go` `OnEvent()` | 分解完成时填充 Input/Output |
| `executor.go` stage 完成 | stage_completed 时填充 Input/Output |

## 6. 前端展示

IssueFlowTree 已有，TaskStep.Input/Output 可在展开详情时显示：

```
▼ ✅ reviewing                                    10:05
    ├── 模型: claude-sonnet-4-20250514
    ├── 决策: approve (score: 85)
    └── 推理: "代码结构清晰，测试覆盖率 85%"
```

前端改动：IssueFlowTree 组件解析 `step.input`/`step.output` JSON，渲染为可读格式。

## 7. 查询与追溯

无需新增 API，现有 Timeline API 已返回 TaskStep 全量数据：

```
GET /api/v3/projects/{projectId}/issues/{issueId}/timeline
```

返回的 TaskStep 中 Input/Output 不再为空，前端自然可以展示。

未来如需专门查询决策：可以在 SQLite 中用 `json_extract` 查询，或新增索引。

## 8. 改造范围

### 新增

- `internal/core/decision.go` — DecisionInput/Output 辅助类型 + PromptHash

### 改造

- `internal/teamleader/scheduler_dispatch.go` — `recordTaskStepWithDecision()` 辅助函数
- `internal/teamleader/review.go` — 审查决策记录 Input/Output
- `internal/teamleader/decompose_handler.go` — 分解决策记录 Input/Output
- `internal/engine/executor.go` — Stage 完成记录 Input/Output
- `web/src/components/IssueFlowTree.tsx` — 解析并展示 Decision 详情

### 不变

- TaskStep 模型和数据库 schema（字段已有）
- Store 接口（SaveTaskStep 签名不变）
- run_events 表
- review_records 表
- Timeline API

## 9. 未来演进

- **方案 C 升级**：如果 Input/Output JSON 字符串查询成为瓶颈，抽出 `decision_meta` 表
- **retrieval_trace**：记录"AI 看到了什么上下文"（v3 OpenViking 设计中提到）
- **DecisionValidator**：对 Decision 做硬规则校验，防止 AI 做出不合理决策
