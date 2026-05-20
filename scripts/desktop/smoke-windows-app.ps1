param(
  [string]$AppDir = "",
  [string]$ExecutableName = "Nexus.exe",
  [int]$TimeoutSeconds = 75
)

$ErrorActionPreference = "Stop"

function Resolve-RootDir {
  $scriptDir = Split-Path -Parent $PSCommandPath
  return (Resolve-Path (Join-Path $scriptDir "../..")).Path
}

function Wait-Until([scriptblock]$Condition, [int]$TimeoutSeconds, [string]$Description) {
  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  while ((Get-Date) -lt $deadline) {
    if (& $Condition) {
      return
    }
    Start-Sleep -Milliseconds 300
  }
  throw "Timed out waiting for $Description"
}

function Read-Log([string]$Path) {
  if (-not (Test-Path $Path)) {
    return ""
  }
  return (Get-Content -Raw -ErrorAction SilentlyContinue $Path)
}

function Find-SidecarProcess([int]$ParentPid, [string]$AppDir) {
  return Get-CimInstance Win32_Process -Filter "Name = 'nexus-server.exe'" |
    Where-Object {
      $_.ParentProcessId -eq $ParentPid -or
      ($_.ExecutablePath -and $_.ExecutablePath.StartsWith($AppDir, [System.StringComparison]::OrdinalIgnoreCase)) -or
      ($_.CommandLine -and $_.CommandLine.Contains($AppDir, [System.StringComparison]::OrdinalIgnoreCase))
    }
}

$rootDir = Resolve-RootDir
if ([string]::IsNullOrWhiteSpace($AppDir)) {
  $AppDir = Join-Path $rootDir "desktop/windows/.build/app/Nexus"
}

$appExe = Join-Path $AppDir $ExecutableName
if (-not (Test-Path $appExe)) {
  throw "Missing Windows app executable: $appExe"
}

$logPath = Join-Path ([Environment]::GetFolderPath([System.Environment+SpecialFolder]::UserProfile)) ".nexus/logs/shell.log"
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $logPath) | Out-Null
$marker = "windows_smoke_$([Guid]::NewGuid().ToString('N'))"
Add-Content -Path $logPath -Value "[$marker] smoke_start"

$previousDisableUpdateCheck = $env:NEXUS_DESKTOP_DISABLE_UPDATE_CHECK
try {
  $env:NEXUS_DESKTOP_DISABLE_UPDATE_CHECK = "1"
  Write-Host "==> Starting $appExe"
  $process = Start-Process -FilePath $appExe -WorkingDirectory $AppDir -PassThru
} finally {
  if ($null -eq $previousDisableUpdateCheck) {
    Remove-Item Env:NEXUS_DESKTOP_DISABLE_UPDATE_CHECK -ErrorAction SilentlyContinue
  } else {
    $env:NEXUS_DESKTOP_DISABLE_UPDATE_CHECK = $previousDisableUpdateCheck
  }
}

try {
  Wait-Until {
    $log = Read-Log $logPath
    $markerIndex = $log.LastIndexOf("[$marker] smoke_start", [System.StringComparison]::Ordinal)
    if ($markerIndex -lt 0) {
      return $false
    }
    $current = $log.Substring($markerIndex)
    return $current.Contains("event=sidecar.health_ready") -and
      ($current.Contains("event=main_window.route_load") -and $current.Contains("path=/")) -and
      ($current.Contains("event=web.ready") -and $current.Contains("location_path=/"))
  } $TimeoutSeconds "launcher web.ready"

  $sidecars = @(Find-SidecarProcess $process.Id $AppDir)
  if ($sidecars.Count -eq 0) {
    throw "Expected bundled nexus-server.exe sidecar process"
  }

  Write-Host "==> Closing app to tray"
  [void]$process.CloseMainWindow()
  Wait-Until {
    $log = Read-Log $logPath
    $markerIndex = $log.LastIndexOf("[$marker] smoke_start", [System.StringComparison]::Ordinal)
    if ($markerIndex -lt 0) {
      return $false
    }
    $current = $log.Substring($markerIndex)
    return $current.Contains("event=main_window.hidden_to_tray")
  } 10 "window hidden to tray"

  $process.Refresh()
  if ($process.HasExited) {
    throw "Expected window close to keep Nexus running in the tray"
  }

  Write-Host "==> Exiting app"
  $exitProcess = Start-Process -FilePath $appExe -WorkingDirectory $AppDir -ArgumentList "--nexus-desktop-exit" -PassThru
  [void]$exitProcess.WaitForExit(5000)
  Wait-Until {
    $process.Refresh()
    return $process.HasExited
  } 20 "app exit"

  Wait-Until {
    return @(Find-SidecarProcess $process.Id $AppDir).Count -eq 0
  } 15 "sidecar cleanup"
} finally {
  if (-not $process.HasExited) {
    Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
  }
  foreach ($sidecar in @(Find-SidecarProcess $process.Id $AppDir)) {
    Stop-Process -Id $sidecar.ProcessId -Force -ErrorAction SilentlyContinue
  }
  Get-CimInstance Win32_Process -Filter "Name = 'msedgewebview2.exe'" |
    Where-Object { $_.CommandLine -and $_.CommandLine.IndexOf("Nexus\cache\WebView2", [System.StringComparison]::OrdinalIgnoreCase) -ge 0 } |
    ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
}

Write-Host "==> Windows app smoke passed"
