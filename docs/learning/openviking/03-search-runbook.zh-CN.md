# OpenViking 搜索与资料导入 Runbook

最后更新：2026-03-03

本文给出“导入资料 -> 浏览目录 -> 搜索”的最短操作路径。

## 1. 状态检查

```powershell
ov status
```

如果报连接失败，先确认 `openviking-server` 正在运行。

## 2. 导入资料

导入 GitHub 仓库（示例）：

```powershell
ov add-resource https://github.com/volcengine/OpenViking --wait
```

导入本地目录（示例）：

```powershell
ov add-resource D:/project/ai-workflow --wait
```

## 3. 浏览资源树

```powershell
ov ls viking://resources/
ov tree viking://resources/ -L 2
```

## 4. 关键检索命令

简单搜索：

```powershell
ov find "OpenViking 是什么"
```

复杂搜索（建议配合范围）：

```powershell
ov search "ai-workflow 里 secretary 和记忆策略如何做"
```

## 5. 结合范围的检索建议

为了减少噪声，优先缩小范围：

- 公司/平台规范：`viking://resources/shared/`
- 项目资料：`viking://resources/projects/{project_id}/`
- 项目记忆：`viking://memory/projects/{project_id}/`

## 6. 失败排查

1. 搜索无结果
- 先检查是否已完成导入；
- 再检查搜索范围是否过窄；
- 最后检查 embedding/vlm 配置是否有效。

2. 结果偏离主题
- 收窄 `target_uri`；
- 优先使用 `find` 做精确检索；
- 对复杂问题再用 `search`。

3. 性能慢
- 减少导入噪声目录；
- 优化并发与模型参数；
- 初期先跑关键项目，再扩展全量资料。
