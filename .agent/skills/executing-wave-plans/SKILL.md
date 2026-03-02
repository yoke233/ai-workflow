---
name: executing-wave-plans
description: Use when executing a multi-wave implementation plan where wave boundaries are strict release gates and next-wave work must wait for explicit acceptance.
---

# Executing Wave Plans

## Overview
Execute plan work one wave at a time with hard gates between waves. This skill owns the default wave governance so plan docs can stay focused on scope, dependencies, and task-level implementation.

**Core principle:** No cross-wave execution. No next wave before current wave is explicitly accepted.

**Announce at start:** "I'm using the executing-wave-plans skill to implement this plan wave-by-wave."

## What This Skill Owns (Can Move Out of Plan)
- Standard wave exit sequence: commit -> review -> fix -> post-fix verification -> verdict
- Default severity policy: High/Medium findings must be fixed or explicitly waived
- Wave transition rule: only `Go` or satisfied `Conditional Go` can enter next wave
- Evidence minimum for each wave (commit, review notes, fix mapping, verification output, verdict)
- Stop conditions and escalation behavior when blocked
- Default TDD development discipline
- Default in-wave parallel execution strategy

## What Must Stay In Plan
- Wave map, task IDs, and dependency DAG
- Exact files and implementation steps per task
- Wave-specific verification commands
- Wave-specific acceptance criteria and business/architecture constraints

## TDD Baseline (Mandatory)
For every implementation task:
1. Write a failing test first (RED).
2. Confirm failure is expected and meaningful.
3. Write minimal code to pass (GREEN).
4. Re-run tests and keep them green during refactor.

If code is written before a failing test, delete that change and restart with TDD.

## Test Policy (Unit-in-Wave + E2E-per-Wave)
Default policy:
- unit tests: mandatory at task level (first RED for each behavior change)
- e2e/smoke: mandatory at wave level (at least one critical flow per wave gate)

Boundary-change rule:
- if a wave changes API/schema/event contract/plugin boundary/DB migration, add focused integration or contract tests in the same wave

Execution rule:
1. Task loop uses unit-first TDD (`RED -> GREEN -> REFACTOR`).
2. During development, run fast unit subset frequently.
3. Before wave gate, run full wave verification including e2e/smoke.
4. If boundary-change rule is triggered, run corresponding integration/contract tests before verdict.

## Parallel Strategy (Default + Optional)
Default mode: sequential tasks within current wave.

Optional in-wave parallel mode is allowed only when all conditions hold:
- tasks are dependency-independent
- no shared critical files
- each lane has isolated branch/worktree
- each lane still follows TDD and review policy

Cross-wave parallelism is forbidden.

## Process

### Step 1: Preflight
1. Read the plan and identify all waves in order.
2. Confirm branch/worktree safety before implementation.
3. Raise plan ambiguity before writing code.

### Step 2: Execute Current Wave
1. Execute only tasks in the current wave.
2. Track task status (`pending` -> `in_progress` -> `completed`).
3. Enforce unit-first TDD per task and wave-level e2e verification.
4. Do not start tasks from later waves, even if they look unblocked.

### Step 3: Run Wave Exit Gate (Mandatory)
1. **Code committed:** at least one traceable commit/PR linked to wave scope
2. **Review completed:** explicit verdict and findings list
3. **Findings fixed:** High/Medium cleared or documented waiver approved
4. **Post-fix verification passed:** run required commands and record outputs
5. **Wave verdict recorded:** `Go` / `Conditional Go` / `No-Go`

### Step 4: Decide Transition
- `Go`: proceed to next wave
- `Conditional Go`: proceed only after listed preconditions are complete
- `No-Go`: stop and ask for direction

### Step 5: Report and Wait
After each wave gate:
- Report implemented tasks
- Report verification evidence
- Report verdict and next-wave readiness
- Say: "Ready for feedback."

## Wave Evidence Template
Use this block for every wave:

```markdown
## Wave N Evidence
- Commit/PR: [hash or link]
- Review: [verdict + findings summary]
- Fixes: [finding -> commit mapping]
- Verification:
  - [command]
  - [result summary]
- Verdict: Go / Conditional Go / No-Go
- Conditional Preconditions (if any): [...]
```

## When to Stop and Ask for Help
Stop immediately when:
- plan instruction is unclear or conflicting
- required dependencies/tools are missing
- verification fails repeatedly without clear root cause
- review returns blocking findings you cannot resolve safely

## Integration
**Required workflow skills:**
- **superpowers:using-git-worktrees** - set up isolated workspace before execution
- **superpowers:finishing-a-development-branch** - finish branch flow after final wave is accepted
