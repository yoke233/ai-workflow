[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. (Join-Path $PSScriptRoot "common.ps1")

$repoRoot = Enter-RepoRoot -ScriptRoot $PSScriptRoot

Write-Host "RepoRoot: $repoRoot"

Invoke-Step -Name "Frontend lint" -CheckLastExitCode -Command {
    npm --prefix web run lint
}
