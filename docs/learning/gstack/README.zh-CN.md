# gstack 学习与迁移计划

> 状态：草案
>
> 创建日期：2026-03-21
>
> 研究对象：
> - 主参考：`garrytan/gstack`（2026 年的 AI software factory / skills workflow）
> - 次参考：`gstackio`（旧 PaaS 方向，仅参考 deploy / service binding 思路）

## 1. 一句话结论

`garrytan/gstack` 最值得我们学习的不是“底层 agent runtime”，而是“角色化工作流 + 产物传递 + 审查门禁 + 持久浏览器 QA”的上层产品设计。

对 `ai-workflow` 来说，正确策略不是整仓照搬，而是：

1. 先迁移可独立运行的流程型 skills。
2. 再把这些 skills 接入 `WorkItem / Thread / Artifact / Gate` 模型。
3. 最后补浏览器与安全 hook 等平台能力，把剩余 skills 吃下来。

## 1.1 当前已落地进展（2026-03-21）

截至目前，以下内容已经落地到仓库：

1. `docs/learning/gstack/` 学习目录与 upstream pin 文件已建立。
2. GitHub importer 已兼容两类布局：
   - `skills/<name>/SKILL.md`
   - `<repo-root>/<name>/SKILL.md`
3. 第一批归一化 builtin skills 已落地：
   - `gstack-office-hours`
   - `gstack-plan-ceo-review`
   - `gstack-plan-eng-review`
   - `gstack-review`
   - `gstack-document-release`
4. `Run.ResultMetadata` 已补一层稳定 artifact 契约，用来表达：
   - `artifact_namespace`
   - `artifact_type`
   - `artifact_format`
   - `artifact_relpath`
   - `artifact_title`
   - `producer_skill`
   - `producer_kind`
   - `summary`
5. HTTP artifact/deliverable 返回已开始抬平 `artifact` 语义，便于 UI 和 API 直接消费。
6. 当前在用的 `step-signal` 路径现在已可通过 HTTP decision payload 或 fallback signal line 携带上述 artifact metadata，
   因此第一批 `gstack-*` skill 不必等待新 artifact 表，就能把结构化产物挂回当前 Run 主链。

这意味着我们已经从“只是研究 gstack”推进到了“第一批 workflow 已可在本仓库中被识别、列出，并具备统一 artifact 语义”的阶段。

## 1.2 当前最小规则

先只保留两条规则，不再扩新概念：

1. `gstack-review`
   放在 `Run.ResultMetadata`
2. `gstack-office-hours`、`gstack-plan-ceo-review`、`gstack-plan-eng-review`、`gstack-document-release`
   放在 `ThreadMessage.Metadata`

## 2. 目标

本计划的目标不是“导入一个外部 skills 仓库”，而是建立一套可持续学习和持续吸收 upstream 的机制：

1. 能搬的先搬，尽快形成最小可用价值。
2. 不能直接搬的，明确缺口和平台前置条件。
3. 以后 upstream 更新时，我们能低成本继续同步和学习。
4. 最终形成 `ai-workflow` 自己的标准 sprint 工作流层。

## 3. 现状判断

### 3.1 我们已有的基础

