# RV4 Shared Thread Second-Evidence Validation Implementation Plan

> **For Agent:** REQUIRED SUB-SKILL: Use executing-wave-plans to implement this plan wave-by-wave.

**Goal:** 通过第二条真实 HTTP API 场景，验证 shared thread 在多 Task 正式闭环下的 query / aggregate / timeline distortion 是否稳定复现，并据此决定是否进入 P3-D entry review。

**Architecture:** 沿用当前正式主链 `Task -> Assignment -> Execution -> Artifact -> Review -> Decision -> done`，不修改正式状态机，也不先做 Thread ↔ Task 多对多重构。RV4 只新增一条真实验证场景和对应证据包，重点观察 shared thread 被多个 Task 复用时，query 侧是否继续出现单边吸附、上下文缺失或 attribution 失真。

**Tech Stack:** Go, `internal/app/api`, `internal/app/query`, `internal/app/task`, `internal/app/thread`, HTTP API integration tests, docs evidence package

**Date:** 2026-03-07
**Status:** Active

---

## Context

RV3 已经证明两件事：

1. 正式闭环没有串线  
   - 两个 Task 都能独立完成 `assignment -> execution -> artifact -> review -> decision -> done`
2. shared thread 的 query 语义开始失真  
   - 一个 Task aggregate 吸进过多 shared-thread message
   - 另一个 Task aggregate / timeline 缺失 shared-thread 上下文

RV3 当前只提供了第一份强证据，因此还不建议直接进入 P3-D implementation。  
RV4 的目标不是修复，而是补第二份真实证据，确认这是不是稳定结构性问题。

## In Scope

- 设计并跑通第二条真实 shared-thread 场景
- 复用同一协作 thread，挂接两个正式 Task
- 两个 Task 都跑完整正式闭环
- 固定输出 aggregate / timeline / raw message 三层证据
- 形成 RV4 implementation note + blocker review
- 给出是否进入 P3-D entry review 的 stop/go 结论

## Out of Scope

- 不修改 Thread ↔ Task 模型
- 不实现 P3-D
- 不引入多对多 link 表
- 不调整正式状态机
- 不引入 WebSocket / external bridge / runtime connector 新能力
- 不做 UI 修复

## Assumptions

- 当前系统仍允许通过 metadata 或现有挂接方式表达 shared-thread 下的 task attribution
- RV3 的 shared-thread 能力和测试构造方式仍可复用
- 正式闭环相关 API 已稳定可用

## Dependency DAG Overview

`Wave 1 (scenario + failing evidence expectations)`  
-> `Wave 2 (execute scenario + collect evidence)`  
-> `Wave 3 (blocker review + P3-D entry verdict)`

## Critical Path

`W1 -> W2 -> W3`

## Wave Map

- **Wave 1: Define second shared-thread scenario**
  - depends_on = []
- **Wave 2: Execute RV4 and capture evidence**
  - depends_on = [W1]
- **Wave 3: Review evidence and decide P3-D entry**
  - depends_on = [W2]

## Global Quality Gates

- 不新增正式领域对象
- 不修改正式闭环状态机
- 必须通过真实 HTTP API 路径，不允许只走 service bypass
- 必须同时检查：
  - raw message attribution
  - task aggregate projection
  - task timeline projection
- 结论必须落成文档，不能只留在测试断言里

## Output Files

- Main plan:
  - `docs/plans/2026-03-07-rv4-shared-thread-second-evidence-validation.md`
- Evidence note:
  - `docs/implementation-notes/0033-rv4-shared-thread-second-evidence-validation.md`
- Stop/go review:
  - `docs/discussions/post-rv4-shared-thread-second-evidence-review.md`
- Test:
  - `internal/app/api/real_use_shared_thread_second_evidence_test.go`

## Regression Commands

