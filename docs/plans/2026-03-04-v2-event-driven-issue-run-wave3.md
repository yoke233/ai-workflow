# Wave 3 Plan: 前端统一切换到 Issue/Profile/Run

## Wave Goal

前端彻底删除 task/plan 概念，页面与 API 客户端统一到 Issue/Profile/Run，UI 文案统一 Team Leader。

## Tasks

### Task W3-T1: 删除 TaskPlan/TaskItem 类型与 store

**Files:**
- Delete: `web/src/stores/plansStore.ts`
- Modify: `web/src/types/workflow.ts`
- Modify: `web/src/types/api.ts`
- Test: `web/src/lib/apiClient.test.ts`

**Depends on:** `[W2-T3]`

**Step 1: Write failing test**
```text
新增类型与序列化测试，断言：
1) Api 类型不再包含 TaskPlan/TaskItem
2) Issue/Profile/Run 类型可被客户端正确解析
```

**Step 2: Run to confirm failure**  
Run: `npm --prefix web run test -- apiClient.test.ts`  
Expected: 旧类型断言失败。

**Step 3: Minimal implementation**
```text
移除 TaskPlan/TaskItem 类型定义与引用，补齐 Issue/Profile/Run 类型。
```

**Step 4: Run tests to confirm pass**  
Run: `npm --prefix web run test -- apiClient.test.ts`  
Expected: 通过。

**Step 5: Commit**
```bash
git add web/src/types/workflow.ts web/src/types/api.ts web/src/lib/apiClient.test.ts
git rm web/src/stores/plansStore.ts
git commit -m "refactor(web): remove task-plan types and adopt issue-profile-run types"
```

### Task W3-T2: API Client 切换到 v2 issue/profile/run 路由

**Files:**
- Modify: `web/src/lib/apiClient.ts`
- Modify: `web/src/lib/apiClient.test.ts`
- Modify: `web/src/types/ws.ts`

**Depends on:** `[W3-T1]`

**Step 1: Write failing test**
```text
新增客户端测试：
1) 请求路径走 /api/v2/issues
2) profile/run API 与事件可解析
3) 不再请求 /plans 或 /tasks 路径
```

**Step 2: Run to confirm failure**  
Run: `npm --prefix web run test -- apiClient.test.ts`  
Expected: 路径断言失败。

**Step 3: Minimal implementation**
```text
替换请求路径与 payload 字段，删除 applyTaskAction 等旧调用。
```

**Step 4: Run tests to confirm pass**  
Run: `npm --prefix web run test -- apiClient.test.ts`  
Expected: 通过。

**Step 5: Commit**
```bash
git add web/src/lib/apiClient.ts web/src/lib/apiClient.test.ts web/src/types/ws.ts
git commit -m "feat(web): switch client routes to v2 issues profiles runs"
```

### Task W3-T3: 视图层重构与 Team Leader 文案替换

**Files:**
- Delete: `web/src/views/PlanView.tsx`
- Delete: `web/src/views/PlanView.test.tsx`
- Modify: `web/src/views/ChatView.tsx`
- Modify: `web/src/views/BoardView.tsx`
- Modify: `web/src/views/PipelineView.tsx`
- Test: `web/src/views/ChatView.test.tsx`
- Test: `web/src/views/BoardView.test.tsx`

**Depends on:** `[W3-T2]`

**Step 1: Write failing test**
```text
新增页面测试：
1) UI 文案与状态提示使用 Team Leader
2) Board/Chat 从 Issue/Run 数据渲染
3) 不再出现 Plan/Task 相关操作按钮
```

**Step 2: Run to confirm failure**  
Run: `npm --prefix web run test -- ChatView.test.tsx BoardView.test.tsx`  
Expected: 旧文案与旧组件断言失败。

**Step 3: Minimal implementation**
```text
删除 PlanView 与对应路由入口。
重构 ChatView/BoardView/PipelineView 的数据映射与 UI copy。
```

**Step 4: Run tests to confirm pass**  
Run: `npm --prefix web run test -- ChatView.test.tsx BoardView.test.tsx`  
Expected: 通过。

**Step 5: Commit**
```bash
git add web/src/views/ChatView.tsx web/src/views/BoardView.tsx web/src/views/PipelineView.tsx web/src/views/ChatView.test.tsx web/src/views/BoardView.test.tsx
git rm web/src/views/PlanView.tsx web/src/views/PlanView.test.tsx
git commit -m "refactor(web): remove plan view and rename secretary UX to team leader"
```

## Risks And Mitigations

- 风险: 前端删除旧类型导致连锁编译失败。  
  缓解: 先改类型与 API client，再改视图，按任务顺序推进。
- 风险: WS 事件名改动导致实时态异常。  
  缓解: 增补 ws 事件测试并对齐后端事件枚举。

## Wave Exit Gate

- Execute mandatory gate sequence via `executing-wave-plans`.
- Wave-specific acceptance:
  - [ ] 前端不存在 TaskPlan/TaskItem/PlanView 主路径引用。
  - [ ] API Client 全部走 issue/profile/run 新路由。
  - [ ] Team Leader 文案替换完成。
- Wave-specific verification:
  - [ ] `pwsh -NoProfile -File .\scripts\test\frontend-unit.ps1`
  - [ ] `pwsh -NoProfile -File .\scripts\test\frontend-build.ps1`
- Boundary-change verification (if triggered):
  - [ ] `go test ./internal/web -count=1 -timeout 60s`

## Next Wave Entry Condition

- Governed by `executing-wave-plans` verdict rule (`Go` / satisfied `Conditional Go` only).

