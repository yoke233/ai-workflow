[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. (Join-Path $PSScriptRoot "common.ps1")

$repoRoot = Enter-RepoRoot -ScriptRoot $PSScriptRoot
Set-SafeTestEnvironment

Write-Host "RepoRoot: $repoRoot"
Write-Host "Run mode: sequential, no background jobs, no loops."
Write-Host "GOMAXPROCS=$env:GOMAXPROCS, GOTEST_TIMEOUT=$env:GOTEST_TIMEOUT"

Invoke-Step -Name "Wave4 V2 smoke baseline" -Command {
    & (Join-Path $PSScriptRoot "v2-smoke.ps1")
}

Write-Host ""
Write-Host "Wave4 integration baseline completed." -ForegroundColor Green
