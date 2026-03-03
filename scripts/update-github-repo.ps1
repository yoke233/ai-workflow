<#
.SYNOPSIS
同步/更新 GitHub 仓库到本地目录，并可选择同步仓库文档到 docs\vendor。

.DESCRIPTION
- 支持多种 GitHub 仓库写法：
  - owner/repo（短格式）
  - github.com/owner/repo(.git)
  - https://github.com/owner/repo(.git)
  - git@github.com:owner/repo(.git)
  - ssh://git@github.com/owner/repo(.git)
  - GitHub Enterprise 同类格式（自定义 host）
- 目录不存在时：自动 clone
- 目录已存在时：自动 fetch + 切换/更新到指定 Ref
- Ref 支持：分支、tag、commit
- 默认使用远端默认分支（通常是 main）
- 默认将 docs 同步到 docs/vendor/<host>-<owner>-<repo>-upstream-docs

.EXAMPLE
pwsh -NoProfile -File .\scripts\update-github-repo.ps1

.EXAMPLE
pwsh -NoProfile -File .\scripts\update-github-repo.ps1 `
  -Repository 'agentclientprotocol/agent-client-protocol' `
  -Ref main

.EXAMPLE
pwsh -NoProfile -File .\scripts\update-github-repo.ps1 `
  -Repository 'git@github.com:owner/repo.git' `
  -TargetPath '.tmp/github-repos/owner-repo' `
  -Force

.EXAMPLE
pwsh -NoProfile -File .\scripts\update-github-repo.ps1 `
  -Repository 'https://github.com/owner/repo' `
  -DocsSourcePath 'website/docs' `
  -CopyDocsTo '.\docs\vendor\owner-repo-upstream-docs'
