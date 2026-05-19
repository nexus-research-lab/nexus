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
  [int]$SmokeTimeoutSeconds = 75,
  [switch]$SkipBuild,
  [switch]$SkipSmoke,
  [switch]$SkipInstaller
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
Use -SkipInstaller to build only the portable zip.
"@
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
  [string]$IconPath
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
ArchitecturesAllowed=x64
ArchitecturesInstallIn64BitMode=x64
ChangesAssociations=yes
CloseApplications=yes

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Additional icons:"; Flags: unchecked

[Files]
Source: "$escapedSourceAppGlob"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "$escapedReadmePath"; DestDir: "{app}"; DestName: "PACKAGE-README.txt"; Flags: ignoreversion
Source: "$escapedMetadataPath"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\$escapedAppName"; Filename: "{app}\$escapedExecutableFileName"; WorkingDir: "{app}"; IconFilename: "{app}\$escapedExecutableFileName"
Name: "{autodesktop}\$escapedAppName"; Filename: "{app}\$escapedExecutableFileName"; WorkingDir: "{app}"; IconFilename: "{app}\$escapedExecutableFileName"; Tasks: desktopicon

[Registry]
Root: HKCU; Subkey: "Software\Classes\nexus"; ValueType: string; ValueName: ""; ValueData: "URL:Nexus Protocol"; Flags: uninsdeletekey
Root: HKCU; Subkey: "Software\Classes\nexus"; ValueType: string; ValueName: "URL Protocol"; ValueData: ""; Flags: uninsdeletevalue
Root: HKCU; Subkey: "Software\Classes\nexus\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\$escapedExecutableFileName,0"; Flags: uninsdeletekey
Root: HKCU; Subkey: "Software\Classes\nexus\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\$escapedExecutableFileName"" ""%1"""; Flags: uninsdeletekey

[Run]
Filename: "{app}\$escapedExecutableFileName"; Description: "Launch $escapedAppName"; Flags: nowait postinstall skipifsilent

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
  if not IsWebView2Installed() then
  begin
    MsgBox('Microsoft Edge WebView2 Runtime is required by Nexus. Install it before launching the app if the WebView does not open.', mbInformation, MB_OK);
  end;
end;
"@ | Set-Content -Encoding UTF8 -Path $Path
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
if ([string]::IsNullOrWhiteSpace($InstallerName)) {
  $InstallerName = "$($AppName)Setup-$appVersion-$resolvedBuildNumber"
}

$stagingRoot = Join-Path $OutputDir "staging"
$stagingDir = Join-Path $stagingRoot $PackageName
$stagedAppDir = Join-Path $stagingDir $AppName
$artifactPath = Join-Path $OutputDir "$PackageName.zip"
$sha256Path = "$artifactPath.sha256"
if ([string]::IsNullOrWhiteSpace($InstallerOutputPath)) {
  $InstallerOutputPath = Join-Path $OutputDir "$InstallerName.exe"
}
$installerSha256Path = "$InstallerOutputPath.sha256"
$installerScriptPath = Join-Path $OutputDir "$PackageName.installer.iss"
if ([string]::IsNullOrWhiteSpace($MetadataPath)) {
  $MetadataPath = "$artifactPath.metadata.json"
}
$metadataStagingPath = Join-Path $stagingDir "PACKAGE-METADATA.json"
$buildInstaller = -not $SkipInstaller -and $env:NEXUS_DESKTOP_PACKAGE_SKIP_INSTALLER -ne "1"

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

foreach ($path in @($stagingDir, $artifactPath, $sha256Path, $MetadataPath, $InstallerOutputPath, $installerSha256Path, $installerScriptPath)) {
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

This package is an unsigned portable zip. The release also provides an unsigned installer when built without -SkipInstaller.
After verifying the sha256 file, unzip the package and run:
  $AppName\$executableFileName

WebView2 Runtime is required. The app stores local data under:
  ~/.nexus

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
    installer = $buildInstaller
  }
  credentials = [ordered]@{
    expected_storage = "dpapi"
    fallback_storage = "file"
  }
  artifact = [ordered]@{
    name = $PackageName
    format = "zip"
  }
  installer_artifact = [ordered]@{
    built = $buildInstaller
    name = $InstallerName
    file = (Split-Path -Leaf $InstallerOutputPath)
    format = "exe"
    tool = "inno-setup"
    code_signed = $false
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

if ($buildInstaller) {
  $compilerPath = Resolve-InnoSetupCompiler $InnoSetupCompiler
  $iconPath = Join-Path $windowsDir "Nexus.Desktop/Resources/AppIcon.ico"
  if (-not (Test-Path -LiteralPath $iconPath)) {
    throw "Missing Windows installer icon: $iconPath"
  }

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
    -IconPath $iconPath

  Write-Host "==> Building Windows installer"
  & $compilerPath "/Qp" $installerScriptPath
  if ($LASTEXITCODE -ne 0) {
    throw "Inno Setup compiler failed with exit code $LASTEXITCODE"
  }
  if (-not (Test-Path -LiteralPath $InstallerOutputPath)) {
    throw "Missing Windows installer artifact: $InstallerOutputPath"
  }

  $installerHash = (Get-FileHash -Algorithm SHA256 $InstallerOutputPath).Hash.ToLowerInvariant()
  "$installerHash  $(Split-Path -Leaf $InstallerOutputPath)" | Set-Content -Encoding ASCII -Path $installerSha256Path
}

Write-Host "Windows app zip: $artifactPath"
Write-Host "sha256: $sha256Path"
Write-Host "metadata: $MetadataPath"
if ($buildInstaller) {
  Write-Host "installer: $InstallerOutputPath"
  Write-Host "installer sha256: $installerSha256Path"
}
