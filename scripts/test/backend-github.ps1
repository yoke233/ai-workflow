[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. (Join-Path $PSScriptRoot "common.ps1")

$repoRoot = Enter-RepoRoot -ScriptRoot $PSScriptRoot
Set-SafeTestEnvironment

Write-Host "RepoRoot: $repoRoot"

Invoke-Step -Name "GitHub plugin tests" -CheckLastExitCode -Command {
    go test -p 4 -timeout $env:GOTEST_TIMEOUT ./internal/plugins/tracker-github ./internal/plugins/scm-github ./internal/plugins/review-github-pr -run 'TestGitHubTracker_|TestGitHubSCM_|TestGitHubPRReview_'
}

Invoke-Step -Name "GitHub dispatcher and e2e tests" -CheckLastExitCode -Command {
    go test -p 4 -timeout $env:GOTEST_TIMEOUT ./internal/github -run 'TestWebhookDispatcher_|TestE2E_GitHub_'
}

Invoke-Step -Name "Webhook handler tests" -CheckLastExitCode -Command {
    go test -p 4 -timeout $env:GOTEST_TIMEOUT ./internal/web -run 'TestWebhook_'
}

