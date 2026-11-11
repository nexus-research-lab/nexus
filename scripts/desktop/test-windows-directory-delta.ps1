# 对两份真实发行版执行与发布流程相同的重建与 Inno 覆盖安装。
$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "windows-inno.ps1")
$root = Join-Path $env:RUNNER_TEMP ("directory-delta-" + [Guid]::NewGuid())
New-Item -ItemType Directory $root | Out-Null
try {
  foreach ($version in @("0.2.7-88", "0.2.8-89")) {
    $name = "NexusSetup-$version.exe"
    gh release download ("v" + $version.Split("-")[0]) -p $name -p "$name.sha256" -D $root
    if ($LASTEXITCODE -ne 0) { throw "Failed to download $name" }
    $sum = ((Get-Content (Join-Path $root "$name.sha256") -Raw).Trim() -split '\s+')[0]
    if ((Get-FileHash (Join-Path $root $name)).Hash -ne $sum) { throw "Invalid $name" }
  }
  $tree = Join-Path $root "next"
  $new = Join-Path $root "NexusSetup-0.2.8-89.exe"
  $process = Start-Process -FilePath $new -ArgumentList "/VERYSILENT /SUPPRESSMSGBOXES /NORESTART /DIR=`"$tree`"" -Wait -PassThru
  if ($process.ExitCode -ne 0) { throw "Failed to extract target installer" }
  Get-ChildItem $tree -Filter "unins*" -File | Remove-Item
  $iss = Join-Path $root "delta.iss"
  $compiler = Join-Path ${env:ProgramFiles(x86)} "Inno Setup 6\ISCC.exe"
  Write-InnoSetupScript -Path $iss -AppName Nexus -ExecutableFileName Nexus.exe -Version "0.2.8" -InstallerName NexusSetup -OutputDir $root -StagedAppDir $tree -ReadmePath unused -MetadataPath unused -IconPath (Resolve-Path "desktop/windows/Nexus.Desktop/Resources/AppIcon.ico") -WebView2BootstrapperPath unused
  Build-DeltaInstaller $tree $compiler $iss $root
  python scripts/desktop/build-package-deltas.py $new --previous (Join-Path $root "NexusSetup-0.2.7-88.exe") --windows-tree $tree
  if ($LASTEXITCODE -ne 0) { throw "Windows delta replay failed" }
} finally {
  Remove-Item -LiteralPath $root -Recurse -Force -ErrorAction SilentlyContinue
}
