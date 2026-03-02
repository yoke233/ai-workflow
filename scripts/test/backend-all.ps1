[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. (Join-Path $PSScriptRoot "common.ps1")

$repoRoot = Enter-RepoRoot -ScriptRoot $PSScriptRoot
Set-SafeTestEnvironment

Write-Host "RepoRoot: $repoRoot"
Write-Host "GOMAXPROCS=$env:GOMAXPROCS, GOTEST_TIMEOUT=$env:GOTEST_TIMEOUT"

Invoke-Step -Name "Backend all packages (go test ./...)" -CheckLastExitCode -Command {
    go test -p 4 -timeout $env:GOTEST_TIMEOUT ./...
}

