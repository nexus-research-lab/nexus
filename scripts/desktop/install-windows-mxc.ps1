param(
  [string]$Version = "1.0.0",
  [string]$InstallRoot = "",
  [string]$EnvFile = ""
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Version)) { $Version = "1.0.0" }

if ([string]::IsNullOrWhiteSpace($InstallRoot)) {
  $InstallRoot = Join-Path $env:LOCALAPPDATA "Nexus\mxc"
}
if ([string]::IsNullOrWhiteSpace($EnvFile)) {
  $EnvFile = Join-Path (Get-Location) ".env.local"
}
$InstallRoot = [IO.Path]::GetFullPath($InstallRoot)
$EnvFile = [IO.Path]::GetFullPath($EnvFile)

$packageRoot = Join-Path ([IO.Path]::GetTempPath()) ("nexus-mxc-" + [Guid]::NewGuid().ToString("N"))
$extractRoot = Join-Path $packageRoot "extract"
New-Item -ItemType Directory -Force -Path $extractRoot | Out-Null
try {
  Push-Location $packageRoot
  $archiveName = (& npm pack "@microsoft/mxc-sdk@$Version" --silent).Trim()
  if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($archiveName)) {
    throw "Unable to download @microsoft/mxc-sdk@$Version"
  }
  $archive = Join-Path $packageRoot $archiveName
  & tar -xzf $archive -C $extractRoot --strip-components=1
  if ($LASTEXITCODE -ne 0) { throw "Unable to extract MXC package" }
  $sourceBin = Join-Path $extractRoot "bin\x64"
  $targetBin = Join-Path $InstallRoot "mxc\bin\x64"
  foreach ($name in @("wxc-exec.exe", "mxc_ffi.dll", "winhttp-proxy-shim.exe")) {
    $source = Join-Path $sourceBin $name
    if (-not (Test-Path -LiteralPath $source -PathType Leaf)) {
      throw "MXC package is missing $name"
    }
  }
  New-Item -ItemType Directory -Force -Path $targetBin | Out-Null
  Copy-Item -Force (Join-Path $sourceBin "wxc-exec.exe") $targetBin
  Copy-Item -Force (Join-Path $sourceBin "mxc_ffi.dll") $targetBin
  Copy-Item -Force (Join-Path $sourceBin "winhttp-proxy-shim.exe") $targetBin
  $executable = Join-Path $targetBin "wxc-exec.exe"
  $probeJson = (& $executable --probe | Out-String) | ConvertFrom-Json
  if (-not $probeJson.probes.baseContainerApiPresent -or $probeJson.needsDaclAugmentation) {
    throw "MXC probe failed: base container API unavailable or DACL augmentation required"
  }
  $envLines = @(
    "NEXUS_MXC_EXECUTABLE=$executable",
    "NEXUS_DESKTOP_MXC_RUNTIME_PATH=$InstallRoot"
  )
  Set-Content -LiteralPath $EnvFile -Value $envLines -Encoding utf8
  Write-Host "Installed MXC $Version at $InstallRoot"
  Write-Host "Wrote environment configuration to $EnvFile"
} finally {
  Pop-Location
  if (Test-Path -LiteralPath $packageRoot) { Remove-Item -LiteralPath $packageRoot -Recurse -Force }
}
