# Thread 制定计划后的审核链路

> 状态：现行
>
> 最后按代码核对：2026-03-21
>
> 适用范围：`Thread` / `Proposal` / `Initiative` / `WorkItem` / `Gate`

## 1. 结论

当前系统里，`thread` 会议收敛出计划后，存在审核，但审核分布在后续几个对象上：

1. `ThreadProposal` 有提交、审批通过、驳回、修订。
2. `Initiative` 有提审、审批通过、驳回。
3. `WorkItem` 执行阶段可以继续挂 `gate/review` 步骤。

重要区别：

- `thread` 会议结束后不会自动完成审核。
- 当前实现是先把会议结果沉淀为 `proposal`，再通过显式 API 进入审批链。

## 2. 当前主链

当前 Requirement / Thread 主链可以概括为：

`thread meeting -> proposal draft/open -> reject/revise/approve -> initiative draft/proposed/executing -> work item gate/review -> done`

分阶段看：

1. `Thread` 负责讨论、收敛、形成计划草案。
2. `Proposal` 负责把计划变成可审批的结构化对象。
3. `Initiative` 负责把获批方案变成可执行的 work item 关系组。
4. `WorkItem` 负责真正执行，并在执行层通过 `gate` 做质量把关。

## 3. Proposal 审核

### 3.1 HTTP 入口

`Proposal` 的 HTTP 入口已经明确区分了创建、提交审批、审批通过、驳回和修订：

- `POST /threads/{threadID}/proposals`
- `POST /proposals/{proposalID}/submit`
- `POST /proposals/{proposalID}/approve`
- `POST /proposals/{proposalID}/reject`
- `POST /proposals/{proposalID}/revise`

这说明 thread 会议收敛出的计划，并不是直接落到执行层，而是先进入 `proposal` 审批流程。

### 3.2 应用层行为

`proposalapp.Service` 当前支持以下状态推进：

1. `CreateProposal`
2. `Submit`
3. `Approve`
4. `Reject`
5. `Revise`

其中：

- `Submit` 会把草案推进到可审批状态。
- `Approve` 会把 proposal 物化为 `initiative` 与多个 `work item`。
- `Reject` 会把 proposal 打回。
- `Revise` 会把 proposal 置为修订中，允许继续编辑后再次提交。

因此，thread 计划的第一层正式审核，是 `proposal` 审批。

## 4. Initiative 审核

`Proposal` 审批通过后，并不是立即无条件执行，而是先生成 `initiative`。

`initiativeapp.Service` 当前支持：

1. `Propose`
2. `Approve`
3. `Reject`

这意味着：

- `Proposal` 过审，只代表计划被接受并物化。
- `Initiative` 仍然可以作为“执行前的正式批准层”。
- 真正进入 `executing`，要再经过一次 `initiative approve`。

因此，thread 计划落地后，当前主链实际上有第二层审批，即 `initiative` 审批。

## 5. WorkItem 执行审核

进入执行层后，系统还支持在 `work item` 内增加 `gate` 步骤。

`gate` 的作用不是审批计划本身，而是审核执行产物是否达标，例如：

- 功能是否完整
- 验收条件是否满足
- 是否需要返工

当前实现支持：

1. `ActionGate`
2. `acceptance_criteria`
3. gate reject 后触发 rework
4. gate approve 后继续或完成

因此，thread 计划在执行阶段还有第三层质量审核，但这已经属于执行审核，不再属于“计划审批”。

## 6. 是否自动审核

当前答案是否定的。

系统没有把“thread 会议收敛完成”直接绑定成“自动进入审批并自动裁决”。

现状是：

1. 先通过 thread 讨论形成方案。
2. 再显式创建 `proposal`。
3. 再显式 `submit/approve/reject/revise`。
4. proposal 通过后生成 `initiative`。
5. 再显式 `propose/approve/reject initiative`。
6. initiative 通过后进入 work item 执行。

所以，如果问“thread 制定完计划后有没有审核”，准确回答是：

- 有。
- 但审核不在 thread meeting 内自动完成。
- 审核主要落在 `proposal` 和 `initiative` 两层。

## 7. 旧 ThreadTask DAG 链路

仓库里还保留一条旧的 `thread task group / thread task DAG` 链路。

那条链路里也有 `review` 任务类型，支持：

- `work -> review`
- `review reject -> retry`
- `review complete -> group done`

这说明旧链路也有审核概念，但它属于 thread 内部任务编排模型，不是当前 Requirement -> Proposal -> Initiative -> WorkItem 这条主链的计划审批入口。

## 8. 当前应如何理解

对当前代码，更准确的理解方式是：

1. `Thread` 负责讨论和收敛。
2. `Proposal` 负责计划审批。
3. `Initiative` 负责执行前批准。
4. `Gate` 负责执行质量审核。

换句话说：

- `thread` 不是“计划一出就直接执行”
- `thread` 也不是“会议里自动审完”
- 而是“会议 -> proposal 审核 -> initiative 审核 -> execution gate 审核”的分层模型
