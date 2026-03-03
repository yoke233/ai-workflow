# OpenViking 配置模板（ov.conf / ovcli.conf）

最后更新：2026-03-03

## 1. `ov.conf` 模板（可直接改值）

项目级保存路径建议：`D:/project/ai-workflow/.runtime/openviking/ov.conf`

```json
{
  "storage": {
    "workspace": "D:/openviking_workspace"
  },
  "log": {
    "level": "INFO",
    "output": "stdout"
  },
  "embedding": {
    "dense": {
      "provider": "openai",
      "api_base": "https://api.openai.com/v1",
      "api_key": "YOUR_OPENAI_API_KEY",
      "model": "text-embedding-3-large",
      "dimension": 3072
    },
    "max_concurrent": 10
  },
  "vlm": {
    "provider": "openai",
    "api_base": "https://api.openai.com/v1",
    "api_key": "YOUR_OPENAI_API_KEY",
    "model": "gpt-4o",
    "max_concurrent": 50
  }
}
```

## 2. `ovcli.conf` 模板（可选）

项目级保存路径建议：`D:/project/ai-workflow/.runtime/openviking/ovcli.conf`

```json
{
  "url": "http://localhost:1933",
  "timeout": 60.0,
  "output": "table"
}
```

说明：

- `url` 是 CLI 默认连接地址，需与你服务实际地址一致。
- 若你实际监听 `8088`，请改成 `http://localhost:8088`。

## 3. 配置字段建议

1. `storage.workspace`
- 指向本地可读写目录，建议独立目录；
- 避免放在会频繁清理的临时目录中。

2. `embedding.dense`
- 用于向量检索；
- `dimension` 必须和模型匹配。

3. `vlm`
- 用于摘要/语义处理等模型能力；
- 可以和 embedding 使用不同模型。

4. 并发参数
- 初期保持默认或中等值，先求稳定；
- 观察资源占用后再调优。

## 4. 安全建议

- 不要把真实 `api_key` 提交到仓库；
- 建议用你本地私有配置文件 + 环境变量管理；
- 如果需要共享模板，保留占位符，不写真实密钥。
- 建议把真实配置放在 `.runtime/openviking`（仓库已忽略），模板留在 `configs/openviking`。
