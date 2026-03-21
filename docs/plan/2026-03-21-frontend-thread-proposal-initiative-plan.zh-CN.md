# 2026-03-21 前端补齐 Requirement / Thread / Proposal / Initiative 操作面计划

> 状态：计划中
>
> 最后按代码核对：2026-03-21

## 1. 目标

把当前已经打通的后端主链：

`Requirement -> Thread -> Proposal -> Initiative -> WorkItem`

补成前端可操作、可观察、可回退的完整界面链路，而不是停留在：

`Requirement -> Thread -> WorkItem`

## 2. 当前前端现状

### 2.1 已有页面与操作

1. `RequirementPage`
   - 支持需求分析
   - 支持选择项目、agents、meeting mode、meeting rounds
   - 支持直接创建 thread
2. `ThreadsPage`
   - 支持列出 thread
   - 支持创建普通 thread
   - 支持跳到“从需求创建”
3. `ThreadDetailPage`
   - 支持消息发送、agent 邀请、附件、文件引用
   - 支持修改 thread summary、meeting mode、routing mode
   - 支持创建或链接 work item
   - 支持旧的 task group 操作
4. `WorkItemsPage / WorkItemDetailPage`
   - 支持工作项列表、详情、运行、取消、编辑
   - 支持展示来源 thread 和依赖 work items

### 2.2 已缺失的主链操作

1. 没有 `Proposal` 类型、API、store、页面
2. 没有 `Initiative` 类型、API、store、页面
3. Thread 页面没有“从讨论产出 proposal 并提交审批”的操作面
4. 没有 proposal 审批页或 proposal 详情页
5. 没有 initiative 详情页、提审页、审批页
6. WorkItem 页面看不到来自哪个 proposal / initiative
7. 没有把 thread 中的系统消息结构化成“审批时间线”

## 3. 设计原则

1. 先补主链闭环，再做体验增强
2. 优先利用当前已有页面，而不是立刻新增过多产品面
3. 保持 thread 是讨论空间，proposal / initiative 是审批与执行编排对象
4. 所有关键状态都要有明确入口、反馈和跳转
5. 每个阶段先补最小可用操作，再补批量操作和富展示

## 4. 建议的前端补齐顺序

### Phase 1：补齐契约层

先补类型和 API，不先做复杂 UI。

范围：

1. `web/src/types/apiV2.ts`
   - 新增 `ThreadProposal`
   - 新增 `ProposalWorkItemDraft`
   - 新增 `Initiative`
   - 新增 `InitiativeDetail`
   - 新增 `InitiativeItem`
   - 新增 proposal / initiative 请求体
2. `web/src/lib/apiClient.ts`
   - proposal:
     - `listThreadProposals`
     - `createThreadProposal`
     - `getProposal`
     - `updateProposal`
     - `replaceProposalDrafts`
     - `submitProposal`
     - `approveProposal`
     - `rejectProposal`
     - `reviseProposal`
   - initiative:
     - `listInitiatives`
     - `getInitiative`
     - `proposeInitiative`
     - `approveInitiative`
     - `rejectInitiative`
     - `cancelInitiative`
     - `listInitiativeThreads`

验收目标：

1. 前端契约能完整表达后端主链
2. `apiClient` 单测覆盖新增路由命中

### Phase 2：在线程页补 proposal 操作面

不急着上独立 proposal 页面，先把最关键操作放进 `ThreadDetailPage`。

建议补充：

1. proposal 列表卡片
   - 当前 thread 下有哪些 proposal
   - 当前 proposal 状态
   - 来源消息、最近更新时间
2. proposal 创建抽屉/弹窗
   - 标题
   - summary
   - content
   - source message
   - work item drafts
3. proposal 审批操作
   - submit
   - reject
   - revise
   - approve
4. proposal 草案编辑
   - work item drafts 可增删改
   - 依赖链可视化或至少列表化

验收目标：

1. 用户能在 thread 内完成“讨论 -> proposal -> 审批”
2. proposal 状态变化能刷新 thread 页面

### Phase 3：补 initiative 详情与执行入口

`approve proposal` 后，需要给用户一个可以继续操作的落点。

建议补充：

1. 新增 `InitiativeDetailPage`
   - 展示 initiative 基本信息
   - 展示 initiative items
   - 展示 work item 依赖关系和状态汇总
   - 展示关联 threads
2. 在 thread / proposal 审批通过后
   - 显示跳转到 initiative 的 CTA
3. 在 initiative 页面补操作
   - propose
   - approve
   - reject
   - cancel

验收目标：

1. proposal 过审后，用户知道下一步去哪
2. initiative 的审批和执行入口不再依赖 API 手工调用

### Phase 4：补工作项来源追踪和审批时间线

当前 `WorkItemDetailPage` 只看得到来源 thread，不够。

建议补充：

1. 新增来源信息块
   - `source_proposal_id`
   - `source_initiative_id`
   - `proposal_temp_id`
2. 新增“审批链路”时间线
   - meeting_summary
   - proposal_submitted
   - proposal_rejected / revised / merged
   - initiative proposed / approved
3. 在 thread 页面把系统消息和结构化对象联动显示

验收目标：

1. 用户能从 work item 反查到 proposal / initiative / thread
2. 前端可视化主链而不是只显示离散消息

### Phase 5：收口旧 Task Group 面板

当前 thread 页同时存在：

1. 新主链：Requirement -> Proposal -> Initiative -> WorkItem
2. 旧链：Thread Task Group

这会让界面语义冲突。

建议：

1. 先把 Task Group 面板降级为实验能力
2. 在线程页中把 proposal / initiative 提升为主操作区
3. 等主链 UI 稳定后，再决定：
   - 隐藏
   - 折叠到高级区
   - 迁移到独立实验入口

## 5. 页面与信息架构建议

### 最小路由补充

建议新增：

1. `/initiatives/:initiativeId`

proposal 页面可以先不独立路由，先嵌在 thread 详情里。

### Thread 页建议分区

当前 `ThreadDetailPage` 建议收敛为 4 个主区：

1. 讨论区
2. Proposal 区
3. Work Item / Initiative 区
4. Agent / Context / Files 区

### 详情跳转建议

建议建立以下稳定跳转：

1. Requirement 创建完成 -> Thread
2. Proposal merged -> Initiative
3. Initiative item -> WorkItem
4. WorkItem source -> Thread / Initiative

## 6. 测试计划

至少分三层补：

1. `apiClient` 单测
   - 校验 proposal / initiative 路由
2. 页面组件测试
   - Thread 页面 proposal 操作流
   - Initiative 详情页状态流
   - WorkItem 来源追踪展示
3. 最小端到端页面流
   - Requirement 创建 thread
   - Thread 创建并审批 proposal
   - 跳转 initiative
   - initiative approve 后跳到 work item

## 7. 风险点

1. 旧 `ThreadTaskGroup` 与新 proposal / initiative 主链共存，容易让用户误解
2. Thread 页面已较重，继续堆 proposal UI 可能导致可维护性下降
3. 如果不先补类型层，页面很容易直接写成临时 `any`
4. 若没有结构化时间线，审批状态会继续依赖 system message 文案解析

## 8. 建议的实施次序

建议按下面顺序实际开工：

1. 契约层
2. Thread 内 proposal 面
3. Initiative 详情页
4. WorkItem 来源与审批链展示
5. Task Group 面板收口

这样可以最短路径补齐真正缺的操作闭环。
