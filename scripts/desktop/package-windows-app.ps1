param(
  [string]$AppName = "Nexus",
  [string]$ExecutableName = "Nexus",
  [string]$RuntimeIdentifier = "win-x64",
  [string]$Configuration = "Release",
  [string]$AppBuildDir = $env:NEXUS_DESKTOP_APP_BUILD_DIR,
  [string]$OutputDir = $env:NEXUS_DESKTOP_PACKAGE_OUTPUT_DIR,
  [string]$Version = $env:NEXUS_DESKTOP_VERSION,
  [string]$BuildNumber = $env:NEXUS_DESKTOP_BUILD_NUMBER,
  [string]$PackageName = $env:NEXUS_DESKTOP_PACKAGE_NAME,
  [string]$MetadataPath = $env:NEXUS_DESKTOP_PACKAGE_METADATA_PATH,
  [int]$SmokeTimeoutSeconds = 75,
  [switch]$SkipBuild,
  [switch]$SkipSmoke
)

$ErrorActionPreference = "Stop"

function Resolve-RootDir {
  $scriptDir = Split-Path -Parent $PSCommandPath
  return (Resolve-Path (Join-Path $scriptDir "../..")).Path
}

function Resolve-AppVersion([string]$rootDir, [string]$version) {
  if (-not [string]::IsNullOrWhiteSpace($version)) {
    return $version
  }
  Push-Location (Join-Path $rootDir "web")
  try {
    return (node -p "require('./package.json').version").Trim()
  } finally {
    Pop-Location
  }
}

function Resolve-BuildNumber([string]$rootDir, [string]$buildNumber) {
  if (-not [string]::IsNullOrWhiteSpace($buildNumber)) {
    return $buildNumber
  }

  $value = & git -C $rootDir rev-list --count HEAD 2>$null
  if ($LASTEXITCODE -eq 0 -and -not [string]::IsNullOrWhiteSpace($value)) {
    return ($value | Select-Object -First 1).Trim()
  }
  return (Get-Date -Format "yyyyMMddHHmmss")
}

function Resolve-GitValue([string]$rootDir, [string[]]$arguments, [string]$fallback) {
  $value = & git -C $rootDir @arguments 2>$null
  if ($LASTEXITCODE -eq 0 -and -not [string]::IsNullOrWhiteSpace($value)) {
    return ($value | Select-Object -First 1).Trim()
  }
  return $fallback
}

function Test-SourceDirty([string]$rootDir) {
  $status = & git -C $rootDir status --porcelain --untracked-files=normal 2>$null
  if ($LASTEXITCODE -ne 0) {
    return $true
  }
  return -not [string]::IsNullOrWhiteSpace(($status -join "`n"))
}

function Resolve-ExecutableFileName([string]$name) {
  if ($name.EndsWith(".exe", [System.StringComparison]::OrdinalIgnoreCase)) {
    return $name
  }
  return "$name.exe"
}

if ($env:NEXUS_DESKTOP_SMOKE_TIMEOUT_SECONDS) {
  $SmokeTimeoutSeconds = [int]$env:NEXUS_DESKTOP_SMOKE_TIMEOUT_SECONDS
}

$rootDir = Resolve-RootDir
$windowsDir = Join-Path $rootDir "desktop/windows"
$appVersion = Resolve-AppVersion $rootDir $Version
$resolvedBuildNumber = Resolve-BuildNumber $rootDir $BuildNumber
$createdAt = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$commitSha = Resolve-GitValue -rootDir $rootDir -arguments @("rev-parse", "HEAD") -fallback "unknown"
$commitShort = Resolve-GitValue -rootDir $rootDir -arguments @("rev-parse", "--short", "HEAD") -fallback "unknown"
$sourceDirty = Test-SourceDirty $rootDir
$executableFileName = Resolve-ExecutableFileName $ExecutableName

if ([string]::IsNullOrWhiteSpace($AppBuildDir)) {
  $AppBuildDir = Join-Path $windowsDir ".build/app/$AppName"
}
if ([string]::IsNullOrWhiteSpace($OutputDir)) {
  $OutputDir = Join-Path $windowsDir ".build/package"
}
if ([string]::IsNullOrWhiteSpace($PackageName)) {
  $PackageName = "$AppName-windows-$appVersion-$resolvedBuildNumber"
}

