# OpenViking 学习目录（ai-workflow）

最后更新：2026-03-03

本目录用于沉淀 OpenViking 在 `ai-workflow` 中的学习记录与实操手册。  
当前采用“先简单、可运行、可扩展”的策略：

- 不按角色拆目录；
- `secretary` 负责记忆写入；
- `worker/reviewer` 只读。

## 项目级配置约定（默认）

OpenViking 视为 `ai-workflow` 的配套组件，默认使用**项目级配置**：

- `D:/project/ai-workflow/.runtime/openviking/ov.conf`（本地私有，不入库）
- `D:/project/ai-workflow/.runtime/openviking/ovcli.conf`（本地私有，不入库）

仓库内只保留模板：

- `configs/openviking/ov.conf.example`
- `configs/openviking/ovcli.conf.example`

## 文档导航

1. [01-install-and-start-windows.zh-CN.md](./01-install-and-start-windows.zh-CN.md)  
   Windows 本机安装与启动（最小闭环）

2. [02-config-template.zh-CN.md](./02-config-template.zh-CN.md)  
   `ov.conf` / `ovcli.conf` 配置模板与字段说明

3. [03-search-runbook.zh-CN.md](./03-search-runbook.zh-CN.md)  
   资料导入、目录浏览、搜索与排错手册

4. [04-ai-workflow-minimal-integration.zh-CN.md](./04-ai-workflow-minimal-integration.zh-CN.md)  
   与 `ai-workflow` 的最小集成方案（先不复杂化角色目录）

5. [05-live-test-params.zh-CN.md](./05-live-test-params.zh-CN.md)  
   联调与真实测试所需参数清单（你填参数，我执行真实测试）

## 本目录约定

- 文档以实操为主，优先给可执行命令和最小配置。
- 先保证“能跑起来 + 有检索结果”，再优化目录和策略。
- 未来如果记忆冲突明显，再增量拆分 `by-role/{role}` 目录，不提前设计过深层级。
