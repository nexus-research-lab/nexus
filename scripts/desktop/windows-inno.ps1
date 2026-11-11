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
#ifdef DeltaFiles
OutputDir={#DeltaOutput}
OutputBaseFilename=delta-install
#else
OutputDir=$escapedOutputDir
OutputBaseFilename=$escapedInstallerName
#endif
SetupIconFile=$escapedIconPath
UninstallDisplayIcon={app}\$escapedExecutableFileName
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
ChangesAssociations=yes
CloseApplications=yes
RestartApplications=no

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Additional icons:"; Flags: unchecked

[InstallDelete]
Type: filesandordirs; Name: "{app}\Resources\db\migrations"

[Files]
#ifdef DeltaFiles
#include DeltaFiles
#else
Source: "$escapedSourceAppGlob"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "$escapedReadmePath"; DestDir: "{app}"; DestName: "PACKAGE-README.txt"; Flags: ignoreversion
Source: "$escapedMetadataPath"; DestDir: "{app}"; Flags: ignoreversion
Source: "$escapedWebView2BootstrapperPath"; DestDir: "{tmp}"; DestName: "MicrosoftEdgeWebView2Setup.exe"; Flags: ignoreversion deleteafterinstall; Check: not IsWebView2Installed
#endif

[Icons]
Name: "{autoprograms}\$escapedAppName"; Filename: "{app}\$escapedExecutableFileName"; WorkingDir: "{app}"; IconFilename: "{app}\$escapedExecutableFileName"
Name: "{autodesktop}\$escapedAppName"; Filename: "{app}\$escapedExecutableFileName"; WorkingDir: "{app}"; IconFilename: "{app}\$escapedExecutableFileName"; Tasks: desktopicon

[Registry]
Root: HKCU; Subkey: "Software\Classes\nexus"; ValueType: string; ValueName: ""; ValueData: "URL:Nexus Protocol"; Flags: uninsdeletekey
Root: HKCU; Subkey: "Software\Classes\nexus"; ValueType: string; ValueName: "URL Protocol"; ValueData: ""; Flags: uninsdeletevalue
Root: HKCU; Subkey: "Software\Classes\nexus\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\$escapedExecutableFileName,0"; Flags: uninsdeletekey
Root: HKCU; Subkey: "Software\Classes\nexus\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\$escapedExecutableFileName"" ""%1"""; Flags: uninsdeletekey

[Run]
#ifndef DeltaFiles
Filename: "{tmp}\MicrosoftEdgeWebView2Setup.exe"; Parameters: "/silent /install"; StatusMsg: "Installing Microsoft Edge WebView2 Runtime..."; Flags: waituntilterminated; Check: not IsWebView2Installed
#endif
Filename: "{app}\$escapedExecutableFileName"; Description: "Launch $escapedAppName"; Flags: nowait postinstall skipifsilent; Check: IsWebView2Installed

[Code]
const
  NexusMutexName = 'Local\NexusDesktop';
  NexusExitArgument = '--nexus-desktop-exit';
  NexusExitWaitAttempts = 200;
  NexusExitWaitDelayMilliseconds = 100;
  NexusFileReleaseDelayMilliseconds = 500;

function IsWebView2Installed(): Boolean;
var
  Version: String;
begin
  Result :=
    RegQueryStringValue(HKLM, 'SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}', 'pv', Version) or
    RegQueryStringValue(HKLM32, 'SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}', 'pv', Version) or
    RegQueryStringValue(HKCU, 'SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}', 'pv', Version);
end;

function WaitForNexusExit(): Boolean;
var
  Attempt: Integer;
begin
  for Attempt := 1 to NexusExitWaitAttempts do
  begin
    if not CheckForMutexes(NexusMutexName) then
    begin
      { mutex 在进程退出末段释放，再留出短暂时间让 Windows 解除映像文件锁。 }
      Sleep(NexusFileReleaseDelayMilliseconds);
      Result := True;
      Exit;
    end;
    Sleep(NexusExitWaitDelayMilliseconds);
  end;

  Result := not CheckForMutexes(NexusMutexName);
end;

function RequestNexusExit(): Boolean;
var
  NexusExecutablePath: String;
  ResultCode: Integer;
begin
  if not CheckForMutexes(NexusMutexName) then
  begin
    Result := True;
    Exit;
  end;

  NexusExecutablePath := ExpandConstant('{app}\$escapedExecutableFileName');
  if not FileExists(NexusExecutablePath) then
  begin
    Log('Nexus mutex exists but the installed executable is missing: ' + NexusExecutablePath);
    Result := False;
    Exit;
  end;

  Log('Requesting the running Nexus instance to exit before installation.');
  if not Exec(
    NexusExecutablePath,
    NexusExitArgument,
    ExpandConstant('{app}'),
    SW_HIDE,
    ewWaitUntilTerminated,
    ResultCode) then
  begin
    Log('Failed to launch the Nexus exit command.');
    Result := False;
    Exit;
  end;

  Log('Nexus exit command completed with code ' + IntToStr(ResultCode) + '.');
  Result := WaitForNexusExit();
end;

function PrepareToInstall(var NeedsRestart: Boolean): String;
begin
  NeedsRestart := False;
  if RequestNexusExit() then
  begin
    Result := '';
    Exit;
  end;

  Result :=
    'Nexus 仍在运行，安装程序无法安全替换文件。' + #13#10 +
    '请从系统托盘退出 Nexus，或在任务管理器结束 Nexus.exe 后重试。';
end;
"@ | Set-Content -Encoding UTF8 -Path $Path
}

function Build-DeltaInstaller([string]$tree, [string]$compilerPath, [string]$installerScriptPath, [string]$stagingDir) {
  $include = Join-Path $stagingDir "delta-files.iss"
  $lines = foreach ($file in Get-ChildItem -LiteralPath $tree -Recurse -File) {
    $rel = [IO.Path]::GetRelativePath($tree, $file.FullName)
    if ($rel -match '[{}"#\r\n]') { throw "Invalid external file path: $rel" }
    $dest = "{app}"
    $parent = [IO.Path]::GetDirectoryName($rel)
    if ($parent) { $dest += "\" + $parent }
    $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $file.FullName).Hash
    'Source: "{{src}}\{0}"; DestDir: "{1}"; ExternalSize: {2}; Hash: "{3}"; Flags: external ignoreversion' -f $rel,$dest,$file.Length,$hash
  }
  # {src} 是 Inno 常量，PowerShell 格式化时需保留花括号。
  $lines | Set-Content -LiteralPath $include -Encoding utf8BOM
  & $compilerPath "/Qp" "/DDeltaFiles=$include" "/DDeltaOutput=$tree" $installerScriptPath
  if ($LASTEXITCODE -ne 0) { throw "Delta installer compilation failed: $LASTEXITCODE" }
}
