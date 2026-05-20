param(
  [string]$Configuration = "Release",
  [string]$RuntimeIdentifier = "win-x64",
  [string]$AppName = "Nexus",
  [string]$ExecutableName = "Nexus",
  [string]$OutputDir = "",
  [string]$Version = "",
  [string]$BuildNumber = "",
  [string]$SelfContained = $env:NEXUS_DESKTOP_SELF_CONTAINED,
  [switch]$CreateArchive
)

$ErrorActionPreference = "Stop"

function Resolve-RootDir {
  $scriptDir = Split-Path -Parent $PSCommandPath
  return (Resolve-Path (Join-Path $scriptDir "../..")).Path
}

function Read-DotEnvValue([string]$rootDir, [string]$key) {
  $dotenvPath = Join-Path $rootDir ".env"
  if (-not (Test-Path $dotenvPath)) {
    return ""
  }

  $pattern = "^\s*(?:export\s+)?$([Regex]::Escape($key))\s*=(.*)$"
  foreach ($line in Get-Content $dotenvPath) {
    $match = [Regex]::Match($line, $pattern)
    if (-not $match.Success) {
      continue
    }
    $value = $match.Groups[1].Value -replace "\s+#.*$", ""
    $value = $value.Trim()
    if (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'"))) {
      $value = $value.Substring(1, $value.Length - 2)
    }
    return $value
  }

  return ""
}

function Use-DotEnvValueIfMissing([string]$rootDir, [string]$key) {
  if (-not [string]::IsNullOrWhiteSpace([Environment]::GetEnvironmentVariable($key))) {
    return
  }
  $value = Read-DotEnvValue $rootDir $key
  if (-not [string]::IsNullOrWhiteSpace($value)) {
    [Environment]::SetEnvironmentVariable($key, $value, "Process")
  }
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
  try {
    return (git -C $rootDir rev-list --count HEAD).Trim()
  } catch {
    return (Get-Date -Format "yyyyMMddHHmmss")
  }
}

function Convert-FileVersion([string]$version, [string]$buildNumber) {
  $parts = @($version.Split(".") | ForEach-Object {
    $value = 0
    if ([int]::TryParse($_, [ref]$value)) {
      $value
    } else {
      0
    }
  })

  while ($parts.Count -lt 3) {
    $parts += 0
  }

  $buildValue = 0
  [void][int]::TryParse($buildNumber, [ref]$buildValue)
  $buildValue = [Math]::Min([Math]::Max($buildValue, 0), 65534)
  return "$($parts[0]).$($parts[1]).$($parts[2]).$buildValue"
}

function Resolve-Bool([string]$value, [bool]$defaultValue) {
  if ([string]::IsNullOrWhiteSpace($value)) {
    return $defaultValue
  }

  switch ($value.Trim().ToLowerInvariant()) {
    "1" { return $true }
    "true" { return $true }
    "yes" { return $true }
    "on" { return $true }
    "0" { return $false }
    "false" { return $false }
    "no" { return $false }
    "off" { return $false }
  }

  throw "Invalid boolean value: $value"
}

function Resolve-WindowsGoArch([string]$runtimeIdentifier) {
  switch ($runtimeIdentifier) {
    "win-x64" { return "amd64" }
  }

  throw "Unsupported Windows RuntimeIdentifier '$runtimeIdentifier'. Current desktop package only supports win-x64."
}

function Resolve-GitValue([string]$rootDir, [string[]]$arguments, [string]$fallback) {
  $value = & git -C $rootDir @arguments 2>$null
  if ($LASTEXITCODE -eq 0 -and -not [string]::IsNullOrWhiteSpace($value)) {
    return ($value | Select-Object -First 1).Trim()
  }
  return $fallback
}

