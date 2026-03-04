# GitHub 集成规范（V2）

## 目标

将 GitHub Issue 评论与本地 `issue/profile/run` 主链路对齐，实现统一观测。

## 触发来源

- `issues.opened`
- `issues.labeled`
- `issue_comment.created`
- `pull_request_review.submitted`

## 命令约定

- `/run`：触发默认 profile 的 run。
- `/run <profile>`：触发指定 profile 的 run。
- `/review`：触发 issue review。

## 幂等规则

- 同一个 GitHub issue 在同一窗口内重复触发时只创建一次本地 issue 关联。
- 重复 `/run` 默认进入“拒绝并提示已有运行态”。

## 权限规则

- 评论作者必须满足项目配置的最小权限。
- 无权限请求返回说明性评论，不触发 run。

## 事件回写

GitHub 评论应包含：

1. run 启动提示
2. run 结束摘要（成功/失败/取消）
3. review 摘要与建议

## 故障处理

- Webhook 验签失败：拒绝并记录审计日志。
- 外部 API 超时：重试并写入失败事件。
- 回写评论失败：不影响本地状态推进，但必须告警。
