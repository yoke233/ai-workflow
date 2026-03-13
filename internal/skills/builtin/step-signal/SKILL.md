---
name: step-signal
description: Signal task completion or gate decisions to the AI Workflow engine
assign_when: Automatically injected for exec and gate steps
version: 3
---

# Step Signal

You are running inside an **ai-workflow** managed step.
You **must** signal your result before ending your response.

## Your Context

| Variable | Value |
|---|---|
| Step Type | `$AI_WORKFLOW_STEP_TYPE` |
| Step ID | `$AI_WORKFLOW_STEP_ID` |
| Issue ID | `$AI_WORKFLOW_ISSUE_ID` |
| Execution ID | `$AI_WORKFLOW_EXEC_ID` |

## How to Signal

### Option A: Run the script (preferred)

```bash
bash ./scripts/signal.sh <decision> "<reason>"
```

The script handles networking automatically — it tries HTTP first, falls back to output if unavailable.

### Option B: Output fallback (if the script is not available)

Print this line in your response:

```
AI_WORKFLOW_SIGNAL: {"decision":"<decision>","reason":"<reason>"}
```

## Decisions

| Step Type | Decision | Meaning |
|---|---|---|
| `exec` | `complete` | Task finished successfully |
| `exec` | `need_help` | Stuck, need human assistance |
| `gate` | `approve` | Code review passes |
| `gate` | `reject` | Code review fails, needs rework |

## Examples

```bash
# Exec step — completed
bash ./scripts/signal.sh complete "implemented auth module with tests"

# Exec step — stuck
bash ./scripts/signal.sh need_help "cannot resolve dependency conflict"

# Gate step — approve
bash ./scripts/signal.sh approve "all acceptance criteria met"

# Gate step — reject
bash ./scripts/signal.sh reject "missing error handling in payment flow"
```

## Rules

1. **Always signal before ending your response.** The engine cannot proceed without it.
2. Signal **once** per execution — do not call multiple times.
