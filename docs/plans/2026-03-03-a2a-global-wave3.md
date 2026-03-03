# A2A 全局接入 Wave 3 Plan

## Wave Goal

在 Wave 2 后端可用的基础上完成前端最小接入与回归收口：提供独立的 A2A 交互入口（避免与现有 ChatView 并行改动冲突）、落地 `a2aClient`、补齐端到端烟测证据。

## Tasks

### Task W3-T1: 新增前端 `a2aClient` 与协议类型

**Files:**
- Create: `web/src/lib/a2aClient.ts`
- Create: `web/src/types/a2a.ts`
- Test: `web/src/lib/a2aClient.test.ts`

**Depends on:** `[W2-T4]`

**Step 1: Write failing test**
```text
新增 client 单测：
1) 正确请求 /.well-known/agent-card.json；
2) message/send、tasks/get、tasks/cancel 请求与响应解析正确；
3) token 注入复用现有前端 token 来源；
4) 401/JSON-RPC error 可稳定映射到前端错误对象。
```

**Step 2: Run to confirm failure**
Run: `npm --prefix web run test -- a2aClient`  
Expected: FAIL。

**Step 3: Minimal implementation**
```text
实现最小 a2aClient：
- fetchAgentCard
- sendMessage
- getTask
- cancelTask
- streamMessage（最小事件消费）
```

**Step 4: Run tests to confirm pass**
Run: `npm --prefix web run test -- a2aClient`  
Expected: PASS。

**Step 5: Commit**
```bash
git add web/src/lib/a2aClient.ts web/src/types/a2a.ts web/src/lib/a2aClient.test.ts
git commit -m "feat(web): add minimal a2a client and protocol types"
```

### Task W3-T2: 增加独立 A2A 入口（避免与 ChatView 冲突）

**Files:**
- Modify: `web/src/App.tsx`
- Modify: `web/src/views/ProjectsView.tsx`（或等效入口导航文件）
- Create: `web/src/views/A2APlaygroundView.tsx`
- Test: `web/src/views/A2APlaygroundView.test.tsx`

**Depends on:** `[W3-T1]`

**Step 1: Write failing test**
```text
新增入口与路由测试：
1) 新入口可见并可进入 A2A 独立页面；
2) 不修改 ChatView 既有行为与路由；
3) 未配置 token 时页面给出明确提示但不影响其他页面。
```

**Step 2: Run to confirm failure**
Run: `npm --prefix web run test -- A2APlaygroundView`  
Expected: FAIL。

**Step 3: Minimal implementation**
```text
新增独立 A2A 页面与入口：
- 使用单独路由承载 A2A 调试/交互
- 避免侵入 ChatView 主流程，降低并行合并冲突
```

**Step 4: Run tests to confirm pass**
Run: `npm --prefix web run test -- A2APlaygroundView`  
Expected: PASS。

**Step 5: Commit**
```bash
git add web/src/App.tsx web/src/views/ProjectsView.tsx web/src/views/A2APlaygroundView.tsx web/src/views/A2APlaygroundView.test.tsx
git commit -m "feat(web): add standalone a2a entry to avoid chatview merge conflicts"
```

### Task W3-T3: 页面交互接线（send/get/cancel/stream）

**Files:**
- Modify: `web/src/views/A2APlaygroundView.tsx`
- Modify: `web/src/stores/*`（按需）
- Test: `web/src/views/A2APlaygroundView.test.tsx`

**Depends on:** `[W3-T2]`

**Step 1: Write failing test**
```text
新增交互测试：
1) 发送 message/send 后展示 task id 和状态；
2) tasks/get 可刷新任务状态；
3) tasks/cancel 可更新为取消状态；
4) stream 事件可增量渲染并在 done 收敛。
```

**Step 2: Run to confirm failure**
Run: `npm --prefix web run test -- A2APlaygroundView`  
Expected: FAIL。