#>
[CmdletBinding()]
param(
  [Alias("RepoUrl")]
  [string]$Repository = "agentclientprotocol/agent-client-protocol",
  [string]$TargetPath = "",
  [string]$Ref = "",
  [switch]$Force,
  [switch]$Depth1,
  [string]$CopyDocsTo = "",
  [string]$DocsSourcePath = "docs",
  [string]$BaseDir = ""
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Write-Step {
  param([string]$Message)
  Write-Host "[github-sync] $Message"
}

function Invoke-Git {
  param(
    [string[]]$Arguments,
    [string]$WorkingDirectory = "",
    [switch]$CaptureOutput
  )

  if ($CaptureOutput) {
    if ($WorkingDirectory) {
      $output = & git -C $WorkingDirectory @Arguments 2>&1
    } else {
      $output = & git @Arguments 2>&1
    }
    if ($LASTEXITCODE -ne 0) {
      $joined = ($output -join "`n").Trim()
      throw "git $($Arguments -join ' ') 执行失败。`n$joined"
    }
    return ($output -join "`n").Trim()
  }

  if ($WorkingDirectory) {
    & git -C $WorkingDirectory @Arguments
  } else {
    & git @Arguments
  }
  if ($LASTEXITCODE -ne 0) {
    throw "git $($Arguments -join ' ') 执行失败。"
  }
}

function Invoke-GitProbe {
  param(
    [string[]]$Arguments,
    [int]$RetryCount = 3,
    [int]$RetryDelaySeconds = 1
  )

  for ($attempt = 1; $attempt -le $RetryCount; $attempt++) {
    $output = & git @Arguments 2>&1
    $exitCode = $LASTEXITCODE
    if ($exitCode -eq 0) {
      return @{
        ok = $true
        output = ($output -join "`n").Trim()
        exitCode = 0
      }
    }

    if ($attempt -lt $RetryCount) {
      Start-Sleep -Seconds $RetryDelaySeconds
      continue
    }

    return @{
      ok = $false
      output = ($output -join "`n").Trim()
      exitCode = $exitCode
    }
  }

  return @{
    ok = $false
    output = ""
    exitCode = 1
  }
}

function Remove-GitSuffix {
  param([string]$RepoName)

  if ($RepoName.EndsWith(".git", [System.StringComparison]::OrdinalIgnoreCase)) {
    return $RepoName.Substring(0, $RepoName.Length - 4)
  }
  return $RepoName
}

function New-RepoSpec {
  param(
    [string]$RepoHost,
    [string]$Owner,
    [string]$Repo,
    [string]$GitRemote
  )

  $hostNormalized = $RepoHost.Trim().ToLowerInvariant()
  $ownerNormalized = $Owner.Trim()
  $repoNormalized = Remove-GitSuffix -RepoName $Repo.Trim()

  if (-not $hostNormalized -or -not $ownerNormalized -or -not $repoNormalized) {
    throw "仓库信息不完整：host=$RepoHost owner=$Owner repo=$Repo"
  }

  return @{
    host = $hostNormalized
    owner = $ownerNormalized
    repo = $repoNormalized
    gitRemote = $GitRemote
    display = "$hostNormalized/$ownerNormalized/$repoNormalized"
    identity = "$hostNormalized/$($ownerNormalized.ToLowerInvariant())/$($repoNormalized.ToLowerInvariant())"
  }
}

function Resolve-RepositorySpec {
  param([string]$RepositoryValue)

  $raw = $RepositoryValue.Trim()
  if (-not $raw) {
    throw "Repository 不能为空。"
  }

  if ($raw -match '^(?<owner>[^/\s]+)/(?<repo>[^/\s]+)$') {
    $owner = $Matches.owner
    $repo = Remove-GitSuffix -RepoName $Matches.repo
    $gitRemote = "https://github.com/$owner/$repo.git"
    return New-RepoSpec -RepoHost "github.com" -Owner $owner -Repo $repo -GitRemote $gitRemote
  }

  if ($raw -match '^(?<host>[^/\s]+\.[^/\s]+)/(?<owner>[^/\s]+)/(?<repo>[^/\s]+)$') {
    $repoHost = $Matches.host
    $owner = $Matches.owner
    $repo = Remove-GitSuffix -RepoName $Matches.repo
    $gitRemote = "https://$repoHost/$owner/$repo.git"
    return New-RepoSpec -RepoHost $repoHost -Owner $owner -Repo $repo -GitRemote $gitRemote
  }

  if ($raw -match '^(?<user>[^@/\s]+)@(?<host>[^:/\s]+):(?<owner>[^/\s]+)/(?<repo>[^/\s]+?)(?:\.git)?/?$') {
    $repoHost = $Matches.host
    $owner = $Matches.owner
    $repo = Remove-GitSuffix -RepoName $Matches.repo
    $gitRemote = "$($Matches.user)@${repoHost}:$owner/$repo.git"
    return New-RepoSpec -RepoHost $repoHost -Owner $owner -Repo $repo -GitRemote $gitRemote
  }

  $uri = $null
  if ([System.Uri]::TryCreate($raw, [System.UriKind]::Absolute, [ref]$uri)) {
    $path = $uri.AbsolutePath.Trim("/")
    $segments = @()
    if ($path) {
      $segments = $path -split "/"
    }

    if ($segments.Count -ne 2) {
      throw "仓库 URL 需要是 host/owner/repo 形式：$raw"
    }

    $owner = $segments[0]
    $repo = Remove-GitSuffix -RepoName $segments[1]
    $gitRemote = $raw.TrimEnd("/")
    return New-RepoSpec -RepoHost $uri.Host -Owner $owner -Repo $repo -GitRemote $gitRemote
  }

  throw "不支持的仓库写法：$raw。请使用 owner/repo、https、ssh 或 git@host:owner/repo。"
}

function Get-RepoPathToken {
  param([hashtable]$RepoSpec)

  $raw = "$($RepoSpec.host)-$($RepoSpec.owner)-$($RepoSpec.repo)"
  return ($raw -replace '[^A-Za-z0-9._-]', '-')
}

function Get-RemoteDefaultBranch {
  param([string]$Repo)
  $probe = Invoke-GitProbe -Arguments @("ls-remote", "--symref", $Repo, "HEAD")
  if (-not $probe.ok) {
    throw "无法读取远端默认分支：$Repo`n$($probe.output)"
  }

  $symref = $probe.output -split "`n"
  foreach ($line in $symref) {
    if ($line -match '^ref:\s+refs/heads/(?<branch>[^\s]+)\s+HEAD$') {
      return $Matches.branch
    }
  }

  return "main"
}

function Resolve-RefKind {
  param(
    [string]$Repo,
    [string]$RefName
  )

  if (-not $RefName) {
    $defaultBranch = Get-RemoteDefaultBranch -Repo $Repo
    return @{
      kind = "branch"
      name = $defaultBranch
    }
  }

  if ($RefName -match '^[0-9a-fA-F]{7,40}$') {
    return @{
      kind = "commit"
      name = $RefName
    }
  }

  $headProbe = Invoke-GitProbe -Arguments @("ls-remote", "--heads", $Repo, "refs/heads/$RefName")
  if ($headProbe.ok -and $headProbe.output) {
    return @{
      kind = "branch"
      name = $RefName
    }
  }

  $tagProbe = Invoke-GitProbe -Arguments @("ls-remote", "--tags", $Repo, "refs/tags/$RefName")
  if ($tagProbe.ok -and $tagProbe.output) {
    return @{
      kind = "tag"
      name = $RefName
    }
  }

  throw "远端仓库不存在 ref: $RefName"
}

function Ensure-CleanTree {
  param(
    [string]$RepoPath,
    [bool]$AllowForce
  )

  $status = Invoke-Git -WorkingDirectory $RepoPath -Arguments @("status", "--porcelain") -CaptureOutput
  if (-not $status) {
    return
  }

  if (-not $AllowForce) {
    throw "目标目录存在未提交改动：$RepoPath。请先清理，或使用 -Force 覆盖。"
  }

  Write-Step "检测到未提交改动，执行强制清理（reset --hard + clean -fd）"
  Invoke-Git -WorkingDirectory $RepoPath -Arguments @("reset", "--hard")
  Invoke-Git -WorkingDirectory $RepoPath -Arguments @("clean", "-fd")
}

function Resolve-AbsolutePath {
  param(
    [string]$PathValue,
    [string]$BaseDirectory
  )

  if ([string]::IsNullOrWhiteSpace($PathValue)) {
    return ""
  }

  if ([System.IO.Path]::IsPathRooted($PathValue)) {
    return [System.IO.Path]::GetFullPath($PathValue)
  }
  return [System.IO.Path]::GetFullPath((Join-Path -Path $BaseDirectory -ChildPath $PathValue))
}

if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
  throw "未找到 git，请先安装并加入 PATH。"
}

