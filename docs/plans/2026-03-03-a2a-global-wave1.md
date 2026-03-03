# A2A 全局接入 Wave 1 Plan

## Wave Goal

完成 A2A 接入基础：直接对接官方 `a2a-go`、新增 A2A 路径配置开关、落地 A2A Bearer Token 鉴权，并保持默认关闭不影响现有链路。

## Tasks

### Task W1-T0: 依赖与基线整理（go module sums）

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

**Depends on:** `[]`

**Step 1: Write failing test**  
保持现有测试不动，本任务通过后续回归验证依赖变更不引入行为回归。

**Step 2: Run to confirm failure**  
无（依赖整理任务）。

**Step 3: Minimal implementation**
```text
补齐/规范 a2a-go 相关依赖与间接依赖，确保后续 Wave 代码可编译。
```

**Step 4: Run tests to confirm pass**
Run: `go test ./internal/config ./internal/web ./cmd/a2a-smoke -count=1`  
Expected: PASS。

**Step 5: Commit**
```bash
git add go.mod go.sum
git commit -m "chore(deps): normalize go module sums for a2a integration"
```

### Task W1-T1: 新增 A2A 配置与认证字段（默认关闭）

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/env.go`
- Modify: `internal/config/role_driven.go`
- Test: `internal/config/config_test.go`

**Depends on:** `[W1-T0]`

**Step 1: Write failing test**
```text
新增配置测试：
1) 默认 a2a.enabled=false；
2) a2a.token 可读取；
3) a2a.enabled=true 且 a2a.token 为空时配置错误（fail-fast）；
4) a2a.version 默认值为 0.3（或可回退到 0.3）。
```

**Step 2: Run to confirm failure**
Run: `go test ./internal/config -run 'TestA2A' -count=1`  
Expected: FAIL。

**Step 3: Minimal implementation**
```text
新增 A2AConfig（最小字段）：
- enabled
- token
- version

并支持环境变量：
- AI_WORKFLOW_A2A_ENABLED
- AI_WORKFLOW_A2A_TOKEN
- AI_WORKFLOW_A2A_VERSION
```

**Step 4: Run tests to confirm pass**
Run: `go test ./internal/config -run 'TestA2A' -count=1`  
Expected: PASS。

**Step 5: Commit**
```bash
git add internal/config/config.go internal/config/env.go internal/config/role_driven.go internal/config/config_test.go
git commit -m "feat(config): add minimal a2a config and token support"
```

### Task W1-T2: A2A 路由接线 + Token 鉴权 + AgentCard

**Files:**
- Create: `internal/web/handlers_a2a.go`
- Create: `internal/web/handlers_a2a_protocol.go`
- Modify: `internal/web/server.go`
- Test: `internal/web/handlers_a2a_test.go`
- Test: `internal/web/server_test.go`

**Depends on:** `[W1-T1]`

**Step 1: Write failing test**
```text
新增 Web 测试：
1) a2a.enabled=false 时 /api/v1/a2a 与 /.well-known/agent-card.json 返回 404；
2) disabled 时 .well-known 不得返回 SPA index（防 fallback 误命中）；
3) enabled=true 且 token 正确时可调用；
4) token 缺失/错误返回 401；
5) unsupported method 返回 JSON-RPC method not found。
```

**Step 2: Run to confirm failure**
Run: `go test ./internal/web -run 'TestA2A' -count=1`  
Expected: FAIL。

**Step 3: Minimal implementation**
```text
实现最小 A2A web 层：
- 注册 /api/v1/a2a
- 注册 /.well-known/agent-card.json
- A2A 专用 Bearer Token 校验
- A2A disabled 时两个入口均硬 404（不进入 SPA fallback）
```

**Step 4: Run tests to confirm pass**
Run: `go test ./internal/web -run 'TestA2A' -count=1`  
Expected: PASS。

**Step 5: Commit**
```bash
git add internal/web/handlers_a2a.go internal/web/handlers_a2a_protocol.go internal/web/server.go internal/web/handlers_a2a_test.go internal/web/server_test.go
git commit -m "feat(web): wire minimal a2a routes with bearer token auth"
```

### Task W1-T3: 让 `cmd/a2a-smoke` 支持认证场景

**Files:**
- Modify: `cmd/a2a-smoke/main.go`
- Test: `cmd/a2a-smoke/main_test.go`

**Depends on:** `[W1-T2]`

**Step 1: Write failing test**
```text
新增 smoke tool 测试：
1) 支持 -token 参数并注入 Authorization: Bearer；
2) 支持 A2A-Version 头；
3) 在 token 场景下可成功请求。
```

**Step 2: Run to confirm failure**
Run: `go test ./cmd/a2a-smoke -run 'TestToken' -count=1`  
Expected: FAIL。

**Step 3: Minimal implementation**
```text
增强 cmd/a2a-smoke：
- 增加 -token 参数
- 请求 AgentCard / JSON-RPC 时注入 Bearer
- 与 a2a-version 参数兼容
```

**Step 4: Run tests to confirm pass**
Run: `go test ./cmd/a2a-smoke -run 'TestToken' -count=1`  
Expected: PASS。

**Step 5: Commit**
```bash
git add cmd/a2a-smoke/main.go cmd/a2a-smoke/main_test.go
git commit -m "feat(a2a-smoke): support bearer token for secured a2a endpoints"
```

## Risks And Mitigations

- 风险：A2A token 配置缺失导致服务不可用。  
  缓解：strict 策略（enabled=true 且 token 为空直接 fail-fast）。

- 风险：A2A 路由误落入 SPA fallback。  
  缓解：disabled 分支对 `/api/v1/a2a` 与 `/.well-known/agent-card.json` 均使用硬 404。

## Wave E2E/Smoke Cases

### Cases
1. 默认配置启动，A2A 路径返回 404。  
2. 开启 A2A 且配置 token，token 错误返回 401。  
3. `cmd/a2a-smoke` 在 token 场景可跑通。

建议命令：
- `go test ./internal/config ./internal/web ./cmd/a2a-smoke -count=1`
- `go run ./cmd/a2a-smoke -card-base-url http://127.0.0.1:8080 -a2a-version 0.3 -token <A2A_TOKEN>`

## Wave Exit Gate
- Execute mandatory gate sequence via `executing-wave-plans`.
- Wave-specific acceptance:
  - [ ] A2A 路由已可按开关启停，默认关闭。
  - [ ] A2A Bearer Token 校验已生效。
  - [ ] `a2a.enabled=true` 且 `a2a.token` 为空时 fail-fast 生效。
  - [ ] `cmd/a2a-smoke` 已支持 token 场景。
- Wave-specific verification:
  - [ ] `go test ./internal/config ./internal/web ./cmd/a2a-smoke -count=1` 通过。

## Next Wave Entry Condition
- Governed by `executing-wave-plans` verdict rule (`Go` / satisfied `Conditional Go` only)。