$stagingRoot = Join-Path $OutputDir "staging"
$stagingDir = Join-Path $stagingRoot $PackageName
$stagedAppDir = Join-Path $stagingDir $AppName
$artifactPath = Join-Path $OutputDir "$PackageName.zip"
$sha256Path = "$artifactPath.sha256"
if ([string]::IsNullOrWhiteSpace($MetadataPath)) {
  $MetadataPath = "$artifactPath.metadata.json"
}
$metadataStagingPath = Join-Path $stagingDir "PACKAGE-METADATA.json"

if ((-not $SkipBuild) -and $env:NEXUS_DESKTOP_PACKAGE_SKIP_BUILD -ne "1") {
  & (Join-Path $rootDir "scripts/desktop/build-windows-app.ps1") `
    -Configuration $Configuration `
    -RuntimeIdentifier $RuntimeIdentifier `
    -AppName $AppName `
    -ExecutableName $ExecutableName `
    -OutputDir $AppBuildDir `
    -Version $appVersion `
    -BuildNumber $resolvedBuildNumber
}

$appExe = Join-Path $AppBuildDir $executableFileName
if (-not (Test-Path -LiteralPath $appExe)) {
  throw "Missing Windows app executable: $appExe"
}

if ((-not $SkipSmoke) -and $env:NEXUS_DESKTOP_PACKAGE_SKIP_SMOKE -ne "1") {
  & (Join-Path $rootDir "scripts/desktop/smoke-windows-app.ps1") `
    -AppDir $AppBuildDir `
    -ExecutableName $executableFileName `
    -TimeoutSeconds $SmokeTimeoutSeconds
}

foreach ($path in @($stagingDir, $artifactPath, $sha256Path, $MetadataPath)) {
  if (Test-Path -LiteralPath $path) {
    Remove-Item -LiteralPath $path -Recurse -Force
  }
}
New-Item -ItemType Directory -Force -Path $stagedAppDir | Out-Null
New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null

Copy-Item -Recurse -Force -Path (Join-Path $AppBuildDir "*") -Destination $stagedAppDir

$readmePath = Join-Path $stagingDir "README.txt"
@"
Nexus Windows app package

Version: $appVersion
Build: $resolvedBuildNumber
Commit: $commitShort
Created: $createdAt

This package is unsigned and is not an installer.
After verifying the sha256 file, unzip the package and run:
  $AppName\$executableFileName

WebView2 Runtime is required. The app stores local data under:
  %LOCALAPPDATA%\Nexus

To reset app data, quit Nexus first, then remove that directory.
To register nexus:// for the unzipped directory, run:
  $AppName\register-nexus-protocol.ps1
"@ | Set-Content -Encoding UTF8 -Path $readmePath

$metadata = [ordered]@{
  app_name = $AppName
  executable_name = $executableFileName
  platform = "windows"
  runtime_identifier = $RuntimeIdentifier
  version = $appVersion
  build_number = $resolvedBuildNumber
  created_at = $createdAt
  source = [ordered]@{
    commit = $commitSha
    short_commit = $commitShort
    dirty = $sourceDirty
  }
  signing = [ordered]@{
    kind = "unsigned"
    code_signed = $false
    installer = $false
  }
  credentials = [ordered]@{
    expected_storage = "dpapi"
    fallback_storage = "file"
  }
  artifact = [ordered]@{
    name = $PackageName
    format = "zip"
  }
  validation = [ordered]@{
    build_script = "scripts/desktop/build-windows-app.ps1"
    smoke_script = "scripts/desktop/smoke-windows-app.ps1"
    smoke_skipped = [bool]($SkipSmoke -or ($env:NEXUS_DESKTOP_PACKAGE_SKIP_SMOKE -eq "1"))
  }
}

$metadata | ConvertTo-Json -Depth 8 | Set-Content -Encoding UTF8 -Path $metadataStagingPath
Copy-Item -Force -LiteralPath $metadataStagingPath -Destination $MetadataPath

Compress-Archive -Path $stagingDir -DestinationPath $artifactPath -Force
$hash = (Get-FileHash -Algorithm SHA256 $artifactPath).Hash.ToLowerInvariant()
"$hash  $(Split-Path -Leaf $artifactPath)" | Set-Content -Encoding ASCII -Path $sha256Path

Write-Host "Windows app zip: $artifactPath"
Write-Host "sha256: $sha256Path"
Write-Host "metadata: $MetadataPath"
