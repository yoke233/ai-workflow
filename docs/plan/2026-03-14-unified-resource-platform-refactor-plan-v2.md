# Unified Resource Platform Refactor Plan v2

> 状态：建议稿
> 日期：2026-03-14
> 目的：作为未来规范方向，替代分裂的“Resource Space Platform Refactor Plan”与“Thread Deliverable & Asset Store”两条建模路线

## 1. 为什么要重写

当前仓库已经暴露出三个长期问题：

- `ResourceBinding` 同时承担项目资源空间、附件对象、Action I/O 绑定三种职责。
- `Run.ResultMarkdown` / `Run.ResultAssets` 仍然是执行产物的主要事实源。
- Thread / Chat / Work Item / Run 的文件资产没有统一对象模型。
- 系统缺少“交付容器”概念，无法把“文档 + 文件”表达为一次并列交付。

原计划方向是对的，但有几个关键问题需要修正：

- 把 `Workspace` 也算进 `ResourceSpace`，会再次混淆运行时临时目录和外部持久资源。
- 同时存在 `action_output` 和 `run_output` 两套事实语义，边界不清。
- `ResourceSpace` / `ResourceObject` 字段有双真相源风险，比如 `root_uri` 与 `bucket/prefix`。
- 与 `AssetStore + assets://` 方案形成双轨模型。

因此需要一份新的统一计划，明确哪些是：

- 外部可寻址空间
- 持久对象
- 业务引用
- 执行声明
- 运行事实

## 2. 目标

### 核心目标

- 建立统一的资源领域模型，覆盖附件、执行产物、Thread 资产、聊天文件。
- 建立统一的交付批次模型，支持同一个 Thread / Run 下存在多次交付。
- 明确区分“路径空间”和“持久对象”。
- 明确区分“Action I/O 声明”和“Run 执行事实”。
- 让 Git / local_fs / S3 / HTTP / WebDAV 等资源访问统一归入一套空间模型。
- 让对象存储后端成为基础设施能力，而不是额外一套业务模型。

### 非目标

- 不做内容去重和版本树。
- 不在第一阶段解决所有权限模型。
- 不保留旧模型的长期兼容写路径。

## 3. 设计原则

### 3.1 一个概念只表达一件事

- `ResourceSpace` 只表达“外部路径空间”。
- `ResourceObject` 只表达“持久对象”。
- `ResourceRef` 只表达“业务归属关系”。
- `DeliveryBatch` 只表达“一次交付批次”。
- `ActionResourceSpec` 只表达“执行前后的 I/O 声明”。
- `Run` 才是执行事实与产物事实的归属。

### 3.2 Workspace 不是 ResourceSpace

- `Workspace` / worktree / sandbox 根目录是运行时临时环境。
- 它可以由某个 `ResourceSpace` 物化而来，但自身不是领域资源。
- 运行结束后它可以被销毁；`ResourceSpace` 和 `ResourceObject` 不应有这种生命周期。

### 3.3 产物归属于 Run，不归属于 Action

- `Action` 代表模板或编排节点。
- `Run` 代表某次执行尝试。
- 真正生成的文件、报告、截图、压缩包，都应挂在 `Run` 下。

### 3.4 资产模型必须收敛成一套

- 旧 `AssetStore` 方案不再作为独立业务模型存在。
- 它吸收进统一资源模型，降级为底层对象存储接口。
- `assets://` 是否保留，只作为底层 locator 形式，不再单独主导领域建模。

### 3.5 交付容器与资源对象分离

- `DeliveryBatch` 不是文件对象。
- Markdown 文档本身也是文件对象。
- 文档和文件在交付时通过 `ResourceRef` 成组出现，而不是“文档下挂附件”。

## 4. 新领域模型

### 4.1 ResourceSpace

表示一个可按路径读取/写入的外部资源空间。

典型例子：

- Git 仓库
- 本地目录
- S3 bucket/prefix
- HTTP base URL
- WebDAV 根目录

建议字段：

