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
  [string]$InstallerName = $env:NEXUS_DESKTOP_INSTALLER_NAME,
  [string]$InstallerOutputPath = $env:NEXUS_DESKTOP_INSTALLER_OUTPUT_PATH,
  [string]$InnoSetupCompiler = $env:NEXUS_INNO_SETUP_COMPILER,
  [string]$SelfContained = $env:NEXUS_DESKTOP_SELF_CONTAINED,
  [string]$WebView2BootstrapperPath = $env:NEXUS_WEBVIEW2_BOOTSTRAPPER_PATH,
  [string]$WebView2BootstrapperUrl = $(if ($env:NEXUS_WEBVIEW2_BOOTSTRAPPER_URL) { $env:NEXUS_WEBVIEW2_BOOTSTRAPPER_URL } else { "https://go.microsoft.com/fwlink/p/?LinkId=2124703" }),
  [string]$SigningCertificatePath = $env:NEXUS_WINDOWS_SIGNING_CERTIFICATE_PATH,
  [string]$SigningCertificateBase64 = $env:NEXUS_WINDOWS_SIGNING_CERT_PFX_BASE64,
  [string]$SigningCertificatePassword = $env:NEXUS_WINDOWS_SIGNING_CERT_PASSWORD,
  [string]$TimestampServer = $(if ($env:NEXUS_WINDOWS_TIMESTAMP_SERVER) { $env:NEXUS_WINDOWS_TIMESTAMP_SERVER } else { "http://timestamp.digicert.com" }),
  [string]$SignToolPath = $env:NEXUS_SIGNTOOL_PATH,
  [int]$SmokeTimeoutSeconds = 75,
  [switch]$SkipBuild,
  [switch]$SkipSmoke,
  [switch]$SkipSigning,
  [switch]$SkipWebView2Bootstrapper
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

function Assert-SupportedRuntimeIdentifier([string]$runtimeIdentifier) {
  if ($runtimeIdentifier -ne "win-x64") {
    throw "Unsupported Windows RuntimeIdentifier '$runtimeIdentifier'. Current installer and sidecar package only support win-x64."
  }
}

function Resolve-InnoSetupCompiler([string]$compilerPath) {
  if (-not [string]::IsNullOrWhiteSpace($compilerPath)) {
    if (Test-Path -LiteralPath $compilerPath) {
      return (Resolve-Path -LiteralPath $compilerPath).Path
    }

    $command = Get-Command $compilerPath -ErrorAction SilentlyContinue
    if ($command) {
      return $command.Source
    }
  }

  foreach ($commandName in @("ISCC.exe", "iscc.exe")) {
    $command = Get-Command $commandName -ErrorAction SilentlyContinue
    if ($command) {
      return $command.Source
    }
  }

  foreach ($candidate in @(
      (Join-Path $env:LOCALAPPDATA "Programs/Inno Setup 6/ISCC.exe"),
      (Join-Path ${env:ProgramFiles(x86)} "Inno Setup 6/ISCC.exe"),
      (Join-Path $env:ProgramFiles "Inno Setup 6/ISCC.exe")
    )) {
    if (Test-Path -LiteralPath $candidate) {
      return (Resolve-Path -LiteralPath $candidate).Path
    }
  }

  throw @"
Inno Setup 6 compiler is required to build the Windows installer.
Install it with:
  winget install --id JRSoftware.InnoSetup -e
or set NEXUS_INNO_SETUP_COMPILER to ISCC.exe.
"@
}

