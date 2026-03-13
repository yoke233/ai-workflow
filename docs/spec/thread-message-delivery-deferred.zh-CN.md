# Thread Message Delivery 暂缓说明

> 状态：记录当前边界 / 暂不实现
>
> 创建日期：2026-03-14
>
> 关联规格：`thread-agent-runtime.zh-CN.md`

## 1. 结论

当前 `Thread` 消息投递继续维持为 **best-effort routing**，不升级为可靠投递系统。

这意味着：

- `thread_messages` 仍然是消息时间线主表
- `thread.send` 的成功语义仍然是“消息已写入 Thread 时间线”
- agent 路由失败仍主要通过 `thread.agent_failed` 事件暴露
- 当前版本**不新增** `thread_message_deliveries`、`delivery attempt`、`delivery ack ledger` 等实体

这个决定是刻意的，不是遗漏。

## 2. 当前行为边界

截至当前代码，Thread 人类消息的处理语义是：

1. 先校验 thread、reply_to、target agent 与 recipient
2. 把 human message 写入 `thread_messages`
3. 发布 `thread.message` 事件
4. 若存在 thread runtime，则异步尝试把消息路由给目标 agent
5. 若路由失败，则发布 `thread.agent_failed` 事件

当前系统**没有**以下保证：

- 不保证每个 recipient 至少成功接收一次
- 不保证消息失败后自动重试
- 不保证存在“某条消息被哪个 agent 实际消费”的持久记录
- 不保证前端可查询每个 recipient 的 delivery 状态

换句话说，当前模型是：

- `message persisted` 与 `message delivered` 不是同一件事
- 系统只保证前者，不保证后者

## 3. 为什么暂不做

当前如果引入 reliable delivery，成本不是“补个字段”，而是引入一整套新子域：

- `ThreadMessageDelivery`
- delivery 状态机
- attempt / retry / backoff
- 调度器或后台 worker
- 与 runtime 的 ack 对齐
- delivery 查询与重试 API

这会把 `Thread` 从“共享讨论容器”升级成“可审计消息投递系统”。

在当前阶段，这个复杂度偏高，主要问题有：

1. 领域体积会明显膨胀
2. 会新增大量状态与恢复路径
3. 需要重新定义 UI 成功语义
4. 需要把 runtime 接口从“发送消息”升级成“处理 delivery 任务”

因此当前不做半套实现：

- 不给 `ThreadMessage` 硬塞单个 `delivery_status`
- 不先做只有失败标记、没有 recipient 维度的伪模型
- 不把 event 当作可靠 delivery 真相源

## 4. 当前推荐表述

所有文档、代码评审、产品语义都应明确：

- Thread 当前支持多 agent 路由
- 路由语义是 **best-effort**
- `thread.agent_failed` 属于观测事件，不代表完整 delivery ledger

应避免以下误导性表述：

- “消息已发送给所有 agent”
- “消息投递可靠”
- “可以查询每条消息的投递结果”
- “失败后系统会自动补投”

更准确的表述应是：

- “消息已写入 Thread，并尝试路由给相关 agent”
- “若路由失败，会产出失败事件供排障”

## 5. 什么时候再做

只有在以下需求真的成立时，再考虑引入 delivery 子模型：

1. 需要按 recipient 查询某条消息是否被成功投递
2. 需要后台自动重试与失败补投
3. 需要 operator 审计“谁没收到、为什么没收到”
4. 需要前端展示消息级投递状态
5. 需要把 Thread 升级为正式协作编排通道，而不是讨论容器

若未来重启这项工作，推荐最小落地顺序：

1. 新增 `thread_message_deliveries` 持久化模型
2. message 与 delivery rows 同事务创建
3. 引入 dispatcher 扫描 `queued/retrying`
4. runtime 接口从 `SendMessage` 升级为以 delivery 为中心
5. 最后再补 UI 与 retry API

## 6. 当前技术债记录

本项技术债成立，但属于**有意识接受的产品/工程边界**，不是当前必须立刻修复的 bug。

短期接受的代价：

- 排障更多依赖事件和日志，而不是 delivery ledger
- 无法精确回答“哪条消息是否被某个 agent 成功接收”
- 无法做稳定的自动重试

短期换来的收益：

- 不额外引入 delivery 子域和一批新实体
- 保持 Thread 模型仍以讨论时间线为中心
- 把复杂度留给真正需要可靠投递时再承担
