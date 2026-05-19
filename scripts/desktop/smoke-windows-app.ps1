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

$logPath = Join-Path $env:LOCALAPPDATA "Nexus/Logs/shell.log"
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $logPath) | Out-Null
$marker = "windows_smoke_$([Guid]::NewGuid().ToString('N'))"
Add-Content -Path $logPath -Value "[$marker] smoke_start"

Write-Host "==> Starting $appExe"
$process = Start-Process -FilePath $appExe -WorkingDirectory $AppDir -PassThru

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

  Write-Host "==> Closing app"
  [void]$process.CloseMainWindow()
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
}

Write-Host "==> Windows app smoke passed"
