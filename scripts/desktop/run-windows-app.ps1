param(
  [string]$Configuration = "Release",
  [string]$RuntimeIdentifier = "win-x64",
  [string]$AppName = "Nexus",
  [string]$ExecutableName = "Nexus",
  [string]$OutputDir = "",
  [string]$Version = "",
  [string]$BuildNumber = "",
  [string]$BundleNXSRuntime = $env:NEXUS_DESKTOP_BUNDLE_NXS_RUNTIME,
  [string]$NXSRuntimePath = $env:NEXUS_DESKTOP_NXS_RUNTIME_PATH,
  [switch]$SkipBuild,
  [switch]$Wait
)

$ErrorActionPreference = "Stop"

function Resolve-RootDir {
  $scriptDir = Split-Path -Parent $PSCommandPath
  return (Resolve-Path (Join-Path $scriptDir "../..")).Path
}

function Resolve-ExecutableFileName([string]$name) {
  if ($name.EndsWith(".exe", [System.StringComparison]::OrdinalIgnoreCase)) {
    return $name
  }
  return "$name.exe"
}

$rootDir = Resolve-RootDir
if ([string]::IsNullOrWhiteSpace($OutputDir)) {
  $OutputDir = Join-Path $rootDir "desktop/windows/.build/app/$AppName"
}

if (-not $SkipBuild) {
  Write-Host "==> Building Windows app"
  & (Join-Path $rootDir "scripts/desktop/build-windows-app.ps1") `
    -Configuration $Configuration `
    -RuntimeIdentifier $RuntimeIdentifier `
    -AppName $AppName `
    -ExecutableName $ExecutableName `
    -OutputDir $OutputDir `
    -Version $Version `
    -BuildNumber $BuildNumber `
    -BundleNXSRuntime $BundleNXSRuntime `
    -NXSRuntimePath $NXSRuntimePath
}

$appExe = Join-Path $OutputDir (Resolve-ExecutableFileName $ExecutableName)
if (-not (Test-Path $appExe)) {
  throw "Missing Windows app executable: $appExe. Run 'make app-win-build' first."
}

Write-Host "==> Starting $appExe"
$process = Start-Process -FilePath $appExe -WorkingDirectory $OutputDir -PassThru
Write-Host "==> Started PID $($process.Id)"

if ($Wait) {
  $process.WaitForExit()
  exit $process.ExitCode
}
