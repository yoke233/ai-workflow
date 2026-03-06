# E2E 测试指南

## 概述

本文档描述如何对 ai-workflow 进行端到端测试，覆盖从 A2A 消息 → 编码 → Review → 合并的完整流程。

## 前置条件

### 1. 准备目标仓库

需要一个真实的 Git 仓库作为测试目标。可以用任意简单项目：

```bash
mkdir /tmp/test-repo && cd /tmp/test-repo
git init && echo "package main" > main.go && git add -A && git commit -m "init"
```

### 2. Codex ACP 认证

codex-acp 通过 `$CODEX_HOME/auth.json` 认证（不是环境变量）：

```bash
# 确认 ~/.codex/auth.json 存在
cat ~/.codex/auth.json
# 格式: {"OPENAI_API_KEY": "sk-xxx"}

# 项目配置里 CODEX_HOME 指向的目录也要有 auth.json
cp ~/.codex/auth.json .ai-workflow/codex-home/auth.json
```

### 3. Codex Provider 配置

`.ai-workflow/codex-home/config.toml`:
```toml
[model]
model_id = "codex-mini"

[model.provider.rightcode]
name = "rightcode"
base_url = "https://right.codes/codex/v1"
requires_openai_auth = true
wire_format = "openai"

[agent]
approval_policy = "never"
sandbox_mode = "danger-full-access"
```

## 测试方式

### 方式一：A2A 协议全流程

最接近真实场景，经过 Team Leader → Issue → Run 完整链路。

```bash
# 1. 初始化项目配置
cd <test-repo>
ai-flow config init

# 2. 注册项目
ai-flow project add test-proj <test-repo-path>

# 3. 启动 server
ai-flow server --port 8080

# 4. 发送需求
curl -X POST http://127.0.0.1:8080/api/v1/a2a \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test-token-123" \
  -d '{
    "jsonrpc": "2.0",
    "id": "1",
    "method": "message/send",
    "params": {
      "message": {
        "role": "user",
        "parts": [{"type": "text", "text": "添加一个 /health HTTP endpoint，返回 JSON {status: ok}，并写单元测试"}]
      },
      "metadata": {"project_id": "test-proj"}
    }
  }'

# 5. 查看进度
# 方式 A: 看 server 日志 (stdout)
# 方式 B: 查 stage-summary API
curl http://127.0.0.1:8080/api/v3/runs/<run-id>/stage-summary

# 方式 C: 查全部事件
curl http://127.0.0.1:8080/api/v3/runs/<run-id>/events
```

### 方式二：acp-probe 单步测试

直接跟 ACP agent 交互，绕过 server/scheduler，适合测试 ACP 协议层或 prompt 效果。

```bash
# 构建 acp-probe 工具
go build ./cmd/acp-probe/

# 修改 cmd/acp-probe/main.go 里的 prompt，然后运行
./acp-probe
# 输出每一条 session/update 的完整 JSON + 类型统计
```

acp-probe 会打印所有 ACP 事件类型及其原始 JSON，用于：
- 确认 ACP 协议格式
- 观察 agent 行为（思考→工具调用→回复）
- 调试认证/连接问题

### 方式三：Go 单元测试 (mock ACP)

`internal/engine/` 里的测试用 `testStageFunc` 替换真实 ACP 调用：

```go
e.TestSetStageFunc(func(ctx context.Context, runID string, stage core.StageID, agent, prompt string) error {
    // 模拟 stage 行为
    return nil
})
```

适合测试 executor 逻辑、状态机、重试策略等，不依赖外部 agent。

Bridge flush 逻辑的测试在 `executor_acp_test.go`，用 `collectBus` mock event bus。

## Pipeline 阶段

standard 模板: `setup → requirements → implement → review → fixup → merge → cleanup`

每个 stage 在 `run_events` 表中产生的事件类型：

| 事件 type | 说明 | 来源 |
|---|---|---|
| `stage_start` | stage 开始 | executor |
| `prompt` | 发给 agent 的完整 prompt | executeStage |
| `agent_thought` | 完整思考内容（chunk 搜集） | bridge flush |
| `tool_call` | 工具调用（含 title） | bridge |
| `tool_call_completed` | 工具结果（含 exit_code, stdout） | bridge |
| `usage_update` | token 用量 (size, used) | bridge |
| `agent_message` | 完整回复内容（chunk 搜集） | bridge flush |
| `done` | stage 最终结果 | promptACPSession |
| `stage_complete` / `stage_failed` | stage 结束 | executor |

## 排查方法

```sql
-- 查看某次 run 的完整事件时间线
SELECT id, event_type, stage, agent,
       substr(data_json, 1, 120) as data_preview,
       error, created_at
FROM run_events
WHERE run_id = '<run-id>'
ORDER BY id;

-- 查看某个 stage 的 prompt
SELECT data_json FROM run_events
WHERE run_id = '<run-id>' AND stage = 'implement'
  AND event_type = 'agent_output'
  AND data_json LIKE '%"type":"prompt"%';

-- 查看工具调用记录
SELECT data_json FROM run_events
WHERE run_id = '<run-id>'
  AND event_type = 'agent_output'
  AND (data_json LIKE '%"type":"tool_call"%'
    OR data_json LIKE '%"type":"tool_call_completed"%');

-- Stage 耗时和 token 统计 (等效于 /stage-summary API)
SELECT stage,
       COUNT(*) as event_count,
       MIN(created_at) as first_activity,
       MAX(created_at) as last_activity
FROM run_events
WHERE run_id = '<run-id>'
GROUP BY stage;
```

## 已知问题与 workaround

1. **Scheduler 信号量泄漏**: run 失败/取消后新 issue 卡在 ready → 重启 server
2. **Windows cleanup Permission Denied**: codex-acp 进程持有文件句柄 → 手动删除 .worktrees 目录
3. **Cleanup 失败导致 issue 卡 action_required**: merge 已成功但 cleanup 失败 → 手动标 done
4. **A2A 每条消息都创建 issue**: 当前无意图判断 → 只发真正的需求消息

## 合并冲突测试方案

模拟步骤：
1. 启动一个 run，在 implement 阶段 codex 修改了 `main.go`
2. 在 implement 执行期间，手动在主分支 commit 一个对 `main.go` 同一行的修改
3. merge 阶段 `git merge` 会报冲突
4. 验证：run 进入 action_required 状态，事件里有 merge_conflict 信息

自动化方法（用 testStageFunc）：
```go
// 在 implement stage mock 里，修改 worktree 里的文件并 commit
// 然后在主分支也修改同一个文件并 commit
// merge stage 会自然遇到冲突
```