```go
type ResourceSpace struct {
    ID        int64
    Kind      string         // "git" | "local_fs" | "s3" | "http" | "webdav"
    RootURI   string         // 唯一定位真相源
    Label     string
    Config    map[string]any // provider-specific config
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

关键决策：

- `RootURI` 是定位真相源。
- 第一阶段不单独引入 `bucket/prefix`、`bucket/key` 结构化列，避免双真相源。
- provider-specific 解析结果先放 `Config`，待真的需要索引和查询再拆结构化字段。

### 4.2 ProjectResourceSpaceLink

项目与资源空间是关联关系，而不是把 `ProjectID` 硬塞进 `ResourceSpace`。

```go
type ProjectResourceSpaceLink struct {
    ID              int64
    ProjectID       int64
    ResourceSpaceID int64
    Role            string // "primary_repo" | "repo" | "shared_drive" | "knowledge_base" | ...
    Label           string
    Metadata        map[string]any
    CreatedAt       time.Time
}
```

这样可以天然支持：

- 一个项目关联多个空间
- 一个空间被多个项目共享
- 同一个空间在不同项目里拥有不同业务角色

### 4.3 ResourceObject

表示一个持久文件对象，而不是路径空间中的某个抽象路径。

```go
type ResourceObject struct {
    ID             int64
    StorageBackend string         // "local" | "s3" | "gcs" | "oss" | ...
    Locator        string         // 对象真实定位符，作为唯一定位真相源
    Size           int64
    MIMEType       string
    Checksum       string
    Metadata       map[string]any
    SourceType     string         // "run" | "work_item_attachment" | "thread_deliverable" | "chat_message" | "import" | ...
    SourceID       string
    CreatedAt      time.Time
    UpdatedAt      time.Time
}
```

关键决策：

- 不使用 `CreatedByRunID` 这种过窄字段，改用通用 `SourceType + SourceID`。
- `Locator` 是对象定位真相源。
- `StorageBackend` 表示对象实际落在哪种对象存储实现上。

### 4.4 ResourceRef

表示业务对象对资源对象的引用关系。

```go
type ResourceRef struct {
    ID               int64
    ResourceObjectID int64
    OwnerType        string // "work_item" | "run" | "thread_deliverable" | "chat_message"
    OwnerID          int64
    Role             string // 见下方受控枚举
    Label            string
    Metadata         map[string]any
    CreatedAt        time.Time
}
```

受控 `Role` 枚举建议：

- `attachment`
- `primary_output`
- `supporting_output`
- `evidence`
- `embedded_asset`
- `input_snapshot`

关键决策：

- 禁止使用自由扩张的 role 命名。
- `OwnerType + Role` 必须有白名单组合。
- `run_output` 和 `action_output` 二选一后，统一改成 `Run + primary_output/supporting_output`。

### 4.5 ActionResourceSpec

表示 Action 的输入输出声明。它不是产物事实。

```go
type ActionResourceSpec struct {
    ID          int64
    ActionID    int64
    Direction   string // "input" | "output"
    Kind        string // "object_ref" | "space_path"
    ObjectRefID *int64
    SpaceID     *int64
    Path        string
    Required    bool
    MediaType   string
    Description string
    Metadata    map[string]any
    CreatedAt   time.Time
}
```

说明：

- `object_ref` 用于消费一个已存在的对象引用。
- `space_path` 用于从外部空间按路径读取或写回。
- 输出声明只是“期望发布到哪里”，不代表已经生成。

### 4.6 DeliveryBatch

`DeliveryBatch` 表示一次可以被别人消费的交付批次。

它的核心语义不是“文件集合”，而是：

- 这是 Thread / Run / WorkItem 下面的一次正式交付
- 同一个 owner 下面允许存在多次交付
- 每次交付由一组 `ResourceRef` 组成

```go
type DeliveryBatch struct {
    ID        int64
    OwnerType string         // "thread" | "work_item" | "run"
    OwnerID   int64
    Kind      string         // "spec_package" | "review_package" | "handoff" | "run_output"
    Sequence  int            // 1, 2, 3...
    Title     string
    Summary   string
    Status    string         // "draft" | "published" | "archived"
    Metadata  map[string]any
    CreatedBy string
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

关键决策：

- `Sequence` 用于表达同一 owner 下的第几次交付
- 一个 `DeliveryBatch` 下面通常有一个 `primary_document`
- 其他文件通过 `ResourceRef` 并列挂在该批次下面

### 4.7 关系树图

下面这棵树用于说明主要实体之间的关系边界：

```text
Project
  └── ProjectResourceSpaceLink[]
      ├── role = primary_repo
      │   └── ResourceSpace(kind=git)
      ├── role = shared_drive
      │   └── ResourceSpace(kind=local_fs | s3 | webdav)
      └── role = knowledge_base
          └── ResourceSpace(kind=http | s3 | webdav)

WorkItem
  ├── Action[]
  │   ├── ActionResourceSpec(direction=input, kind=object_ref)
  │   │   └── ResourceRef(role=input_snapshot)
  │   │       └── ResourceObject
  │   ├── ActionResourceSpec(direction=input, kind=space_path)
  │   │   └── ResourceSpace + path
  │   └── ActionResourceSpec(direction=output, kind=space_path | object_ref)
  └── ResourceRef(role=attachment)
      └── ResourceObject

Run
  ├── belongs to Action
  ├── ResultMarkdown(summary only)
  └── DeliveryBatch(kind=run_output, seq=1..N)
      ├── ResourceRef(role=primary_document)
      │   └── ResourceObject(.md)
      ├── ResourceRef(role=supporting_file)
      │   └── ResourceObject
      └── ResourceRef(role=evidence)
          └── ResourceObject

Thread
  ├── DeliveryBatch(kind=spec_package, seq=1)
  │   ├── ResourceRef(role=primary_document)
  │   │   └── ResourceObject(.md)
  │   └── ResourceRef(role=supporting_file)
  │       └── ResourceObject
  └── DeliveryBatch(kind=review_package, seq=2)
      ├── ResourceRef(role=primary_document)
      │   └── ResourceObject(.md)
      └── ResourceRef(role=evidence)
          └── ResourceObject

ChatMessage
  └── ResourceRef(role=attachment | embedded_asset)
      └── ResourceObject

Workspace / Worktree / Sandbox
  └── runtime-only
      ├── may materialize from ResourceSpace + path
      ├── may stage ResourceObject as local files
      └── is NOT a ResourceSpace
```

这棵树强调四件事：

- 项目关联的是 `ResourceSpace`，不是直接关联对象。
- `DeliveryBatch` 是交付批次容器，Markdown 文档和其他文件都作为 `ResourceObject` 进入交付。
- 交付内的条目语义通过 `ResourceRef.role` 表达，不再单独引入 `DeliveryItem`。
- `WorkItem`、`ChatMessage` 等仍可直接通过 `ResourceRef` 引用对象。
- `ActionResourceSpec` 是声明，不是事实。
- `Workspace` 是运行时容器，不进入持久资源领域模型。

## 5. 服务边界

### 5.1 ResourceObjectService

负责对象注册、引用创建、打开下载、删除策略。

```go
type ResourceObjectService interface {
    RegisterObject(ctx context.Context, in RegisterObjectInput) (*core.ResourceObject, error)
    CreateRef(ctx context.Context, in CreateResourceRefInput) (*core.ResourceRef, error)
    GetObject(ctx context.Context, objectID int64) (*core.ResourceObject, error)
    GetRef(ctx context.Context, refID int64) (*core.ResourceRef, error)
    ListRefs(ctx context.Context, ownerType string, ownerID int64) ([]*core.ResourceRef, error)
    OpenObject(ctx context.Context, objectID int64) (io.ReadCloser, *OpenedObjectInfo, error)
    DeleteRef(ctx context.Context, refID int64) error
}
```

规则：

- 删除 `ResourceRef` 不等于删除对象。
- 对象只有在没有引用且满足清理策略时才物理删除。

### 5.2 ResourceSpaceService

负责路径空间的读取、物化、写回。

```go
type ResourceSpaceService interface {
    GetSpace(ctx context.Context, id int64) (*core.ResourceSpace, error)
    MaterializePath(ctx context.Context, spaceID int64, path string, workDir string) (*MaterializedPath, error)
    WritePath(ctx context.Context, spaceID int64, path string, localFile string) (*WrittenPath, error)
    StatPath(ctx context.Context, spaceID int64, path string) (*ResourcePathInfo, error)
}
```

关键决策：

- 用 `WritePath`，不用歧义更大的 `Collect`。
- 它不处理附件、产物、聊天语义，只处理空间与路径。

### 5.3 ActionIOService

负责编排 Action 执行前后的 I/O。

```go
type ActionIOService interface {
    PrepareInputs(ctx context.Context, actionID int64, workDir string) (*PreparedActionIO, error)
    PublishRunOutputs(ctx context.Context, actionID int64, runID int64, workDir string) (*PublishedRunOutputs, error)
}
```

规则：

- 输入准备时，解析 `ActionResourceSpec`。
- 若是 `object_ref`，通过 `ResourceObjectService` 物化。
- 若是 `space_path`，通过 `ResourceSpaceService` 物化。
- 执行后发布的对象统一挂到 `Run`，而不是 `Action`。

### 5.4 ObjectStore

这是底层基础设施接口，用于代替旧 `AssetStore` 的定位。

```go
type ObjectStore interface {
    Put(ctx context.Context, name string, r io.Reader, mediaType string) (locator string, size int64, checksum string, err error)
    Open(ctx context.Context, locator string) (io.ReadCloser, error)
    Delete(ctx context.Context, locator string) error
}
```

说明：

- 它是基础设施，不是业务模型。
- 本地文件系统、S3、OSS、GCS 都实现它。
- `ResourceObjectService` 通过它落对象。

### 5.5 DeliveryBatchService

负责交付批次的创建与查询，以及把对象加入某次交付。

```go
type DeliveryBatchService interface {
    CreateBatch(ctx context.Context, in CreateDeliveryBatchInput) (*core.DeliveryBatch, error)
    AttachObject(ctx context.Context, batchID int64, objectID int64, in AttachBatchObjectInput) (*core.ResourceRef, error)
    GetBatch(ctx context.Context, batchID int64) (*core.DeliveryBatchView, error)
    ListBatches(ctx context.Context, ownerType string, ownerID int64) ([]*core.DeliveryBatch, error)
}
```

规则：

- `DeliveryBatch` 只表达一次交付，不直接承载大文本和二进制。
- 文档内容通过 Markdown 文件对象表达。
- 文件内容放在 `ResourceObject`。
- 同一个 owner 下可以有多个 batch。

## 6. 运行时与事实模型

### 6.1 运行前

1. 读取 `ActionResourceSpec`
2. `object_ref` 输入：
   - 解析到 `ResourceObject`
   - 物化到工作目录
3. `space_path` 输入：
   - 通过 `ResourceSpaceService.MaterializePath` 拉到工作目录
4. 返回本地输入清单给执行器

### 6.2 运行后

1. 扫描声明的输出
2. 将生成文件注册为 `ResourceObject`
3. 如需要对外交付，则创建 `DeliveryBatch(owner_type=run, kind=run_output)`
4. 将说明性文本写成 Markdown 文件对象，并以 `ResourceRef(role=primary_document)` 挂到 batch
5. 将文件产物以 `ResourceRef(role=supporting_file | evidence)` 挂到 batch
6. 如声明要求写回路径空间，则调用 `ResourceSpaceService.WritePath`
7. `Run.ResultMarkdown` 仅保留摘要，不再承载完整产物事实

## 7. 数据库改造

### 新增表

- `resource_spaces`
- `project_resource_space_links`
- `resource_objects`
- `resource_refs`
- `delivery_batches`
- `action_resource_specs`

### 保留但弱化的旧表/字段

- `resource_bindings`：迁移期读取后彻底废弃
- `executions.result_assets`：迁移后废弃
- `issues.resource_binding_id`：改为指向 project-space link 或新的 repo selection 关系，最终移除

### 索引建议

- `resource_refs(owner_type, owner_id, role)`
- `resource_refs(resource_object_id)`
- `project_resource_space_links(project_id, role)`
- `action_resource_specs(action_id, direction)`
- `resource_objects(source_type, source_id)`

## 8. API 规范调整

### 8.1 Work Item 附件

保留业务语义：

- `POST /work-items/{id}/attachments`
- `GET /attachments/{id}`
- `GET /attachments/{id}/download`
- `DELETE /attachments/{id}`

内部实现改为：

- 上传文件 -> `ResourceObject`
- 建立 `ResourceRef(owner_type=work_item, role=attachment)`

### 8.2 Run / Execution 产物

统一改为由 `DeliveryBatch(owner_type=run, kind=run_output)` 组装。

其中：

- 说明文档 -> `ResourceRef(role=primary_document)`，对应 `.md` 文件对象
- 文件产物 -> `ResourceRef(role=supporting_file | evidence)`

### 8.3 Thread 交付

新增或改造：

- `POST /threads/{id}/delivery-batches`
- `POST /delivery-batches/{id}/items`
- `GET /delivery-batches/{id}`

说明：

- Thread 下可以有多次交付批次
- 每次交付批次里可以同时看到“一个主文档”和“多个文件”
- 这些内容通过同一组 `ResourceRef` 表达

### 8.4 项目资源空间

主接口建议：

- `GET /projects/{id}/resource-spaces`
- `POST /projects/{id}/resource-spaces`
- `DELETE /project-resource-space-links/{id}`

说明：

- 项目关联的是 `ProjectResourceSpaceLink`
- 不是把 `ProjectID` 内嵌在 `ResourceSpace`

## 9. 实施阶段

### Phase 0：冻结旧方向，统一方案

- 将本计划设为统一资源规范的唯一主文档
- 明确旧 `AssetStore` 方案吸收为 `ObjectStore`
- 明确旧资源平台计划不再继续演化

### Phase 1：先落对象模型

- 新增 `resource_objects` / `resource_refs`
- 新增 `ObjectStore`
- 将 Work Item 附件改到新模型
- 将 `Run.ResultAssets` 改到新模型

验收：

- 新附件不再写 `resource_bindings(kind=attachment)`
- 新执行产物不再以内联数组为事实源

### Phase 2：接入 DeliveryBatch

- 新增 `delivery_batches`
- Thread 交付接入 `DeliveryBatch + ResourceRef + ResourceObject`
- Work Item briefing 能消费 Thread 下某次交付批次的 Markdown 文档对象与交付文件

验收：

- Thread / Work Item / Run 使用同一套交付模型和对象模型

### Phase 3：重建资源空间模型

- 新增 `resource_spaces` / `project_resource_space_links`
- 将 Git / local_fs / S3 / HTTP / WebDAV 迁移到 `ResourceSpace`
- workspace preparation 改为通过 `ProjectResourceSpaceLink` 选择 repo 空间

验收：

- `ResourceBinding` 不再是主资源模型
- 工作区来源和项目空间关系可清晰追踪

### Phase 4：重建 Action I/O

- `action_resources` 重构为 `action_resource_specs`
- 支持 `object_ref` / `space_path`
- 执行器接入 `ActionIOService`

验收：

- Action 能同时消费对象输入和路径输入
- Run 输出始终挂在 `Run`，不挂在 `Action`

### Phase 5：删除旧模型

- 删除 `ResourceBinding(kind=attachment)` 路径
- 删除 `Run.ResultAssets` 主逻辑
- 删除旧 `ActionResource` 绑定逻辑
- 评估并移除 `issues.resource_binding_id`

## 10. Implementation Checklist

### 10.1 Phase 0：方案冻结与收口

- [ ] 将本文件设为统一资源重构的唯一主文档
- [ ] 在团队内确认废弃旧 `AssetStore` 业务建模路线
- [ ] 在团队内确认 `Workspace != ResourceSpace`
- [ ] 在团队内确认 `Run` 是产物事实唯一归属
- [ ] 在团队内确认 `Action` 只保留 I/O 声明

### 10.2 Phase 1：对象模型与附件/产物落地

- [ ] 新增 `internal/core/resource_object.go`
- [ ] 新增 `internal/core/resource_ref.go`
- [ ] 新增 `internal/core/object_store.go`
- [ ] 新增 SQLite 表 `resource_objects`
- [ ] 新增 SQLite 表 `resource_refs`
- [ ] 实现 `ResourceObjectStore` / `ResourceRefStore`
- [ ] 实现本地 `ObjectStore`
- [ ] 将 Work Item 附件上传改为创建 `ResourceObject + ResourceRef`
- [ ] 将附件下载改为通过 `ResourceObjectService` 打开对象
- [ ] 将 `Run.ResultAssets` 改为从 `ResourceRef(owner_type=run)` 读取
- [ ] 保留 `Run.ResultMarkdown`，但改成摘要字段语义
- [ ] 为附件与 run 产物补单元测试和集成测试

### 10.3 Phase 2：DeliveryBatch 接入

- [ ] 新增 `internal/core/delivery_batch.go`
- [ ] 新增 SQLite 表 `delivery_batches`
- [ ] 实现 `DeliveryBatchStore`
- [ ] 实现 `DeliveryBatchService`
- [ ] 新增 `POST /threads/{id}/delivery-batches`
- [ ] 新增 `POST /delivery-batches/{id}/items`
- [ ] 让 Thread 支持多次交付批次
- [ ] 让每次交付批次支持“主 markdown 文档 + 多文件”组合
- [ ] 让 Work Item briefing 可以拉取 Thread 某次交付批次中的 markdown 文档和文件
- [ ] 为 DeliveryBatch / 交付流转补测试

### 10.4 Phase 3：ResourceSpace 与项目关联模型

- [ ] 新增 `internal/core/resource_space.go`
- [ ] 新增 `internal/core/project_resource_space_link.go`
- [ ] 新增 SQLite 表 `resource_spaces`
- [ ] 新增 SQLite 表 `project_resource_space_links`
- [ ] 实现 `ResourceSpaceStore`
- [ ] 实现 `ProjectResourceSpaceLinkStore`
- [ ] 将 Git / local_fs / S3 / HTTP / WebDAV 统一注册到 `ResourceSpaceService`
- [ ] 新增 `/projects/{id}/resource-spaces` 主接口
- [ ] 重构 workspace selection，使其依赖 `ProjectResourceSpaceLink`
- [ ] 补资源空间 CRUD、路径物化、路径写回测试

### 10.5 Phase 4：Action I/O 重建

- [ ] 新增 `internal/core/action_resource_spec.go`
- [ ] 新增 SQLite 表 `action_resource_specs`
- [ ] 实现 `ActionResourceSpecStore`
- [ ] 实现 `ActionIOService`
- [ ] 支持 `object_ref` 输入
- [ ] 支持 `space_path` 输入
- [ ] 支持 `space_path` 输出写回
- [ ] 让执行器在运行前通过 `PrepareInputs` 准备输入
- [ ] 让执行器在运行后通过 `PublishRunOutputs` 发布输出
- [ ] 删除旧 `ActionResource` 解析主路径
- [ ] 补 Action I/O 单元测试和端到端测试

### 10.6 Phase 5：旧模型删除

- [ ] 删除 `ResourceBinding(kind=attachment)` 创建逻辑
- [ ] 删除附件对 `resource_bindings` 的读取主逻辑
- [ ] 删除 `executions.result_assets` 的主逻辑依赖
- [ ] 删除旧 `action_resources` API 和存储逻辑
- [ ] 迁移并移除 `issues.resource_binding_id`
- [ ] 清理前端旧类型与 API client 字段
- [ ] 清理旧文档、旧测试、旧命名

### 10.7 Cross-cutting Checklist

- [ ] 为 `OwnerType + Role` 建立白名单校验
- [ ] 为 `ResourceObject` 加入删除保护与引用计数策略
- [ ] 为对象下载和附件下载统一鉴权策略
- [ ] 为对象上传和路径写回补审计日志
- [ ] 为 SQLite migration 制定明确迁移脚本
- [ ] 更新 `web/src/types/apiV2.ts`
- [ ] 更新前端页面对附件、产物、DeliveryBatch 的类型消费
- [ ] 补回归测试矩阵

## 11. 代码落点建议

- `internal/core/resource_space.go`
- `internal/core/resource_object.go`
- `internal/core/resource_ref.go`
- `internal/core/delivery_batch.go`
- `internal/core/action_resource_spec.go`
- `internal/core/object_store.go`
- `internal/application/resource/object_service.go`
- `internal/application/resource/space_service.go`
- `internal/application/resource/action_io_service.go`
- `internal/application/delivery_batch/service.go`
- `internal/adapters/store/sqlite/*`
- `internal/adapters/http/workitem_attachment.go`
- `internal/adapters/http/artifact.go`
- `internal/adapters/http/thread*.go`
- `internal/application/flow/engine.go`
- `internal/application/flow/resource_resolver.go`
- `internal/adapters/workspace/provider/*`
- `web/src/types/apiV2.ts`

## 12. 验收标准

- 系统里只存在一套资源对象模型。
- 系统里存在统一交付批次模型，支持同一 owner 下多次交付。
- `Workspace` 与 `ResourceSpace` 在模型和代码上明确分离。
- `Run` 是产物事实唯一归属。
- `Action` 只保留 I/O 声明，不再持有产物事实。
- Thread、Work Item、Run、Chat 均可复用同一类 `ResourceObject`。
- Git / local_fs / S3 / HTTP / WebDAV 都能通过统一空间服务访问。

## 13. 最终决策摘要

- 要做统一资源平台，而且应该做。
- 要引入统一交付批次容器，支持同一 owner 下多次交付。
- 不再把 `Workspace` 建模为 `ResourceSpace`。
- 不再保留独立的 `AssetStore` 业务模型。
- 不再使用 `CreatedByRunID` 这类过窄来源字段。
- 不再使用 `action_output` / `run_output` 并存语义。
- `Run` 是产物事实层，`Action` 是声明层。
