[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. (Join-Path $PSScriptRoot "common.ps1")

$repoRoot = Enter-RepoRoot -ScriptRoot $PSScriptRoot

Write-Host "RepoRoot: $repoRoot"

Invoke-Step -Name "P3.5 legacy terminology gate" -Command {
    $legacyPattern = 'review_panel|change_agent|implement_agent'
    $legacyArgs = @(
        '-n',
        $legacyPattern,
        'internal',
        'cmd',
        'configs',
        'web',
        'docs/spec',
        '-g',
        '!web/node_modules/**',
        '-g',
        '!web/dist/**'
    )

    $legacyHits = & rg @legacyArgs
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Found forbidden legacy terms:" -ForegroundColor Red
        $legacyHits | ForEach-Object { Write-Host $_ }
        throw "legacy terminology detected"
    }
    if ($LASTEXITCODE -ne 1) {
        throw "rg failed while checking legacy terminology (exit code: $LASTEXITCODE)"
    }
}

Invoke-Step -Name "P3.5 role-driven terminology presence gate" -CheckLastExitCode -Command {
    rg -n 'review_orchestrator|change_role|implement_role|stage\.role' docs/spec docs/plans internal cmd configs web -g '!docs/plans/archive/**' -g '!web/node_modules/**' -g '!web/dist/**'
}

Write-Host ""
Write-Host "P3.5 terminology gate passed." -ForegroundColor Green
