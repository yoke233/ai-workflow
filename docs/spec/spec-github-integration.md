# GitHub 集成规范（V2）

## 目标

将 GitHub 事件与本地 `issue/workflow_profile/workflow_run` 主链路对齐，
保证外部触发与本地观测一致。

## 触发来源

- `issues.opened`
- `issues.labeled`
- `issue_comment.created`
- `pull_request_review.submitted`

## 命令约定

- `/run`：按 issue 当前 `workflow_profile` 触发 run。
- `/run <profile>`：使用指定档位触发（`normal|strict|fast_release`）。
- `/review`：触发 issue review（若存在 `review_scope.files`，仅审该范围）。
- `/cancel`：取消当前活跃 run（可选实现）。

## 幂等与并发规则

- 同一 GitHub issue 在幂等窗口内重复触发，只建立一次本地 issue 关联。
- 当 issue 已有活跃 run 时，重复 `/run` 应拒绝并返回当前 run 信息。
- webhook 重放必须可识别并忽略重复执行。

## 权限规则

- 评论作者必须满足项目配置的最小权限。
- 无权限请求返回说明性评论，不触发 run。
- 权限判定失败需写审计日志。

## 事件回写

GitHub 评论应包含：

1. run 启动提示（run_id + profile）
2. run 结束摘要（`done/failed/timeout/cancelled`）
3. review 摘要与建议
4. 失败原因（如权限不足、配置缺失、执行超时）

## 故障处理

- Webhook 验签失败：拒绝并记录审计日志。
- 外部 API 超时：按策略重试并写入失败事件。
- 评论回写失败：不阻断本地状态推进，但必须告警。
- 本地状态写入失败：返回失败并禁止“仅 GitHub 成功”的假阳性。

## 可观测性

- 每次 GitHub 触发都必须能追踪到 `issue_id` 与 `run_id`。
- 允许从 issue 时间线反查来源 webhook 与 comment id。
- 必须支持按仓库/issue 查询最近失败的 webhook 处理记录。