```powershell
go test -count=1 -run TestRealUseSharedThreadSecondEvidenceViaHTTP -v ./internal/app/api
go test -count=1 ./internal/app/api ./internal/app/query ./internal/app/task ./internal/app/thread
go test -count=1 ./...
```

## Test Policy

- 先写真实 HTTP 集成测试，再补最小实现或测试辅助
- 不为通过测试而修改正式业务语义
- 如果需要测试辅助，只能加在测试侧 helper，不得把 workaround 混入正式代码
- 每波结束必须给出证据与结论，不能只给“测试通过”

## Workspace Strategy

- 默认使用一个 plan-level branch / worktree
- 三个 wave 共用同一上下文
- 如需并行，只允许在 Wave 内做临时测试辅助分支，Wave 结束前必须合回主执行分支

---

# Wave 1: Define Second Shared-Thread Scenario

## Wave Goal

设计一个比 RV3 更强的 shared-thread 真实场景，使 distortion 是否复现能被稳定观察，而不是偶发个例。

### Task W1-T1: Define scenario shape

**Files:**
- Create: `internal/app/api/real_use_shared_thread_second_evidence_test.go`

**Depends on:** `[]`

**Step 1: Write failing test**
```text
定义一个真实场景：
- 创建 shared thread
- 创建 Task A / Task B
- 两个 Task 都挂接同一个 shared thread
- shared thread 中穿插至少两轮 cross-task coordination message
- Task A / Task B 各自产生 assignment / execution / artifact / review / decision / done
- 最后分别拉取 Task A / Task B 的 aggregate / timeline / raw message evidence
- 断言：
  1. 正式闭环全部成功
  2. shared-thread message 在两边 aggregate / timeline 的可见性行为被稳定记录
  3. 如果出现同型 distortion，测试不一定失败，但必须把证据稳定写入 note 所需结构
```

**Step 2: Run to confirm failure**
Run: `go test -count=1 -run TestRealUseSharedThreadSecondEvidenceViaHTTP -v ./internal/app/api`  
Expected: 测试不存在或场景尚未实现，失败

**Step 3: Minimal implementation**
```text
先在测试里搭建场景，不改正式业务代码。
优先复用 RV3 的 helper / HTTP 路径 / assertion style。
```

**Step 4: Run tests to confirm pass**
Run: `go test -count=1 -run TestRealUseSharedThreadSecondEvidenceViaHTTP -v ./internal/app/api`  
Expected: 场景能跑到证据采集点

**Step 5: Commit**
```bash
git add internal/app/api/real_use_shared_thread_second_evidence_test.go
git commit -m "test(api): add rv4 shared thread second evidence scenario"
```

### Task W1-T2: Define evidence contract

**Files:**
- Modify: `internal/app/api/real_use_shared_thread_second_evidence_test.go`

**Depends on:** `[W1-T1]`

**Step 1: Write failing test**
```text
明确采集并对比以下证据：
- Task A aggregate
- Task B aggregate
- Task A timeline
- Task B timeline
- shared thread raw messages
- raw messages 中的 metadata.task_id / sender / occurred_at / message ids
```

**Step 2: Run to confirm failure**
Run: `go test -count=1 -run TestRealUseSharedThreadSecondEvidenceViaHTTP -v ./internal/app/api`  
Expected: 缺少完整证据采集或断言

**Step 3: Minimal implementation**
```text
在测试里增加 evidence snapshot 结构，保证后续 implementation note 可直接引用。
```

**Step 4: Run tests to confirm pass**
Run: `go test -count=1 -run TestRealUseSharedThreadSecondEvidenceViaHTTP -v ./internal/app/api`  
Expected: 测试可稳定输出完整证据

**Step 5: Commit**
```bash
git add internal/app/api/real_use_shared_thread_second_evidence_test.go
git commit -m "test(api): capture rv4 shared-thread evidence snapshots"
```

## Wave Exit Gate
- Execute mandatory gate sequence via `executing-wave-plans`.
- Wave-specific acceptance:
  - [ ] 第二条 shared-thread 场景已定义完成
  - [ ] 证据采集结构已固定
