# Legacy V1 Views

本目录保存的是旧 `/api/v1` UI 子树，对应 issue / run / chat / ops 这一代页面模型。

当前状态：

- 现行入口已经切到 `web/src/App.tsx` + `web/src/pages/*` + `V2WorkbenchContext`。
- 本目录页面不属于当前主工作台入口。
- 本目录仍依赖 `web/src/lib/apiClient.ts`、`/api/v1/*` 以及部分旧组件。

清理约定：

- 不再向本目录新增业务能力。
- 若需要实现新需求，优先在 `web/src/pages/*` 和 `/api/v2` 上完成。
- 只有在做 v1 兼容修复、迁移对照、或最终归档/删除时，才应修改本目录。
