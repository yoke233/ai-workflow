[CmdletBinding()]
param(
    [switch]$SkipTerminologyGate,
    [switch]$SkipGoTests
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. (Join-Path $PSScriptRoot "common.ps1")

$repoRoot = Enter-RepoRoot -ScriptRoot $PSScriptRoot
Set-SafeTestEnvironment

Write-Host "RepoRoot: $repoRoot"
Write-Host "Smoke target: buildable current baseline"
Write-Host "GOMAXPROCS=$env:GOMAXPROCS, GOTEST_TIMEOUT=$env:GOTEST_TIMEOUT"

if (-not $SkipTerminologyGate) {
    Invoke-Step -Name "Terminology gate (README + docs/spec)" -Command {
        $legacyPattern = '\\b(plan|plans|task|tasks|Run|Runs|dag|secretary)\\b'
        $hits = & rg -n --ignore-case $legacyPattern README.md docs/spec

        if ($LASTEXITCODE -eq 0) {
            Write-Host $hits
            throw "Legacy terminology found in README/docs/spec."
        }
        if ($LASTEXITCODE -gt 1) {
            throw "Failed to run terminology gate with rg."
        }

        Write-Host "Terminology gate passed."
    }
}

if (-not $SkipGoTests) {
    Invoke-Step -Name "Current backend build smoke" -CheckLastExitCode -Command {
        go build ./...
    }
}

Write-Host ""
Write-Host "Smoke completed." -ForegroundColor Green