- Wave-specific verification:
  - [ ] `go test -count=1 -run TestRealUseSharedThreadSecondEvidenceViaHTTP -v ./internal/app/api`
  - [ ] 能稳定输出 aggregate / timeline / raw message 三层证据
- Boundary-change verification (if triggered):
  - [ ] 无

## Next Wave Entry Condition
- Governed by `executing-wave-plans` verdict rule (`Go` / satisfied `Conditional Go` only).

---

# Wave 2: Execute RV4 and Capture Evidence

## Wave Goal

跑通第二条 shared-thread 真实场景，形成完整 evidence package，并判断 distortion 是否与 RV3 同型复现。

### Task W2-T1: Execute formal closure path

**Files:**
- Modify: `internal/app/api/real_use_shared_thread_second_evidence_test.go`

**Depends on:** `[W1-T2]`

**Step 1: Write failing test**
```text
让两个 Task 都完整走完：
- assignment accept / activate
- execution create / complete
- artifact record
- review submit
- decision finalize
- task done
并断言 formal closure 没串线
```

**Step 2: Run to confirm failure**
Run: `go test -count=1 -run TestRealUseSharedThreadSecondEvidenceViaHTTP -v ./internal/app/api`  
Expected: 闭环步骤不完整或断言不满足

**Step 3: Minimal implementation**
```text
只补测试侧驱动和必要 fixture，除非发现真实阻塞，不修改正式域模型。
```

**Step 4: Run tests to confirm pass**
Run: `go test -count=1 -run TestRealUseSharedThreadSecondEvidenceViaHTTP -v ./internal/app/api`  
Expected: 两个 Task 都成功到 done

**Step 5: Commit**
```bash
git add internal/app/api/real_use_shared_thread_second_evidence_test.go
git commit -m "test(api): validate rv4 formal closure on shared thread"
```

### Task W2-T2: Compare distortion pattern with RV3

**Files:**
- Create: `docs/implementation-notes/0033-rv4-shared-thread-second-evidence-validation.md`

**Depends on:** `[W2-T1]`

**Step 1: Write failing test**
```text
无代码测试；这里的“失败”指尚无文档化结论。
```

**Step 2: Run to confirm failure**
Run: `go test -count=1 ./internal/app/api ./internal/app/query ./internal/app/task ./internal/app/thread`  
Expected: 测试通过，但尚无文档结论

**Step 3: Minimal implementation**
```text
在 implementation note 中明确记录：
- shared thread 场景图
- Task A / Task B formal closure 结果
- aggregate 差异
- timeline 差异
- raw message attribution 可见性
- 与 RV3 的同型 / 异型判断
```

**Step 4: Run tests to confirm pass**
Run: `go test -count=1 ./internal/app/api ./internal/app/query ./internal/app/task ./internal/app/thread`  
Expected: 相关测试通过，证据文档已齐备

**Step 5: Commit**
```bash
git add docs/implementation-notes/0033-rv4-shared-thread-second-evidence-validation.md
git commit -m "docs(notes): record rv4 shared-thread second evidence"
```

## Wave Exit Gate
- Execute mandatory gate sequence via `executing-wave-plans`.
- Wave-specific acceptance:
  - [ ] 两个 Task 均完成正式闭环
  - [ ] aggregate / timeline / raw message 三层证据已落档
  - [ ] 已明确与 RV3 是否同型复现
- Wave-specific verification:
  - [ ] `go test -count=1 -run TestRealUseSharedThreadSecondEvidenceViaHTTP -v ./internal/app/api`
  - [ ] `go test -count=1 ./internal/app/api ./internal/app/query ./internal/app/task ./internal/app/thread`
  - [ ] `go test -count=1 ./...`
- Boundary-change verification (if triggered):
  - [ ] 若触及 query 边界，仅允许增加测试辅助，不允许顺手修 P3-D