function Resolve-WebView2Bootstrapper(
  [string]$bootstrapperPath,
  [string]$bootstrapperUrl,
  [string]$outputDir,
  [bool]$skipDownload
) {
  if ($skipDownload) {
    return ""
  }

  if (-not [string]::IsNullOrWhiteSpace($bootstrapperPath)) {
    if (-not (Test-Path -LiteralPath $bootstrapperPath)) {
      throw "Missing WebView2 bootstrapper: $bootstrapperPath"
    }
    return (Resolve-Path -LiteralPath $bootstrapperPath).Path
  }

  if ([string]::IsNullOrWhiteSpace($bootstrapperUrl)) {
    throw "WebView2 bootstrapper URL is empty. Set NEXUS_WEBVIEW2_BOOTSTRAPPER_URL."
  }

  New-Item -ItemType Directory -Force -Path $outputDir | Out-Null
  $downloadPath = Join-Path $outputDir "MicrosoftEdgeWebView2Setup.exe"
  Write-Host "==> Downloading WebView2 Evergreen bootstrapper"
  Invoke-WebRequest -Uri $bootstrapperUrl -OutFile $downloadPath
  return (Resolve-Path -LiteralPath $downloadPath).Path
}

function Resolve-SignTool([string]$signToolPath) {
  if (-not [string]::IsNullOrWhiteSpace($signToolPath)) {
    if (Test-Path -LiteralPath $signToolPath) {
      return (Resolve-Path -LiteralPath $signToolPath).Path
    }

    $command = Get-Command $signToolPath -ErrorAction SilentlyContinue
    if ($command) {
      return $command.Source
    }
  }

  foreach ($commandName in @("signtool.exe", "signtool")) {
    $command = Get-Command $commandName -ErrorAction SilentlyContinue
    if ($command) {
      return $command.Source
    }
  }

  $kitRoots = @()
  if (-not [string]::IsNullOrWhiteSpace(${env:ProgramFiles(x86)})) {
    $kitRoots += (Join-Path ${env:ProgramFiles(x86)} "Windows Kits/10/bin")
  }
  if (-not [string]::IsNullOrWhiteSpace($env:ProgramFiles)) {
    $kitRoots += (Join-Path $env:ProgramFiles "Windows Kits/10/bin")
  }
  $kitRoots = $kitRoots | Where-Object { Test-Path -LiteralPath $_ }

  foreach ($root in $kitRoots) {
    $candidate = Get-ChildItem -Path $root -Filter "signtool.exe" -Recurse -ErrorAction SilentlyContinue |
      Where-Object { $_.FullName -match "\\x64\\signtool\.exe$" } |
      Sort-Object FullName -Descending |
      Select-Object -First 1
    if ($candidate) {
      return $candidate.FullName
    }
  }

  throw "signtool.exe is required when Windows signing certificate variables are configured."
}

function Resolve-SigningCertificatePath(
  [string]$certificatePath,
  [string]$certificateBase64,
  [string]$outputDir
) {
  if (-not [string]::IsNullOrWhiteSpace($certificatePath)) {
    if (-not (Test-Path -LiteralPath $certificatePath)) {
      throw "Missing Windows signing certificate: $certificatePath"
    }
    return (Resolve-Path -LiteralPath $certificatePath).Path
  }

  if ([string]::IsNullOrWhiteSpace($certificateBase64)) {
    return ""
  }

  New-Item -ItemType Directory -Force -Path $outputDir | Out-Null
  $certificateOutputPath = Join-Path $outputDir "windows-signing-cert.pfx"
  [System.IO.File]::WriteAllBytes($certificateOutputPath, [Convert]::FromBase64String($certificateBase64))
  return (Resolve-Path -LiteralPath $certificateOutputPath).Path
}

function Invoke-WindowsSigning(
  [string]$path,
  [string]$toolPath,
  [string]$certificatePath,
  [string]$certificatePassword,
  [string]$timestampServer
) {
  if ([string]::IsNullOrWhiteSpace($certificatePath)) {
    return
  }

  if (-not (Test-Path -LiteralPath $path)) {
    throw "Missing file to sign: $path"
  }

  $arguments = @("sign", "/fd", "SHA256")
  if (-not [string]::IsNullOrWhiteSpace($timestampServer)) {
    $arguments += @("/tr", $timestampServer, "/td", "SHA256")
  }
  $arguments += @("/f", $certificatePath)
  if (-not [string]::IsNullOrWhiteSpace($certificatePassword)) {
    $arguments += @("/p", $certificatePassword)
  }
  $arguments += $path

  Write-Host "==> Signing $(Split-Path -Leaf $path)"
  & $toolPath @arguments
  if ($LASTEXITCODE -ne 0) {
    throw "signtool failed for $path with exit code $LASTEXITCODE"
  }
}

