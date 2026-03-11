# V2 E2E 集成测试覆盖

测试文件: `internal/v2/api/integration_test.go`

完整栈组装: Store + MemBus + EventPersister + ConfigRegistry + FlowEngine + FlowScheduler + HTTP API (chi)

| # | 测试 | 验证内容 |
|---|------|---------|
| 1 | FullLifecycle | Project → Flow → DAG(A→B→C) → Scheduler 队列 → 执行 → Execution 验证 → 事件持久化 → Stats |
| 2 | FanOutFanIn | 扇出 DAG: A → (B,C,D) → E 并发执行，验证汇聚节点 |
| 3 | GatePass | exec → gate(pass) → exec，验证 gate.passed 事件 |
| 4 | RetryThenSucceed | transient 错误重试后成功，验证多次 Execution 记录 |
| 5 | PermanentFailure | permanent 错误不重试，Flow 立即失败 |
| 6 | CancelRunningFlow | 通过 Scheduler 取消运行中 Flow |
| 7 | ProjectCRUD | 创建/读取/更新/列表/删除 |
| 8 | ResourceBindingCRUD | 项目资源绑定完整生命周期 |
| 9 | AgentDriverProfileCRUD | Driver/Profile CRUD + 能力溢出 422 + 引用保护 409 |
| 10 | WebSocketEvents | 实时 WebSocket 接收 flow 生命周期事件 |
| 11 | SchedulerStats | 调度器统计端点 |
| 12 | FlowWithInvalidProject | project_id 不存在 → 404 |
| 13 | ConcurrentFlows | 3 个 Flow 并发执行，scheduler 并发控制 (max=2) |

## 运行方式

```bash
# 仅跑集成测试
go test ./internal/v2/api/ -run TestIntegration -v -count=1

# 跑全部 v2 测试
go test ./internal/v2/... -count=1
```

## 尚未覆盖

- 真实 ACP Agent 进程 (当前使用 mock executor)
- Workspace 隔离 (git worktree / local_fs)
- LeadAgent 聊天会话生命周期
- Composite 步骤通过 HTTP API 触发