$rootDir = Resolve-RootDir
Use-DotEnvValueIfMissing $rootDir "NEXUS_DESKTOP_GITHUB_CLIENT_ID"
Use-DotEnvValueIfMissing $rootDir "NEXUS_DESKTOP_GITHUB_CLIENT_SECRET"
$windowsDir = Join-Path $rootDir "desktop/windows"
$projectPath = Join-Path $windowsDir "Nexus.Desktop/Nexus.Desktop.csproj"
$appVersion = Resolve-AppVersion $rootDir $Version
$resolvedBuildNumber = Resolve-BuildNumber $rootDir $BuildNumber
$fileVersion = Convert-FileVersion $appVersion $resolvedBuildNumber
$publishSelfContained = Resolve-Bool $SelfContained $false
$publishSelfContainedValue = $publishSelfContained.ToString().ToLowerInvariant()
$goArch = Resolve-WindowsGoArch $RuntimeIdentifier
$gitCommit = Resolve-GitValue -rootDir $rootDir -arguments @("rev-parse", "--short=12", "HEAD") -fallback "unknown"
$buildDate = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")

if ([string]::IsNullOrWhiteSpace($OutputDir)) {
  $OutputDir = Join-Path $windowsDir ".build/app/$AppName"
}

$intermediateDir = Join-Path $windowsDir ".build/intermediates"
$sidecarPath = Join-Path $intermediateDir "nexus-server.exe"
$publishDir = Join-Path $intermediateDir "publish"
$resourcesDir = Join-Path $OutputDir "Resources"
$packageDir = Join-Path $windowsDir ".build/package"

Write-Host "==> Building web/dist"
Push-Location (Join-Path $rootDir "web")
try {
  pnpm install --frozen-lockfile
  $env:NEXUS_DESKTOP_BUILD = "1"
  pnpm build
} finally {
  Remove-Item Env:NEXUS_DESKTOP_BUILD -ErrorAction SilentlyContinue
  Pop-Location
}

Write-Host "==> Building Go sidecar"
New-Item -ItemType Directory -Force -Path $intermediateDir | Out-Null
Push-Location $rootDir
$previousCgoEnabled = $env:CGO_ENABLED
$previousGoos = $env:GOOS
$previousGoarch = $env:GOARCH
try {
  $env:CGO_ENABLED = if ($env:CGO_ENABLED) { $env:CGO_ENABLED } else { "0" }
  $env:GOOS = "windows"
  $env:GOARCH = $goArch
  $versionPackage = "github.com/nexus-research-lab/nexus/internal/version"
  $ldflags = "-s -w -X $versionPackage.AppVersion=$appVersion -X $versionPackage.GitCommit=$gitCommit -X $versionPackage.BuildDate=$buildDate"
  go build -trimpath -ldflags $ldflags -o $sidecarPath ./cmd/nexus-server
} finally {
  if ($null -eq $previousCgoEnabled) { Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue } else { $env:CGO_ENABLED = $previousCgoEnabled }
  if ($null -eq $previousGoos) { Remove-Item Env:GOOS -ErrorAction SilentlyContinue } else { $env:GOOS = $previousGoos }
  if ($null -eq $previousGoarch) { Remove-Item Env:GOARCH -ErrorAction SilentlyContinue } else { $env:GOARCH = $previousGoarch }
  Pop-Location
}

Write-Host "==> Publishing Windows shell"
dotnet publish $projectPath `
  -c $Configuration `
  -r $RuntimeIdentifier `
  --self-contained $publishSelfContainedValue `
  -p:NexusDesktopVersion=$appVersion `
  -p:NexusDesktopBuildNumber=$resolvedBuildNumber `
  -p:NexusDesktopFileVersion=$fileVersion `
  -p:NexusDesktopAssemblyVersion=$fileVersion `
  -o $publishDir

Write-Host "==> Assembling $OutputDir"
Remove-Item -Recurse -Force $OutputDir -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
New-Item -ItemType Directory -Force -Path $resourcesDir | Out-Null