function ConvertTo-InnoValue([string]$value) {
  return $value.Replace('"', '""')
}

function Write-InnoSetupScript(
  [string]$Path,
  [string]$AppName,
  [string]$ExecutableFileName,
  [string]$Version,
  [string]$InstallerName,
  [string]$OutputDir,
  [string]$StagedAppDir,
  [string]$ReadmePath,
  [string]$MetadataPath,
  [string]$IconPath,
  [string]$WebView2BootstrapperPath
) {
  $escapedAppName = ConvertTo-InnoValue $AppName
  $escapedExecutableFileName = ConvertTo-InnoValue $ExecutableFileName
  $escapedVersion = ConvertTo-InnoValue $Version
  $escapedInstallerName = ConvertTo-InnoValue $InstallerName
  $escapedOutputDir = ConvertTo-InnoValue $OutputDir
  $escapedSourceAppGlob = ConvertTo-InnoValue (Join-Path $StagedAppDir "*")
  $escapedReadmePath = ConvertTo-InnoValue $ReadmePath
  $escapedMetadataPath = ConvertTo-InnoValue $MetadataPath
  $escapedIconPath = ConvertTo-InnoValue $IconPath
  $escapedWebView2BootstrapperPath = ConvertTo-InnoValue $WebView2BootstrapperPath

  @"
[Setup]
AppId=Nexus.Desktop
AppName=$escapedAppName
AppVersion=$escapedVersion
AppVerName=$escapedAppName $escapedVersion
AppPublisher=Nexus Research Lab
AppPublisherURL=https://github.com/nexus-research-lab/nexus
AppSupportURL=https://github.com/nexus-research-lab/nexus/issues
DefaultDirName={localappdata}\Programs\$escapedAppName
DefaultGroupName=$escapedAppName
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
OutputDir=$escapedOutputDir
OutputBaseFilename=$escapedInstallerName
SetupIconFile=$escapedIconPath
UninstallDisplayIcon={app}\$escapedExecutableFileName
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
ChangesAssociations=yes
CloseApplications=yes

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Additional icons:"; Flags: unchecked

[Files]
Source: "$escapedSourceAppGlob"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "$escapedReadmePath"; DestDir: "{app}"; DestName: "PACKAGE-README.txt"; Flags: ignoreversion
Source: "$escapedMetadataPath"; DestDir: "{app}"; Flags: ignoreversion
Source: "$escapedWebView2BootstrapperPath"; DestDir: "{tmp}"; DestName: "MicrosoftEdgeWebView2Setup.exe"; Flags: ignoreversion deleteafterinstall; Check: not IsWebView2Installed

[Icons]
Name: "{autoprograms}\$escapedAppName"; Filename: "{app}\$escapedExecutableFileName"; WorkingDir: "{app}"; IconFilename: "{app}\$escapedExecutableFileName"
Name: "{autodesktop}\$escapedAppName"; Filename: "{app}\$escapedExecutableFileName"; WorkingDir: "{app}"; IconFilename: "{app}\$escapedExecutableFileName"; Tasks: desktopicon

[Registry]
Root: HKCU; Subkey: "Software\Classes\nexus"; ValueType: string; ValueName: ""; ValueData: "URL:Nexus Protocol"; Flags: uninsdeletekey
Root: HKCU; Subkey: "Software\Classes\nexus"; ValueType: string; ValueName: "URL Protocol"; ValueData: ""; Flags: uninsdeletevalue
Root: HKCU; Subkey: "Software\Classes\nexus\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\$escapedExecutableFileName,0"; Flags: uninsdeletekey
Root: HKCU; Subkey: "Software\Classes\nexus\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\$escapedExecutableFileName"" ""%1"""; Flags: uninsdeletekey

[Run]
Filename: "{tmp}\MicrosoftEdgeWebView2Setup.exe"; Parameters: "/silent /install"; StatusMsg: "Installing Microsoft Edge WebView2 Runtime..."; Flags: waituntilterminated; Check: not IsWebView2Installed
Filename: "{app}\$escapedExecutableFileName"; Description: "Launch $escapedAppName"; Flags: nowait postinstall skipifsilent; Check: IsWebView2Installed

[Code]
function IsWebView2Installed(): Boolean;
var
  Version: String;
begin
  Result :=
    RegQueryStringValue(HKLM, 'SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}', 'pv', Version) or
    RegQueryStringValue(HKLM32, 'SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}', 'pv', Version) or
    RegQueryStringValue(HKCU, 'SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}', 'pv', Version);
end;

function InitializeSetup(): Boolean;
begin
  Result := True;
end;
"@ | Set-Content -Encoding UTF8 -Path $Path
}

