# AI Workflow 配置技能

你正在操作 ai-workflow 编排器配置。配置格式为 **TOML**，文件位于 `.ai-workflow/config.toml`。

## 配置文件位置

- 项目配置: `.ai-workflow/config.toml`
- 机密文件: `.ai-workflow/secrets.yaml`
- JSON Schema: `go run ./cmd/gen-schema > configs/config-schema.json`

## 当前主线配置

### Run

```toml
[run]
default_template = "standard"
global_timeout = "2h"
auto_infer_template = true
max_total_retries = 5
```

### Scheduler

```toml
[scheduler]
max_global_agents = 3
max_project_runs = 2

  [scheduler.watchdog]
  enabled = true
  interval = "5m"
```

### Runtime

`runtime` 是当前主线。agent driver、profile、sandbox、mcp、prompt 都从这里读取。

```toml
[[runtime.agents.drivers]]
id = "codex-acp"
launch_command = "npx"
launch_args = ["-y", "@zed-industries/codex-acp"]
  [runtime.agents.drivers.capabilities_max]
  fs_read = true
  fs_write = true
  terminal = true

[[runtime.agents.profiles]]
id = "worker"
name = "Worker (Codex)"
driver = "codex-acp"
role = "worker"
capabilities = ["dev.backend", "dev.frontend", "test"]
actions_allowed = ["read_context", "search_files", "fs_write", "terminal", "submit"]
prompt_template = "implement"
  [runtime.agents.profiles.session]
  reuse = true
  max_turns = 24
  idle_ttl = "15m"
```

### Server

```toml
[server]
host = "127.0.0.1"
port = 8080
```

### GitHub

```toml
[github]
enabled = true
owner = "your-org"
repo = "your-repo"
webhook_enabled = true
pr_enabled = true

  [github.pr]
  auto_create = true
  branch_prefix = "flow/"
```

### Store

```toml
[store]
driver = "sqlite"
path = ".ai-workflow/data.db"
```

### Log

```toml
[log]
level = "info"
file = ".ai-workflow/logs/app.log"
max_size_mb = 100
max_age_days = 30
```

## 验证规则

1. 仅支持当前 schema 中定义的字段，未知字段直接报错。
2. `runtime.mcp.profile_bindings` 必须引用存在的 `runtime.agents.profiles.id` 和 `runtime.mcp.servers.id`。
3. `runtime.mcp.profile_bindings.tool_mode = "allow_list"` 时，`tools` 不能为空。
4. `scheduler.watchdog.enabled = true` 时，各种 TTL/interval 必须大于 0。

## 说明

- `agents`、`roles`、`role_bindings`、`team_leader`、`a2a` 已不再属于现行配置模型。
- 需要新增 agent 或调整角色，请直接修改 `runtime.agents.drivers` / `runtime.agents.profiles`。