Copy-Item -Recurse -Force (Join-Path $publishDir "*") $OutputDir
Copy-Item -Force $sidecarPath (Join-Path $resourcesDir "nexus-server.exe")
Copy-Item -Recurse -Force (Join-Path $rootDir "web/dist") (Join-Path $resourcesDir "Web")
New-Item -ItemType Directory -Force -Path (Join-Path $resourcesDir "db") | Out-Null
Copy-Item -Recurse -Force (Join-Path $rootDir "db/migrations") (Join-Path $resourcesDir "db/migrations")
Copy-Item -Recurse -Force (Join-Path $rootDir "skills") (Join-Path $resourcesDir "skills")

$desktopEnvPath = Join-Path $resourcesDir "desktop.env"
Remove-Item -Force $desktopEnvPath -ErrorAction SilentlyContinue
$desktopEnvLines = @()
if (-not [string]::IsNullOrWhiteSpace($env:NEXUS_DESKTOP_GITHUB_CLIENT_ID)) {
  $desktopEnvLines += "CONNECTOR_GITHUB_CLIENT_ID=$($env:NEXUS_DESKTOP_GITHUB_CLIENT_ID.Trim())"
}
if (-not [string]::IsNullOrWhiteSpace($env:NEXUS_DESKTOP_GITHUB_CLIENT_SECRET)) {
  $desktopEnvLines += "CONNECTOR_GITHUB_CLIENT_SECRET=$($env:NEXUS_DESKTOP_GITHUB_CLIENT_SECRET.Trim())"
}
if ($desktopEnvLines.Count -gt 0) {
  $desktopEnvLines | Set-Content -Encoding UTF8 $desktopEnvPath
}

$registerProtocolScript = Join-Path $OutputDir "register-nexus-protocol.ps1"
@"
`$ErrorActionPreference = "Stop"
`$exe = Join-Path `$PSScriptRoot "$ExecutableName.exe"
if (-not (Test-Path `$exe)) {
  throw "Missing Nexus executable: `$exe"
}
`$protocolKey = "HKCU:\Software\Classes\nexus"
New-Item -Force -Path `$protocolKey | Out-Null
Set-Item -Path `$protocolKey -Value "URL:Nexus Protocol"
New-ItemProperty -Force -Path `$protocolKey -Name "URL Protocol" -Value "" | Out-Null
New-Item -Force -Path "`$protocolKey\shell\open\command" | Out-Null
Set-Item -Path "`$protocolKey\shell\open\command" -Value "`"`$exe`" `"%1`""
Write-Host "Registered nexus:// protocol for `$exe"
"@ | Set-Content -Encoding UTF8 $registerProtocolScript

$unregisterProtocolScript = Join-Path $OutputDir "unregister-nexus-protocol.ps1"
@"
`$ErrorActionPreference = "Stop"
Remove-Item -Recurse -Force "HKCU:\Software\Classes\nexus" -ErrorAction SilentlyContinue
Write-Host "Unregistered nexus:// protocol"
"@ | Set-Content -Encoding UTF8 $unregisterProtocolScript

Remove-Item -Recurse -Force $intermediateDir -ErrorAction SilentlyContinue

Write-Host "==> Built $OutputDir"

if ($CreateArchive) {
  Write-Host "==> Creating Windows app archive"
  New-Item -ItemType Directory -Force -Path $packageDir | Out-Null
  $archiveBaseName = "$AppName-windows-$appVersion-$resolvedBuildNumber"
  $archivePath = Join-Path $packageDir "$archiveBaseName.zip"
  $sha256Path = "$archivePath.sha256"
  Remove-Item -Force $archivePath, $sha256Path -ErrorAction SilentlyContinue
  Compress-Archive -Path $OutputDir -DestinationPath $archivePath -Force
  $hash = (Get-FileHash -Algorithm SHA256 $archivePath).Hash.ToLowerInvariant()
  "$hash  $(Split-Path -Leaf $archivePath)" | Set-Content -Encoding ASCII $sha256Path
  Write-Host "archive: $archivePath"
  Write-Host "sha256: $sha256Path"
}
