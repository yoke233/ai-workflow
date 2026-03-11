# Legacy UI Archive

该目录保存已从 `web/src` 迁出的前端 legacy UI 资产，目的是让当前主源码树只保留现行路由链。

当前归档范围：

- `src/views/*`
- `src/v2/*`
- `src/v3/*`
- `src/archive/legacy/*`
- `src/lib/a2aClient*`
- `src/types/a2a.ts`
- 仅被旧页面使用的 `src/components/*`
- 仅被旧页面使用的 `src/stores/*`

约定：

- 后续新功能只落在 `web/src/pages/*`、`web/src/contexts/*`、`web/src/lib/apiClientV2.ts` 及其配套主链路。
- 这里的文件仅用于兼容排查、迁移对照和最终删除前的历史留档。
- 若需要恢复某段旧逻辑，请先确认它不属于当前路由页面依赖链。