if ($env:NEXUS_DESKTOP_SMOKE_TIMEOUT_SECONDS) {
  $SmokeTimeoutSeconds = [int]$env:NEXUS_DESKTOP_SMOKE_TIMEOUT_SECONDS
}

$rootDir = Resolve-RootDir
$windowsDir = Join-Path $rootDir "desktop/windows"
Assert-SupportedRuntimeIdentifier $RuntimeIdentifier
$appVersion = Resolve-AppVersion $rootDir $Version
$resolvedBuildNumber = Resolve-BuildNumber $rootDir $BuildNumber
$packageSelfContained = Resolve-Bool $SelfContained $true
$packageSelfContainedValue = $packageSelfContained.ToString().ToLowerInvariant()
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
if ([string]::IsNullOrWhiteSpace($InstallerName)) {
  $InstallerName = "$($AppName)Setup-$appVersion-$resolvedBuildNumber"
}

$stagingRoot = Join-Path $OutputDir "staging"
$stagingDir = Join-Path $stagingRoot $PackageName
$stagedAppDir = Join-Path $stagingDir $AppName
if ([string]::IsNullOrWhiteSpace($InstallerOutputPath)) {
  $InstallerOutputPath = Join-Path $OutputDir "$InstallerName.exe"
}
$installerSha256Path = "$InstallerOutputPath.sha256"
$installerScriptPath = Join-Path $OutputDir "$PackageName.installer.iss"
if ([string]::IsNullOrWhiteSpace($MetadataPath)) {
  $MetadataPath = Join-Path $OutputDir "$PackageName.metadata.json"
}
$metadataStagingPath = Join-Path $stagingDir "PACKAGE-METADATA.json"
$signingCertificateResolvedPath = ""
$resolvedSignToolPath = ""
$signingWorkDir = Join-Path ([System.IO.Path]::GetTempPath()) ("nexus-windows-signing-" + [Guid]::NewGuid().ToString("N"))
if (-not $SkipSigning) {
  $signingCertificateResolvedPath = Resolve-SigningCertificatePath `
    -certificatePath $SigningCertificatePath `
    -certificateBase64 $SigningCertificateBase64 `
    -outputDir $signingWorkDir
  if (-not [string]::IsNullOrWhiteSpace($signingCertificateResolvedPath)) {
    $resolvedSignToolPath = Resolve-SignTool $SignToolPath
  }
}
$signingEnabled = -not [string]::IsNullOrWhiteSpace($signingCertificateResolvedPath)

if ((-not $SkipBuild) -and $env:NEXUS_DESKTOP_PACKAGE_SKIP_BUILD -ne "1") {
  & (Join-Path $rootDir "scripts/desktop/build-windows-app.ps1") `
    -Configuration $Configuration `
    -RuntimeIdentifier $RuntimeIdentifier `
    -AppName $AppName `
    -ExecutableName $ExecutableName `
    -OutputDir $AppBuildDir `
    -Version $appVersion `
    -BuildNumber $resolvedBuildNumber `
    -SelfContained $packageSelfContainedValue
}

$appExe = Join-Path $AppBuildDir $executableFileName
if (-not (Test-Path -LiteralPath $appExe)) {
  throw "Missing Windows app executable: $appExe"
}

if ($signingEnabled) {
  $shellBinaryBaseName = [System.IO.Path]::GetFileNameWithoutExtension($executableFileName)
  $signTargets = @(
    $appExe,
    (Join-Path $AppBuildDir "$shellBinaryBaseName.dll"),
    (Join-Path $AppBuildDir "Resources/nexus-server.exe")
  ) | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -Unique

  foreach ($signTarget in $signTargets) {
    Invoke-WindowsSigning `
      -path $signTarget `
      -toolPath $resolvedSignToolPath `
      -certificatePath $signingCertificateResolvedPath `
      -certificatePassword $SigningCertificatePassword `
      -timestampServer $TimestampServer
  }
}

