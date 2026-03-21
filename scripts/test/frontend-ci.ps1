[CmdletBinding()]
param(
    [switch]$WithE2E
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. (Join-Path $PSScriptRoot "common.ps1")

$repoRoot = Enter-RepoRoot -ScriptRoot $PSScriptRoot

Write-Host "RepoRoot: $repoRoot"

Invoke-Step -Name "Frontend local CI baseline" -Command {
    & (Join-Path $PSScriptRoot "frontend-lint.ps1")
    & (Join-Path $PSScriptRoot "frontend-unit.ps1")
    & (Join-Path $PSScriptRoot "frontend-build.ps1")
}

if ($WithE2E) {
    Invoke-Step -Name "Frontend browser E2E" -Command {
        & (Join-Path $PSScriptRoot "frontend-e2e.ps1")
    }
}