$defaultBaseDir = ""
if ($PSScriptRoot) {
  $defaultBaseDir = [System.IO.Path]::GetFullPath((Join-Path -Path $PSScriptRoot -ChildPath ".."))
} else {
  $defaultBaseDir = (Get-Location).Path
}

$baseDirResolved = $defaultBaseDir
if ($BaseDir) {
  $baseDirResolved = Resolve-AbsolutePath -PathValue $BaseDir -BaseDirectory $defaultBaseDir
}

$repoInfo = Resolve-RepositorySpec -RepositoryValue $Repository
$repoToken = Get-RepoPathToken -RepoSpec $repoInfo

if (-not $PSBoundParameters.ContainsKey("TargetPath") -or [string]::IsNullOrWhiteSpace($TargetPath)) {
  $TargetPath = ".tmp/github-repos/$repoToken"
}

if (-not $PSBoundParameters.ContainsKey("CopyDocsTo")) {
  $CopyDocsTo = "docs/vendor/$repoToken-upstream-docs"
}

$targetFullPath = Resolve-AbsolutePath -PathValue $TargetPath -BaseDirectory $baseDirResolved
$targetParent = Split-Path -Path $targetFullPath -Parent
$refInfo = Resolve-RefKind -Repo $repoInfo.gitRemote -RefName $Ref

Write-Step "基准目录: $baseDirResolved"
Write-Step "目标目录: $targetFullPath"
Write-Step "仓库标识: $($repoInfo.display)"
Write-Step "远端仓库: $($repoInfo.gitRemote)"
Write-Step "目标引用: $($refInfo.kind) $($refInfo.name)"

if (-not (Test-Path -LiteralPath $targetFullPath)) {
  if (-not (Test-Path -LiteralPath $targetParent)) {
    New-Item -ItemType Directory -Path $targetParent -Force | Out-Null
  }

  $cloneArgs = @("clone")
  if ($Depth1 -and $refInfo.kind -ne "commit") {
    $cloneArgs += "--depth=1"
  }
  if ($refInfo.kind -in @("branch", "tag")) {
    $cloneArgs += @("--branch", $refInfo.name)
  }
  if ($refInfo.kind -eq "branch") {
    $cloneArgs += "--single-branch"
  }
  $cloneArgs += @($repoInfo.gitRemote, $targetFullPath)

  Write-Step "目录不存在，执行 clone"
  Invoke-Git -Arguments $cloneArgs
} else {
  if (-not (Test-Path -LiteralPath (Join-Path -Path $targetFullPath -ChildPath ".git"))) {
    throw "目标目录已存在但不是 git 仓库：$targetFullPath"
  }

  $currentOrigin = Invoke-Git -WorkingDirectory $targetFullPath -Arguments @("remote", "get-url", "origin") -CaptureOutput
  $originMatched = $false
  try {
    $currentOriginInfo = Resolve-RepositorySpec -RepositoryValue $currentOrigin
    $originMatched = ($currentOriginInfo.identity -eq $repoInfo.identity)
  } catch {
    $originMatched = ($currentOrigin.TrimEnd("/") -eq $repoInfo.gitRemote.TrimEnd("/"))
  }

  if (-not $originMatched) {
    if ($Force) {
      Write-Step "origin 不匹配，使用 -Force 更新 origin 到 $($repoInfo.gitRemote)"
      Invoke-Git -WorkingDirectory $targetFullPath -Arguments @("remote", "set-url", "origin", $repoInfo.gitRemote)
    } else {
      throw "origin 不匹配：当前=$currentOrigin，期望=$($repoInfo.gitRemote)。使用 -Force 可覆盖 origin。"
    }
  }

  Ensure-CleanTree -RepoPath $targetFullPath -AllowForce $Force.IsPresent
  Write-Step "目录已存在，执行 fetch"
  Invoke-Git -WorkingDirectory $targetFullPath -Arguments @("fetch", "--tags", "--prune", "origin")
}