- DAG 规划与 materialize：
  [`internal/application/planning/service.go`](/D:/project/ai-workflow/internal/application/planning/service.go#L11)
- Thread agent runtime：
  [`docs/spec/thread-agent-runtime.zh-CN.md`](/D:/project/ai-workflow/docs/spec/thread-agent-runtime.zh-CN.md#L16)
- ThreadTask 调度与任务组生命周期：
  [`internal/application/threadtaskapp/service.go`](/D:/project/ai-workflow/internal/application/threadtaskapp/service.go#L14)
- Skills 校验与读取：
  [`internal/skills/skillset.go`](/D:/project/ai-workflow/internal/skills/skillset.go#L16)
- Skills HTTP 管理面：
  [`internal/adapters/http/skills.go`](/D:/project/ai-workflow/internal/adapters/http/skills.go#L53)

### 3.2 当前缺口

- GitHub importer 只支持仓库内 `skills/<name>` 布局，不支持 `gstack` 这种“仓库顶层每个目录就是一个 skill”的布局：
  [`internal/skills/github_importer.go`](/D:/project/ai-workflow/internal/skills/github_importer.go#L94)
- skill frontmatter 当前只稳定使用 `name` / `description`，并未把 `hooks`、`allowed-tools`、`benefits-from` 等字段当作运行时语义处理：
  [`internal/skills/skillset.go`](/D:/project/ai-workflow/internal/skills/skillset.go#L145)
- 还没有可复用的持久态浏览器 worker。
- 还没有平台级的 tool hook / policy 机制，无法原样承接 `careful` / `freeze` / `guard`。
- 还没有把“设计文档 / review 结论 / QA 结论”正式纳入 artifact 主链。

## 4. skill 迁移分层

### 4.1 A 类：优先迁移，低成本高价值

这类 skills 主要是流程与提示词组织，几乎不依赖 gstack 私有能力。

建议第一批迁移：

1. `office-hours`
2. `plan-ceo-review`
3. `plan-eng-review`
4. `review`
5. `document-release`

原因：

- 直接提升我们现有平台的“工作流层”质量。
- 能和现有 `planning`、`threadtask`、`gate` 容易对接。
- 不依赖浏览器 daemon。
- 不依赖 hook 机制。

### 4.2 B 类：第二批迁移，需接系统语义

1. `ship`
2. `investigate`
3. `codex`
4. `retro`
5. `plan-design-review`
6. `design-consultation`

这些 skills 不是不能用，而是不能原样使用。

需要改造点：

- 路径从 `~/.gstack/...` 改为 `.ai-workflow/...` 或正式 artifact。
- 把 git / PR / test / review 的约定改为我们自己的系统语义。
- 把“本地文件记忆”改造成“artifact + store + event”。

### 4.3 C 类：依赖平台能力，暂不直接迁移

1. `browse`
2. `setup-browser-cookies`
3. `qa`
4. `qa-only`
5. `design-review`

这类技能依赖 `gstack` 最重的浏览器子系统，不能只复制 `SKILL.md`。

前置条件：

1. 持久态 browser daemon
2. 元素 ref 寻址
3. 浏览器状态文件与生命周期管理
4. 认证 cookie 导入
5. 可供 skill 使用的统一浏览器命令接口

### 4.4 D 类：不建议按 skill 搬，应做成平台能力

1. `careful`
2. `freeze`
3. `guard`
4. `unfreeze`
5. `gstack-upgrade`

原因：

- `careful` / `freeze` / `guard` 依赖 hook 拦截工具调用。
- `gstack-upgrade` 只服务于 gstack 自身分发模式。

正确做法：

- 把安全限制做成 `ai-workflow` 自己的 runtime policy / sandbox policy / tool middleware。
- 不把它们当普通文档型 skill 处理。

## 5. 迁移原则

### 5.1 不是“import and pray”，而是“vendor + normalize”

建议采用两层结构：

1. `upstream snapshot`
   - 保留原始 upstream 内容，用于对照、升级、diff
2. `runtime-ready normalized skill`
   - 删除 gstack 私有前置逻辑
   - 改写为适合 `ai-workflow` 的版本

推荐目录约定：

```text
docs/learning/gstack/                 # 学习与迁移文档
internal/skills/vendor/gstack-upstream/  # upstream 快照（只读）
internal/skills/builtin/gstack-*         # 归一化后的可运行版本
```

说明：

- `vendor/gstack-upstream/` 用于持续学习和升级 diff。
- `builtin/gstack-*` 用于实际运行。
- 统一加 `gstack-` 前缀，避免与现有 / 未来 skills 撞名。

### 5.2 所有迁移都要做“去私货”处理

默认删除或改写以下内容：

1. `~/.gstack/...` 本地状态路径
2. `gstack-config`
3. `gstack-update-check`
4. `gstack-telemetry-log`
5. `community mode` / `telemetry` / `upgrade`
6. `$B` 浏览器命令引用
7. `hooks` 配置

保留的内容：

1. 技能定位
2. 角色心智
3. workflow 主体
4. 结果标准
5. review / QA / ship 的判定框架

### 5.3 不要让 skill 只写本地文件

从一开始就应规划 skill 输出的落点：

- 产品/方案类输出：`ArtifactType=design_doc`
- 工程评审输出：`ArtifactType=eng_review`
- 代码审查输出：`ArtifactType=review_report`
- 文档修正输出：`ArtifactType=doc_update_plan`
- 发版输出：`ArtifactType=ship_report`

这些 artifact 当前只挂到两处：

1. `Run.ResultMetadata`
2. `ThreadMessage.Metadata`

当前不新增独立 `Artifact` 表，也不再引入额外 owner 抽象。

## 6. 分阶段计划

### Phase 0：建立学习与 vendor 基线

目标：

- 固定 upstream 来源
- 建立本地快照目录
- 明确第一批迁移范围

动作：

1. 在仓库中建立 `docs/learning/gstack/`
2. 建立 `internal/skills/vendor/gstack-upstream/` 约定
3. 记录 upstream repo、branch、commit、抓取时间
4. 初步选择第一批 skills

完成标准：

- 学习文档就位
- 有明确的 upstream pin 策略
- 有第一批 skill 白名单

### Phase 1：支持 gstack 仓库结构导入

目标：

- 让 importer 能读取 `gstack` 风格仓库布局

建议改造：

1. importer 同时支持：
   - `skills/<name>/SKILL.md`
   - `<repo-root>/<name>/SKILL.md`
2. 支持导入时传 `ref`
3. 导入结果记录来源 repo / ref / commit

完成标准：

- 能从 `garrytan/gstack` 成功抓到单个 skill
- 能保留 upstream 来源信息

### Phase 2：第一批 skills 归一化

目标：

- 产出可实际运行的 `gstack-*` skills

第一批：

1. `gstack-office-hours`
2. `gstack-plan-ceo-review`
3. `gstack-plan-eng-review`
4. `gstack-review`
5. `gstack-document-release`

归一化规则：

1. 删除 gstack preamble
2. 删除 telemetry / upgrade / contributor mode
3. 改写输出路径和 artifact 约定
4. 改写与我们 tool / runtime 一致的说明

完成标准：

- 每个 skill 都能通过当前 `ValidateSkillMD`
- 可被 skills API 列出与读取
- 有最小集成测试

### Phase 3：接入 artifact 与 gate

目标：

- 不只是“能运行”，而是进入平台主链

动作：

1. skill 结果统一落 artifact
2. `review` 结果可驱动 gate
3. `plan-*` 结果能回写到 planning / thread
4. `document-release` 能生成可执行 doc patch / checklist

完成标准：

- skill 输出不再是孤立 markdown
- 可在 UI / API / event 中追踪

### Phase 4：第二批技能迁移

目标：

- 接入更强的交付类能力

候选：

1. `gstack-ship`
2. `gstack-investigate`
3. `gstack-codex`
4. `gstack-retro`
5. `gstack-plan-design-review`

关键前提：

- 与 GitHub / tests / coverage / review 状态的系统集成足够清晰

### Phase 5：浏览器与 QA 子系统

目标：

- 为 `browse` / `qa` / `design-review` 建平台底座

建议：

1. 单独立项实现 browser worker
2. 学习 gstack 的：
   - daemon 模型
   - state file
   - ref system
   - version auto-restart
3. 完成后再迁移：
   - `gstack-browse`
   - `gstack-qa`
   - `gstack-qa-only`
   - `gstack-design-review`

## 7. 持续跟进 upstream 的机制

我们不应该只做一次性迁移，而要保留“继续学习 upstream”的能力。

### 7.1 版本跟踪

建议在后续补一个清单文件，例如：

```yaml
repo: https://github.com/garrytan/gstack
branch: main
commit: <pinned-commit>
synced_at: <utc-datetime>
skills:
  - office-hours
  - plan-ceo-review
  - plan-eng-review
  - review
  - document-release
watch_dirs:
  - office-hours
  - plan-ceo-review
  - plan-eng-review
  - review
  - document-release
  - browse
```

用途：

1. 固定当前学习基线
2. 后续能做 diff
3. 明确我们关心哪些目录

### 7.2 升级节奏

建议节奏：

1. 每周一次轻量 diff
2. 每月一次人工复盘
3. 每次发布前评估是否吸收重要上游变化

检查重点：

1. 新 skill 出现
2. 既有 skill workflow 明显升级
3. 浏览器层架构变化
4. 新的质量门禁 / 评审策略

### 7.3 升级策略

对每次 upstream 变化，做三类判定：

1. `adopt`
   - 可以直接吸收
2. `adapt`
   - 值得吸收，但要先改
3. `observe`
   - 先记录，不急着做

不要追求全量同步。
原则是：同步那些能提升我们平台默认路径的变化。

## 8. 建议的第一批落地范围

如果只允许做一小步，建议就做下面这组：

1. importer 支持 gstack 顶层 skill 目录
2. vendored upstream snapshot
3. 归一化 5 个 skills：
   - `gstack-office-hours`
   - `gstack-plan-ceo-review`
   - `gstack-plan-eng-review`
   - `gstack-review`
   - `gstack-document-release`
4. 给它们补最小测试
5. 把输出先落到 `.ai-workflow/artifacts/`，后续再接 store

这是当前性价比最高的切入点。

这组动作目前已经基本完成；下一步更合适的推进方向是：

1. 让 `gstack-review` 产物参与 gate 语义
2. 让线程页能识别 `ThreadMessage.Metadata` 里的 `gstack` 产物

## 9. 明确不做的事

当前阶段不建议：

1. 直接把整个 gstack 原样 vendoring 为运行时 skills
2. 先做 `browse` 再做流程型 skills
3. 为了迁移 `careful` / `freeze` 去硬塞一个半成品 hook 系统
4. 把所有 skill 都挂进第一版
5. 跟 upstream 追求逐字同步

## 10. 下一步建议

按优先级排序：

1. 做 importer 兼容改造，支持 `gstack` 仓库布局
2. 建 `vendor/gstack-upstream` 快照目录与元数据
3. 归一化第一批 5 个技能
4. 为这 5 个技能设计统一 artifact 输出约定
5. 再评估第二批技能

---

## 附录 A：参考来源

- `garrytan/gstack`：<https://github.com/garrytan/gstack>
- `gstack` 架构文档：<https://github.com/garrytan/gstack/blob/main/ARCHITECTURE.md>
- `gstack` 技能总览：<https://github.com/garrytan/gstack/blob/main/docs/skills.md>

## 附录 B：本计划回答的问题

本文件主要回答以下问题：

1. 哪些能挪
2. 哪些不能直接挪
3. 先做什么最值
4. 如何避免一次性调研后失联
5. 如何把 upstream 持续变成我们的学习源