if ((-not $SkipSmoke) -and $env:NEXUS_DESKTOP_PACKAGE_SKIP_SMOKE -ne "1") {
  & (Join-Path $rootDir "scripts/desktop/smoke-windows-app.ps1") `
    -AppDir $AppBuildDir `
    -ExecutableName $executableFileName `
    -TimeoutSeconds $SmokeTimeoutSeconds
}

foreach ($path in @($stagingDir, $MetadataPath, $InstallerOutputPath, $installerSha256Path, $installerScriptPath)) {
  if (Test-Path -LiteralPath $path) {
    Remove-Item -LiteralPath $path -Recurse -Force
  }
}
New-Item -ItemType Directory -Force -Path $stagedAppDir | Out-Null
New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
Get-ChildItem -LiteralPath $OutputDir -Filter "$AppName-windows-*.zip*" -File -ErrorAction SilentlyContinue |
  Remove-Item -Force

Copy-Item -Recurse -Force -Path (Join-Path $AppBuildDir "*") -Destination $stagedAppDir

$readmePath = Join-Path $stagingDir "README.txt"
$installerSigningLabel = if ($signingEnabled) { "Authenticode-signed" } else { "unsigned" }
@"
Nexus Windows app package

Version: $appVersion
Build: $resolvedBuildNumber
Commit: $commitShort
Created: $createdAt

This installation was produced by a $installerSigningLabel Inno Setup installer:
  $InstallerName.exe

The installer bundles the WebView2 Evergreen bootstrapper. The app stores local data under:
  ~/.nexus

To reset app data, quit Nexus first, then remove that directory.
"@ | Set-Content -Encoding UTF8 -Path $readmePath

$webView2BootstrapperUrlForMetadata = $null
if (-not $SkipWebView2Bootstrapper) {
  $webView2BootstrapperUrlForMetadata = $WebView2BootstrapperUrl
}
$signingKind = "unsigned"
$signingTimestampServer = $null
if ($signingEnabled) {
  $signingKind = "authenticode"
  $signingTimestampServer = $TimestampServer
}

$metadata = [ordered]@{
  app_name = $AppName
  executable_name = $executableFileName
  platform = "windows"
  runtime_identifier = $RuntimeIdentifier
  version = $appVersion
  build_number = $resolvedBuildNumber
  created_at = $createdAt
  runtime = [ordered]@{
    dotnet_self_contained = $packageSelfContained
    runtime_identifier = $RuntimeIdentifier
    supported_runtime_identifiers = @("win-x64")
  }
  dependencies = [ordered]@{
    webview2 = [ordered]@{
      installer_bootstrapper = $true
      bootstrapper_url = $webView2BootstrapperUrlForMetadata
    }
  }
  source = [ordered]@{
    commit = $commitSha
    short_commit = $commitShort
    dirty = $sourceDirty
  }
  signing = [ordered]@{
    kind = $signingKind
    code_signed = $signingEnabled
    installer = $true
    timestamp_server = $signingTimestampServer
  }
  credentials = [ordered]@{
    expected_storage = "dpapi"
    fallback_storage = "file"
  }
  artifact = [ordered]@{
    name = $InstallerName
    format = "exe"
    kind = "installer"
  }
  installer_artifact = [ordered]@{
    built = $true
    name = $InstallerName
    file = (Split-Path -Leaf $InstallerOutputPath)
    format = "exe"
    tool = "inno-setup"
    code_signed = $signingEnabled
  }
  validation = [ordered]@{
    build_script = "scripts/desktop/build-windows-app.ps1"
    smoke_script = "scripts/desktop/smoke-windows-app.ps1"
    smoke_skipped = [bool]($SkipSmoke -or ($env:NEXUS_DESKTOP_PACKAGE_SKIP_SMOKE -eq "1"))
  }
}

$metadata | ConvertTo-Json -Depth 8 | Set-Content -Encoding UTF8 -Path $metadataStagingPath
Copy-Item -Force -LiteralPath $metadataStagingPath -Destination $MetadataPath

if ($SkipWebView2Bootstrapper) {
  throw "Windows installer requires the WebView2 Evergreen bootstrapper."
}

$compilerPath = Resolve-InnoSetupCompiler $InnoSetupCompiler
$iconPath = Join-Path $windowsDir "Nexus.Desktop/Resources/AppIcon.ico"
if (-not (Test-Path -LiteralPath $iconPath)) {
  throw "Missing Windows installer icon: $iconPath"
}
$resolvedWebView2BootstrapperPath = Resolve-WebView2Bootstrapper `
  -bootstrapperPath $WebView2BootstrapperPath `
  -bootstrapperUrl $WebView2BootstrapperUrl `
  -outputDir $OutputDir `
  -skipDownload $false
$removeResolvedWebView2Bootstrapper = [string]::IsNullOrWhiteSpace($WebView2BootstrapperPath)

New-Item -ItemType Directory -Force -Path (Split-Path -Parent $InstallerOutputPath) | Out-Null
Write-InnoSetupScript `
  -Path $installerScriptPath `
  -AppName $AppName `
  -ExecutableFileName $executableFileName `
  -Version $appVersion `
  -InstallerName $InstallerName `
  -OutputDir (Split-Path -Parent $InstallerOutputPath) `
  -StagedAppDir $stagedAppDir `
  -ReadmePath $readmePath `
  -MetadataPath $metadataStagingPath `
  -IconPath $iconPath `
  -WebView2BootstrapperPath $resolvedWebView2BootstrapperPath

Write-Host "==> Building Windows installer"
& $compilerPath "/Qp" $installerScriptPath
if ($LASTEXITCODE -ne 0) {
  throw "Inno Setup compiler failed with exit code $LASTEXITCODE"
}
if (-not (Test-Path -LiteralPath $InstallerOutputPath)) {
  throw "Missing Windows installer artifact: $InstallerOutputPath"
}

if ($signingEnabled) {
  Invoke-WindowsSigning `
    -path $InstallerOutputPath `
    -toolPath $resolvedSignToolPath `
    -certificatePath $signingCertificateResolvedPath `
    -certificatePassword $SigningCertificatePassword `
    -timestampServer $TimestampServer
}

$installerHash = (Get-FileHash -Algorithm SHA256 $InstallerOutputPath).Hash.ToLowerInvariant()
"$installerHash  $(Split-Path -Leaf $InstallerOutputPath)" | Set-Content -Encoding ASCII -Path $installerSha256Path

Write-Host "metadata: $MetadataPath"
Write-Host "installer: $InstallerOutputPath"
Write-Host "installer sha256: $installerSha256Path"

Remove-Item -Recurse -Force $stagingRoot -ErrorAction SilentlyContinue
Remove-Item -Force $installerScriptPath -ErrorAction SilentlyContinue
if ($removeResolvedWebView2Bootstrapper) {
  Remove-Item -Force $resolvedWebView2BootstrapperPath -ErrorAction SilentlyContinue
}
Remove-Item -Recurse -Force $signingWorkDir -ErrorAction SilentlyContinue