switch ($refInfo.kind) {
  "branch" {
    $branchExists = & git -C $targetFullPath show-ref --verify --quiet "refs/heads/$($refInfo.name)"
    $branchExists = ($LASTEXITCODE -eq 0)
    if ($branchExists) {
      Invoke-Git -WorkingDirectory $targetFullPath -Arguments @("checkout", $refInfo.name)
    } else {
      Invoke-Git -WorkingDirectory $targetFullPath -Arguments @("checkout", "-b", $refInfo.name, "--track", "origin/$($refInfo.name)")
    }

    if ($Force) {
      Invoke-Git -WorkingDirectory $targetFullPath -Arguments @("reset", "--hard", "origin/$($refInfo.name)")
      Invoke-Git -WorkingDirectory $targetFullPath -Arguments @("clean", "-fd")
    } else {
      Invoke-Git -WorkingDirectory $targetFullPath -Arguments @("pull", "--ff-only", "origin", $refInfo.name)
    }
  }
  "tag" {
    Invoke-Git -WorkingDirectory $targetFullPath -Arguments @("checkout", "--detach", "refs/tags/$($refInfo.name)")
  }
  "commit" {
    $commitKnown = & git -C $targetFullPath cat-file -e "$($refInfo.name)^{commit}" 2>$null
    $commitKnown = ($LASTEXITCODE -eq 0)
    if (-not $commitKnown) {
      Invoke-Git -WorkingDirectory $targetFullPath -Arguments @("fetch", "origin", $refInfo.name)
    }
    Invoke-Git -WorkingDirectory $targetFullPath -Arguments @("checkout", "--detach", $refInfo.name)
  }
}

$head = Invoke-Git -WorkingDirectory $targetFullPath -Arguments @("rev-parse", "HEAD") -CaptureOutput
$headShort = Invoke-Git -WorkingDirectory $targetFullPath -Arguments @("rev-parse", "--short", "HEAD") -CaptureOutput
$headDate = Invoke-Git -WorkingDirectory $targetFullPath -Arguments @("show", "-s", "--format=%ci", "HEAD") -CaptureOutput

if ($CopyDocsTo) {
  $docsSource = Resolve-AbsolutePath -PathValue $DocsSourcePath -BaseDirectory $targetFullPath
  if (-not (Test-Path -LiteralPath $docsSource)) {
    throw "未找到 docs 源目录：$docsSource（可用 -DocsSourcePath 指定）"
  }

  $docsTarget = Resolve-AbsolutePath -PathValue $CopyDocsTo -BaseDirectory $baseDirResolved
  if (-not (Test-Path -LiteralPath $docsTarget)) {
    New-Item -ItemType Directory -Path $docsTarget -Force | Out-Null
  }

  Copy-Item -Path (Join-Path -Path $docsSource -ChildPath "*") -Destination $docsTarget -Recurse -Force
  Write-Step "已同步 docs: $docsSource -> $docsTarget"
  Write-Host "docs_target=$docsTarget"
}

Write-Step "更新完成"
Write-Host "repo_spec=$($repoInfo.display)"
Write-Host "repo_url=$($repoInfo.gitRemote)"
Write-Host "repo_path=$targetFullPath"
Write-Host "ref_kind=$($refInfo.kind)"
Write-Host "ref_name=$($refInfo.name)"
Write-Host "head=$head"
Write-Host "head_short=$headShort"
Write-Host "head_date=$headDate"
