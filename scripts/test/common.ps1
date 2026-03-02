Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Enter-RepoRoot {
    param(
        [Parameter(Mandatory = $true)]
        [string]$ScriptRoot
    )

    $repoRoot = Resolve-Path -LiteralPath (Join-Path $ScriptRoot "..\..")
    Set-Location -LiteralPath $repoRoot
    return $repoRoot.Path
}

function Invoke-Step {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Name,
        [Parameter(Mandatory = $true)]
        [scriptblock]$Command,
        [switch]$CheckLastExitCode
    )

    Write-Host ""
    Write-Host "==> $Name" -ForegroundColor Cyan
    $startedAt = Get-Date
    $global:LASTEXITCODE = 0

    & $Command

    if ($CheckLastExitCode -and $LASTEXITCODE -ne 0) {
        throw "Step '$Name' failed with exit code $LASTEXITCODE."
    }

    $elapsed = [math]::Round(((Get-Date) - $startedAt).TotalSeconds, 2)
    Write-Host "<== $Name (${elapsed}s)" -ForegroundColor Green
}

function Set-SafeTestEnvironment {
    if (-not $env:GOMAXPROCS) {
        $env:GOMAXPROCS = "4"
    }
    if (-not $env:GOTEST_TIMEOUT) {
        $env:GOTEST_TIMEOUT = "20m"
    }
}
