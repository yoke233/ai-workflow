# OpenViking 项目级配置模板

本目录提供 `ai-workflow` 使用 OpenViking 的模板文件：

- `docker-compose.example.yml`
- `ov.conf.example`
- `ovcli.conf.example`

## 快速使用

1. 复制模板到项目运行目录（本地私有，不入库）：

```powershell
New-Item -ItemType Directory -Force .runtime/openviking | Out-Null
Copy-Item configs/openviking/ov.conf.example .runtime/openviking/ov.conf
Copy-Item configs/openviking/ovcli.conf.example .runtime/openviking/ovcli.conf
```

2. 填写 `.runtime/openviking/ov.conf` 的模型参数。

3. 启动 OpenViking（在 `configs/openviking` 目录）：

```powershell
docker compose -f docker-compose.example.yml up -d
```

4. 探活：

```powershell
go run ./cmd/viking probe --base-url http://127.0.0.1:1933 --timeout 3s
```

