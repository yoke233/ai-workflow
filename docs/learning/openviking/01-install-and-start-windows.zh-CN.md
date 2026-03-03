# OpenViking Windows 安装与启动（最小闭环）

最后更新：2026-03-03

本文目标：在本机完成以下闭环。

1. 安装 `openviking` 包；
2. 启动 `openviking-server`；
3. 用 `ov` CLI 做状态检查与资料搜索。

## 1. 前置条件

- Windows PowerShell
- Python 3.10+（建议 3.12）
- 可访问模型提供方 API（用于 embedding/VLM）

## 2. 创建虚拟环境并安装

```powershell
py -3.12 -m venv .venv
.\.venv\Scripts\Activate.ps1

python -m pip install --upgrade pip
pip install openviking
```

## 3. 准备项目级配置目录

```powershell
New-Item -ItemType Directory -Force "D:\project\ai-workflow\.runtime\openviking" | Out-Null
New-Item -ItemType Directory -Force "D:\project\ai-workflow\.runtime\openviking\data" | Out-Null
```

## 4. 写配置文件

推荐从项目模板复制：

```powershell
Copy-Item D:\project\ai-workflow\configs\openviking\ov.conf.example D:\project\ai-workflow\.runtime\openviking\ov.conf
Copy-Item D:\project\ai-workflow\configs\openviking\ovcli.conf.example D:\project\ai-workflow\.runtime\openviking\ovcli.conf
```

配置说明见：`02-config-template.zh-CN.md`

## 5. 设置环境变量

```powershell
$env:OPENVIKING_CONFIG_FILE = "D:/project/ai-workflow/.runtime/openviking/ov.conf"
```

可选：配置 CLI 文件路径。

```powershell
$env:OPENVIKING_CLI_CONFIG_FILE = "D:/project/ai-workflow/.runtime/openviking/ovcli.conf"
```

## 6. 启动服务（两种方式）

方式 A：直接启动二进制（本地 Python 环境）

```powershell
openviking-server
```

如果命令不在 PATH 中，使用：

```powershell
.\.venv\Scripts\openviking-server.exe
```

方式 B：按项目 docker compose 启动（推荐与你的 infra 容器一起管理）

```powershell
cd D:\project\ai-workflow\configs\openviking
docker compose -f docker-compose.example.yml up -d
```

## 7. 基础验证

新开一个 PowerShell，激活同一个 `.venv`，执行：

```powershell
ov status
ov ls viking://resources/
```

若需要 HTTP 方式快速探测（在 `ai-workflow` 仓库）：

```powershell
go run ./cmd/viking probe --base-url http://127.0.0.1:8088 --timeout 3s
```

## 8. 常见问题

1. `openviking-server` 找不到  
   先确认激活了 `.venv`，再用 `.\.venv\Scripts\openviking-server.exe` 直接执行。

2. `probe` 报 `connect refused`  
   表示服务未启动，或端口不是你配置的端口。检查服务日志与实际监听地址。

3. `ov` 命令报配置错误  
   检查 `OPENVIKING_CONFIG_FILE` 是否指向正确文件，JSON 是否合法。