## Next Wave Entry Condition
- Governed by `executing-wave-plans` verdict rule (`Go` / satisfied `Conditional Go` only).

---

# Wave 3: Review Evidence and Decide P3-D Entry

## Wave Goal

基于 RV3 + RV4 两条真实证据，决定是否满足进入 P3-D entry review 的门槛。

### Task W3-T1: Write blocker review

**Files:**
- Create: `docs/discussions/post-rv4-shared-thread-second-evidence-review.md`

**Depends on:** `[W2-T2]`

**Step 1: Write failing test**
```text
无代码测试；这里的“失败”指没有正式 stop/go 文档。
```

**Step 2: Run to confirm failure**
Run: `go test -count=1 ./...`  
Expected: 工程通过，但还没有正式 entry verdict

**Step 3: Minimal implementation**
```text
review 文档必须回答：
- RV4 是否再次复现 shared-thread distortion
- distortion 是否与 RV3 同型
- formal closure 是否仍稳定
- 当前问题是否已满足 P3-D entry 条件
- 是否继续留在 real-use-first 阶段，还是进入 P3-D entry review
```

**Step 4: Run tests to confirm pass**
Run: `go test -count=1 ./...`  
Expected: 全量通过，review 文档完成

**Step 5: Commit**
```bash
git add docs/discussions/post-rv4-shared-thread-second-evidence-review.md
git commit -m "docs(review): add rv4 shared-thread second evidence verdict"
```

### Task W3-T2: Sync frontstage status

**Files:**
- Modify: `docs/progress-checklist.md`
- Modify: `docs/README.md`

**Depends on:** `[W3-T1]`

**Step 1: Write failing test**
```text
无代码测试；这里的“失败”指前台状态还没有同步 RV4 结论。
```

**Step 2: Run to confirm failure**
Run: `go test -count=1 ./...`  
Expected: 工程通过，但 docs 前台状态未同步

**Step 3: Minimal implementation**
```text
同步更新：
- RV4 已完成
- 当前是否进入 P3-D entry review
- 如果未进入，明确下一步仍是 real-use-first / evidence-first
```

**Step 4: Run tests to confirm pass**
Run: `go test -count=1 ./...`  
Expected: 全量通过，文档入口同步

**Step 5: Commit**
```bash
git add docs/progress-checklist.md docs/README.md
git commit -m "docs(progress): sync rv4 shared-thread evidence outcome"
```

## Wave Exit Gate
- Execute mandatory gate sequence via `executing-wave-plans`.
- Wave-specific acceptance:
  - [ ] RV4 blocker review 已完成
  - [ ] 已得出明确 verdict：进入或不进入 P3-D entry review
  - [ ] 前台入口已同步
- Wave-specific verification:
  - [ ] `go test -count=1 ./...`
  - [ ] `docs/implementation-notes/0033-rv4-shared-thread-second-evidence-validation.md` 已可独立阅读
  - [ ] `docs/discussions/post-rv4-shared-thread-second-evidence-review.md` 已给出明确 stop/go
- Boundary-change verification (if triggered):
  - [ ] 无

## Next Wave Entry Condition
- Governed by `executing-wave-plans` verdict rule (`Go` / satisfied `Conditional Go` only).

---

## P3-D Entry Rule

只有在以下条件同时成立时，才建议从 RV4 进入 `P3-D entry review`：

- RV3 和 RV4 都复现 shared-thread query distortion
- distortion 类型基本同型，而不是偶发随机差异
- distortion 已影响“按 Task 理解协作上下文”
- 当前问题不能通过更轻的 query / projection 修补说明性解决
- formal closure 虽稳定，但 shared context attribution 已不足以继续扩大真实使用

## Default Expected Verdict

默认预期不是“直接进入 P3-D implementation”，而是：

- 先完成 RV4
- 再做一次 `P3-D entry review`
- 只有 entry review 通过，才新开 P3-D implementation plan
