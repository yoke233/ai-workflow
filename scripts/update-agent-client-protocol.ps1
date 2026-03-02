<#
.SYNOPSIS
兼容入口：脚本已重命名为 update-github-repo.ps1。

.DESCRIPTION
此文件仅保留向后兼容。新功能（例如更多 GitHub 仓库写法支持）请使用：
  scripts/update-github-repo.ps1
#>
$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$newScript = Join-Path -Path $PSScriptRoot -ChildPath "update-github-repo.ps1"
if (-not (Test-Path -LiteralPath $newScript)) {
  throw "未找到新脚本：$newScript"
}

Write-Warning "scripts/update-agent-client-protocol.ps1 已重命名，请改用 scripts/update-github-repo.ps1。当前调用将自动转发。"
& $newScript @args
exit $LASTEXITCODE
