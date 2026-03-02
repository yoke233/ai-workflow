---
name: writing-wave-plans
description: Use when you have specs or requirements for a large multi-step feature and need a multi-wave implementation plan with strict per-wave quality gates before moving forward.
---

# Writing Wave Plans

## Overview
Write comprehensive multi-wave implementation plans for engineers with near-zero project context. The output must be executable, traceable, and gated wave-by-wave.

Assume the implementer is technically strong but unfamiliar with local architecture and weak at test design.

Use DRY, YAGNI, TDD, and frequent commits.

**Announce at start:** "I'm using the writing-wave-plans skill to create the implementation plan."

**Save plans to:**
- Main plan: `docs/plans/YYYY-MM-DD-<topic>-implementation.md`
- Wave plans: `docs/plans/YYYY-MM-DD-<topic>-wave<N>.md`

## Hard Gate
Use the `executing-wave-plans` skill as the default execution contract for wave gates.

In plan files, avoid restating the full generic gate sequence repeatedly. Keep only wave-specific acceptance and verification requirements that execution cannot infer.

## When to Use
- User asks to split a complex delivery into phases/waves.
- Work includes cross-module changes with dependencies.
- You need handoff-ready plans, not a rough idea list.
- You need explicit quality gates between waves.

Do not use for:
- Single small fix with no staged rollout.
- Reviewing existing plan quality only.

## Required Inputs
Collect before writing:
1. Goal and non-goals
2. Source specs/docs
3. Current codebase constraints and existing capabilities
4. Required test gates and acceptance criteria
5. Available parallelism (team/execution capacity)

If inputs are incomplete, add an `Assumptions` section explicitly.

## Plan Document Header
Every generated main plan MUST start with:

```markdown
# [Topic] Implementation Plan

> **For Agent:** REQUIRED SUB-SKILL: Use executing-wave-plans to implement this plan wave-by-wave.

**Goal:** [One sentence]

**Architecture:** [2-3 sentences]

**Tech Stack:** [Key libraries/systems]

---
```

## Main Plan Structure
The main plan must include:
1. Context and scope (in-scope / out-of-scope)
2. Dependency DAG overview and critical path
3. Wave map (`Wave 1..N`) with explicit `depends_on`
4. Global quality gates (F/Q/C/D if applicable)
5. Per-wave output file links
6. Full regression command set
7. Test policy: unit-first TDD in tasks + mandatory e2e/smoke per wave + boundary-triggered integration/contract tests

## Wave Plan Structure
Each wave file must include:
1. Wave goal
2. Tasks with IDs and dependencies
3. Test strategy per task
4. Risks and mitigations
5. Wave-specific Exit Gate and Next Wave Entry Condition
6. Wave e2e/smoke cases and entry data

Use this gate template in every wave:

```markdown
## Wave Exit Gate
- Execute mandatory gate sequence via `executing-wave-plans`.
- Wave-specific acceptance:
  - [ ] [criterion 1]
  - [ ] [criterion 2]
- Wave-specific verification:
  - [ ] [wave e2e/smoke command + expected signal]
  - [ ] [command + expected signal]
- Boundary-change verification (if triggered):
  - [ ] [integration/contract command + expected signal]

## Next Wave Entry Condition
- Governed by `executing-wave-plans` verdict rule (`Go` / satisfied `Conditional Go` only).
```

## Task Granularity
For implementation tasks, keep steps bite-sized (2-5 minutes each):
1. Write failing test
2. Run and confirm failure
3. Write minimal implementation
4. Run tests and confirm pass
5. Commit

## Task Template
Use this structure for each task in wave files:

````markdown
### Task WN-TK: [Short Title]

**Files:**
- Create: `exact/path/new_file.ext`
- Modify: `exact/path/existing.ext`
- Test: `exact/path/test_file.ext`

**Depends on:** `[WN-TA, WN-TB]` or `[]`

**Step 1: Write failing test**
```text
[Concrete test case]
```

**Step 2: Run to confirm failure**
Run: `[exact command]`
Expected: `[expected failure signal]`

**Step 3: Minimal implementation**
```text
[minimal code or precise change instruction]
```

**Step 4: Run tests to confirm pass**
Run: `[exact command]`
Expected: `[expected pass signal]`

**Step 5: Commit**
```bash
git add [exact files]
git commit -m "[conventional commit message]"
```
````

## Quick Commands
```powershell
rg -n 'spec|需求|scope|范围|Wave|depends_on|Gate|验收|commit|review|修复|回归' docs
rg -n 'BuildFromConfig|newDefaultRegistry|config|server|scheduler|cli' internal cmd
rg -n 'go test|npm test|typecheck|race' docs/plans
git log --oneline -n 30
```

## Execution Handoff
After writing and saving the plan, offer:

1. Current Session: execute wave-by-wave in this session using `executing-wave-plans`.
2. Parallel Session (separate): execute using `executing-wave-plans` in a new session.

## Done Criteria
- Main plan + wave files are generated and linked.
- Every wave references `executing-wave-plans` gate and includes wave-specific acceptance/verification.
- Dependencies are explicit and enforce wave-by-wave progression.
- Plan can be executed by a zero-context engineer without hidden assumptions.
