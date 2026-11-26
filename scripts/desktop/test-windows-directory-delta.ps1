# 对两份真实发行版执行与发布流程相同的重建与 Inno 覆盖安装。
$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "windows-inno.ps1")
$root = Join-Path $env:RUNNER_TEMP ("directory-delta-" + [Guid]::NewGuid())
New-Item -ItemType Directory $root | Out-Null
$cacheDir = if ([string]::IsNullOrWhiteSpace($env:NEXUS_WINDOWS_DELTA_FIXTURE_CACHE_DIR)) { $root } else { $env:NEXUS_WINDOWS_DELTA_FIXTURE_CACHE_DIR }
New-Item -ItemType Directory -Force -Path $cacheDir | Out-Null
try {
  $fixtures = @{}
  foreach ($version in @("0.2.7-88", "0.2.8-89")) {
    $name = "NexusSetup-$version.exe"
    $exePath = Join-Path $cacheDir $name
    $shaPath = Join-Path $cacheDir "$name.sha256"
    $cachedValid = $false
    if ((Test-Path -LiteralPath $exePath) -and (Test-Path -LiteralPath $shaPath)) {
      $cachedSum = ((Get-Content -LiteralPath $shaPath -Raw).Trim() -split '\s+')[0]
      if ((Get-FileHash -LiteralPath $exePath).Hash -eq $cachedSum) {
        $cachedValid = $true
      }
    }
    if (-not $cachedValid) {
      Write-Host "==> [delta] Downloading $name"
      gh release download ("v" + $version.Split("-")[0]) -p $name -p "$name.sha256" -D $cacheDir --clobber
      if ($LASTEXITCODE -ne 0) { throw "Failed to download $name" }
      $sum = ((Get-Content -LiteralPath $shaPath -Raw).Trim() -split '\s+')[0]
      if ((Get-FileHash -LiteralPath $exePath).Hash -ne $sum) { throw "Invalid $name" }
    } else {
      Write-Host "==> [delta] Reusing cached $name"
    }
    $fixtures[$version] = $exePath
  }
  $tree = Join-Path $root "next"
  $new = $fixtures["0.2.8-89"]
  Write-Host "==> [delta] Extracting target installer"
  $process = Start-Process -FilePath $new -ArgumentList "/VERYSILENT /SUPPRESSMSGBOXES /NORESTART /DIR=`"$tree`"" -Wait -PassThru
  if ($process.ExitCode -ne 0) { throw "Failed to extract target installer" }
  Get-ChildItem $tree -Filter "unins*" -File | Remove-Item
  $iss = Join-Path $root "delta.iss"
  $compiler = Join-Path ${env:ProgramFiles(x86)} "Inno Setup 6\ISCC.exe"
  Write-InnoSetupScript -Path $iss -AppName Nexus -ExecutableFileName Nexus.exe -Version "0.2.8" -InstallerName NexusSetup -OutputDir $root -StagedAppDir $tree -ReadmePath unused -MetadataPath unused -IconPath (Resolve-Path "desktop/windows/Nexus.Desktop/Resources/AppIcon.ico") -WebView2BootstrapperPath unused
  Write-Host "==> [delta] Building delta installer stub"
  Build-DeltaInstaller $tree $compiler $iss $root
  Write-Host "==> [delta] Running build-package-deltas.py"
  python scripts/desktop/build-package-deltas.py $new --previous $fixtures["0.2.7-88"] --windows-tree $tree
  if ($LASTEXITCODE -ne 0) { throw "Windows delta replay failed" }
} finally {
  Remove-Item -LiteralPath $root -Recurse -Force -ErrorAction SilentlyContinue
}
