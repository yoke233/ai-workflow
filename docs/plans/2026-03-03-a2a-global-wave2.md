# A2A 全局接入 Wave 2 Plan

## Wave Goal

在 Wave 1 鉴权与协议入口基础上，完成 `secretary` 薄桥接与 `internal/web` 协议适配，并提供最小可用 A2A 方法集：`message/send`、`tasks/get`、`tasks/cancel`、`message/stream`。

## Tasks

### Task W2-T1: 新增 `secretary` A2A Bridge

**Files:**
- Create: `internal/secretary/a2a_bridge.go`
- Test: `internal/secretary/a2a_bridge_test.go`
- Modify: `internal/secretary/manager.go`（按需导出 bridge 需要的最小能力）

**Depends on:** `[W1-T3]`

**Step 1: Write failing test**
```text
新增 bridge 测试：
1) SendMessage 返回 task snapshot；
2) GetTask / CancelTask 正常；
3) task not found 与 invalid input 错误映射稳定；
4) project_id 作用域校验可覆盖。
```

**Step 2: Run to confirm failure**
Run: `go test ./internal/secretary -run 'TestA2ABridge' -count=1`  
Expected: FAIL。

**Step 3: Minimal implementation**
```text
实现 A2ABridge：
- SendMessage
- GetTask
- CancelTask

返回领域任务快照（task_id/state 等），不直接暴露 a2a-go 协议类型。
```

**Step 4: Run tests to confirm pass**
Run: `go test ./internal/secretary -run 'TestA2ABridge' -count=1`  
Expected: PASS。

**Step 5: Commit**
```bash
git add internal/secretary/a2a_bridge.go internal/secretary/a2a_bridge_test.go internal/secretary/manager.go
git commit -m "feat(secretary): add thin a2a bridge over existing manager"
```

### Task W2-T2: Web JSON-RPC 方法适配（send/get/cancel）

**Files:**
- Modify: `internal/web/handlers_a2a.go`
- Modify: `internal/web/handlers_a2a_protocol.go`
- Modify: `internal/web/handlers_a2a_test.go`

**Depends on:** `[W2-T1]`

**Step 1: Write failing test**
```text
新增/扩展 handler 测试：
1) message/send 返回 task snapshot；
2) tasks/get 返回一致状态；
3) tasks/cancel 状态转换正确；
4) invalid params 返回 -32602；
5) project scope mismatch 返回业务错误码。
```

**Step 2: Run to confirm failure**
Run: `go test ./internal/web -run 'TestA2A(MessageSend|TasksGet|TasksCancel|InvalidParams)' -count=1`  
Expected: FAIL。

**Step 3: Minimal implementation**
```text
在 A2A handler 中分发：
- message/send -> bridge.SendMessage
- tasks/get -> bridge.GetTask
- tasks/cancel -> bridge.CancelTask
- 在 handlers_a2a_protocol.go 统一把 bridge 结果组装为 JSON-RPC result
```

**Step 4: Run tests to confirm pass**
Run: `go test ./internal/web -run 'TestA2A(MessageSend|TasksGet|TasksCancel|InvalidParams)' -count=1`  
Expected: PASS。

**Step 5: Commit**
```bash
git add internal/web/handlers_a2a.go internal/web/handlers_a2a_protocol.go internal/web/handlers_a2a_test.go
git commit -m "feat(a2a): implement send get cancel methods via secretary bridge"
```

### Task W2-T3: 增加最小 `message/stream` 能力

**Files:**
- Modify: `internal/web/handlers_a2a.go`
- Create: `internal/web/handlers_a2a_stream.go`
- Test: `internal/web/handlers_a2a_stream_test.go`

**Depends on:** `[W2-T2]`

**Step 1: Write failing test**
```text
新增 stream 测试：
1) message/stream 返回增量事件序列；
2) 未授权返回 401；
3) 请求取消时不阻塞。
```

**Step 2: Run to confirm failure**
Run: `go test ./internal/web -run 'TestA2A(Stream|Unauthorized)' -count=1`  
Expected: FAIL。

**Step 3: Minimal implementation**
```text
实现最小流式能力：
- 解析 message/stream params
- 触发 bridge.SendMessage
- 按 SSE 事件写出 delta/task/done
```

**Step 4: Run tests to confirm pass**
Run: `go test ./internal/web -run 'TestA2A(Stream|Unauthorized)' -count=1`  
Expected: PASS。

**Step 5: Commit**
```bash
git add internal/web/handlers_a2a.go internal/web/handlers_a2a_stream.go internal/web/handlers_a2a_stream_test.go
git commit -m "feat(a2a): add minimal message stream support"
```

### Task W2-T4: 使用 `cmd/a2a-smoke` 做后端协议烟测

**Files:**
- Create: `docs/reports/a2a-backend-smoke.md`

**Depends on:** `[W2-T3]`

**Step 1: Run backend regression**
Run: `go test ./internal/web ./internal/secretary -count=1`  
Expected: PASS。

**Step 2: Run smoke (token scene)**
Run:
```powershell
go run ./cmd/a2a-smoke -card-base-url http://127.0.0.1:8080 -a2a-version 0.3 -token <A2A_TOKEN>
```
Expected: 输出 `send_result` 与 `task_state`。

**Step 3: Record evidence**
```text
把命令、参数、结果摘要写入 docs/reports/a2a-backend-smoke.md
```

**Step 4: Commit**
```bash
git add docs/reports/a2a-backend-smoke.md
git commit -m "test(a2a): add backend smoke evidence based on cmd/a2a-smoke"
```

## Risks And Mitigations

- 风险：A2A handler 直接堆业务逻辑。  
  缓解：业务流程收敛到 `secretary/a2a_bridge.go`，协议对象组装集中在 `internal/web` 适配层。

- 风险：多项目场景下 project scope 漏校验。  
  缓解：bridge 层对 `project_id` 执行 scope 校验并在 web 层映射稳定错误码。

## Wave E2E/Smoke Cases

### Cases
1. `message/send` 可创建/返回 task。
2. `tasks/get` 可查询状态。
3. `tasks/cancel` 可执行取消。
4. `message/stream` 可返回最小事件流。
5. `cmd/a2a-smoke` 在认证场景跑通。

建议命令：
- `go test ./internal/web ./internal/secretary -run 'TestA2A|TestA2ABridge' -count=1`
- `go run ./cmd/a2a-smoke -card-base-url http://127.0.0.1:8080 -a2a-version 0.3 -token <A2A_TOKEN>`

## Wave Exit Gate
- Execute mandatory gate sequence via `executing-wave-plans`.
- Wave-specific acceptance:
  - [ ] A2A 最小方法集 + 流式可用。
  - [ ] `cmd/a2a-smoke` 已在 token 场景跑通并留证据。
- Wave-specific verification:
  - [ ] `go test ./internal/web ./internal/secretary -count=1` 通过。
  - [ ] `pwsh -NoProfile -File .\scripts\test\backend-all.ps1` 通过。

## Next Wave Entry Condition
- Governed by `executing-wave-plans` verdict rule (`Go` / satisfied `Conditional Go` only)。
