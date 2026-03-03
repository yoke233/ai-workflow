# 真实联调参数清单（你填我测）

最后更新：2026-03-03

你填完以下参数后，我可以做真实联调（连接 OpenViking、导入、检索、验证）。

## 1. 必填参数

1. OpenViking 服务地址
- 例如：`http://127.0.0.1:8088` 或 `http://127.0.0.1:1933`

2. 鉴权方式
- `none` 或 `api_key`
- 如果是 `api_key`，提供 key 或对应环境变量名

## 2. 模型参数（OpenViking 服务侧）

1. Embedding
- `provider`
- `api_base`
- `api_key`（或环境变量）
- `model`
- `dimension`

2. VLM/LLM
- `provider`
- `api_base`
- `api_key`（或环境变量）
- `model`

## 3. 可选参数

- `workspace` 本地路径（建议独立目录）
- `max_concurrent` 并发参数
- CLI 地址 `ovcli.conf.url`

## 4. 最小联调命令（你填参数后）

```powershell
# 0) 可选：容器启动（项目级）
cd D:\project\ai-workflow\configs\openviking
docker compose -f docker-compose.example.yml up -d

# 1) 服务探活
go run ./cmd/viking probe --base-url <YOUR_BASE_URL> --timeout 3s

# 2) 策略检查
go run ./cmd/viking plan --project demo-project --mode chat --role secretary

# 3) CLI 状态
ov status

# 4) 导入与搜索
ov add-resource https://github.com/volcengine/OpenViking --wait
ov find "OpenViking 是什么"
```

## 5. 联调通过标准

1. `probe` 至少一个健康端点返回 2xx。
2. `ov status` 正常返回服务状态。
3. `ov add-resource ... --wait` 可完成。
4. `ov find` 可返回有效结果。
