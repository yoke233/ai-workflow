<#
.SYNOPSIS
同步 A2A 官方协议仓库，并更新本仓库 vendor 协议文档。

.DESCRIPTION
这是 scripts/update-github-repo.ps1 的 A2A 专用包装器，默认参数：
- Repository: a2aproject/A2A
- DocsSourcePath: docs
- CopyDocsTo: docs/vendor/a2a-protocol-upstream-docs

可通过参数覆盖默认值，并透传常用控制项（Ref、Force、Depth1、TargetPath、BaseDir）。
#>
[CmdletBinding()]
param(
  [string]$Repository = "a2aproject/A2A",
  [string]$TargetPath = "",
  [string]$Ref = "",
  [switch]$Force,
  [switch]$Depth1,
  [string]$CopyDocsTo = "docs/vendor/a2a-protocol-upstream-docs",
  [string]$DocsSourcePath = "docs",
  [string]$BaseDir = ""
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$syncScript = Join-Path -Path $PSScriptRoot -ChildPath "update-github-repo.ps1"
if (-not (Test-Path -LiteralPath $syncScript)) {
  throw "未找到依赖脚本：$syncScript"
}

$forward = @{
  Repository = $Repository
  DocsSourcePath = $DocsSourcePath
  CopyDocsTo = $CopyDocsTo
}

if ($PSBoundParameters.ContainsKey("TargetPath")) {
  $forward.TargetPath = $TargetPath
}
if ($PSBoundParameters.ContainsKey("Ref")) {
  $forward.Ref = $Ref
}
if ($PSBoundParameters.ContainsKey("BaseDir")) {
  $forward.BaseDir = $BaseDir
}
if ($Force) {
  $forward.Force = $true
}
if ($Depth1) {
  $forward.Depth1 = $true
}

Write-Host "[a2a-protocol-sync] repository=$Repository"
Write-Host "[a2a-protocol-sync] docs=$DocsSourcePath -> $CopyDocsTo"
& $syncScript @forward
exit $LASTEXITCODE
