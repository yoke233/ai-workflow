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

Invoke-Step -Name "Backend full test" -Command {
    & (Join-Path $PSScriptRoot "backend-all.ps1")
}

Invoke-Step -Name "Backend GitHub integration suite" -Command {
    & (Join-Path $PSScriptRoot "backend-github.ps1")
}

Invoke-Step -Name "Frontend unit suite" -Command {
    & (Join-Path $PSScriptRoot "frontend-unit.ps1")
}

Invoke-Step -Name "Frontend production build" -Command {
    & (Join-Path $PSScriptRoot "frontend-build.ps1")
}

Write-Host ""
Write-Host "P3 integration test suite completed." -ForegroundColor Green