**Step 3: Minimal implementation**
```text
在独立 A2A 页面中接线 a2aClient：
- 请求发送与状态查询
- 取消控制
- 流式事件输出（最小 UI）
```

**Step 4: Run tests to confirm pass**
Run: `npm --prefix web run test -- A2APlaygroundView`  
Expected: PASS。

**Step 5: Commit**
```bash
git add web/src/views/A2APlaygroundView.tsx web/src/stores
git commit -m "feat(web): wire a2a send/get/cancel/stream interactions"
```

### Task W3-T4: 全链路回归与证据归档

**Files:**
- Create: `docs/reports/a2a-e2e-smoke.md`

**Depends on:** `[W3-T3]`

**Step 1: Backend regression**
Run: `pwsh -NoProfile -File .\scripts\test\backend-all.ps1`  
Expected: PASS。

**Step 2: Frontend regression**
Run:
```powershell
pwsh -NoProfile -File .\scripts\test\frontend-unit.ps1
pwsh -NoProfile -File .\scripts\test\frontend-build.ps1
```
Expected: PASS。

**Step 3: Integration + smoke**
Run:
```powershell
pwsh -NoProfile -File .\scripts\test\p3-integration.ps1
go run ./cmd/a2a-smoke -card-base-url http://127.0.0.1:8080 -a2a-version 0.3 -token <A2A_TOKEN>
```
Expected: PASS，包含 `send_result` 与 `task_state`。

**Step 4: Record evidence**
```text
把关键命令、提交 SHA、结果摘要写入 docs/reports/a2a-e2e-smoke.md。
```

**Step 5: Commit**
```bash
git add docs/reports/a2a-e2e-smoke.md
git commit -m "test(a2a): add end-to-end smoke evidence for frontend and backend"
```

## Risks And Mitigations

- 风险：多人同时改 ChatView 造成冲突与回归。  
  缓解：Wave3 默认走独立入口 `A2APlaygroundView`，不强耦合到 ChatView。

- 风险：前端 token 配置缺失导致误判后端故障。  
  缓解：页面明确展示 token 缺失提示，并在请求层保留原始 401/JSON-RPC 错误。

## Wave E2E/Smoke Cases

### Cases
1. 进入独立 A2A 页面后可完成 send/get/cancel 基础闭环。  
2. stream 事件可被页面增量展示并正确收尾。  
3. backend/frontend/integration 全部回归通过。  
4. `cmd/a2a-smoke` 在 token 场景通过并留证据。

建议命令：
- `pwsh -NoProfile -File .\scripts\test\backend-all.ps1`
- `pwsh -NoProfile -File .\scripts\test\frontend-unit.ps1`
- `pwsh -NoProfile -File .\scripts\test\frontend-build.ps1`
- `pwsh -NoProfile -File .\scripts\test\p3-integration.ps1`
- `go run ./cmd/a2a-smoke -card-base-url http://127.0.0.1:8080 -a2a-version 0.3 -token <A2A_TOKEN>`

## Wave Exit Gate
- Execute mandatory gate sequence via `executing-wave-plans`.
- Wave-specific acceptance:
  - [ ] 已新增独立 A2A 入口，不阻塞 ChatView 并行改动。
  - [ ] A2A 页面完成 send/get/cancel/stream 最小交互。
  - [ ] E2E/Smoke 证据已归档。
- Wave-specific verification:
  - [ ] `pwsh -NoProfile -File .\scripts\test\backend-all.ps1` 通过。
  - [ ] `pwsh -NoProfile -File .\scripts\test\frontend-unit.ps1` 通过。
  - [ ] `pwsh -NoProfile -File .\scripts\test\frontend-build.ps1` 通过。
  - [ ] `pwsh -NoProfile -File .\scripts\test\p3-integration.ps1` 通过。

## Next Wave Entry Condition
- 已为最后一波，无后续 Wave。完成后进入收尾合并与发布流程。
