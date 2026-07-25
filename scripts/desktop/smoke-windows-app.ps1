param(
  [string]$AppDir = "",
  [string]$ExecutableName = "Nexus.exe",
  [int]$TimeoutSeconds = 75,
  [string]$ExpectNXSRuntime = $env:NEXUS_DESKTOP_SMOKE_EXPECT_NXS_RUNTIME
)

$ErrorActionPreference = "Stop"

Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;

public static class NexusWindowChromeProbe
{
    [StructLayout(LayoutKind.Sequential)]
    private struct Rect
    {
        internal int Left;
        internal int Top;
        internal int Right;
        internal int Bottom;
    }

    [StructLayout(LayoutKind.Sequential)]
    private struct Point
    {
        internal int X;
        internal int Y;
    }

    [DllImport("user32.dll")]
    private static extern IntPtr SetThreadDpiAwarenessContext(IntPtr context);

    [DllImport("dwmapi.dll")]
    private static extern int DwmGetWindowAttribute(IntPtr hwnd, int attribute, out Rect value, int size);

    [DllImport("user32.dll")]
    private static extern bool GetWindowRect(IntPtr hwnd, out Rect value);

    [DllImport("user32.dll")]
    private static extern IntPtr WindowFromPoint(Point point);

    [DllImport("user32.dll")]
    private static extern IntPtr SendMessage(IntPtr hwnd, uint message, IntPtr wParam, IntPtr lParam);

    [DllImport("user32.dll")]
    public static extern bool IsZoomed(IntPtr hwnd);

    public static string ValidateResizeBoundary(IntPtr hwnd)
    {
        IntPtr previousDpi = SetThreadDpiAwarenessContext(new IntPtr(-4));
        try
        {
            if (DwmGetWindowAttribute(hwnd, 9, out Rect bounds, Marshal.SizeOf<Rect>()) != 0 &&
                !GetWindowRect(hwnd, out bounds))
            {
                return "cannot read window bounds";
            }

            var edges = new[]
            {
                (Name: "left", X: bounds.Left + 2, Y: (bounds.Top + bounds.Bottom) / 2, Hit: 10),
                (Name: "right", X: bounds.Right - 2, Y: (bounds.Top + bounds.Bottom) / 2, Hit: 11),
                (Name: "top", X: (bounds.Left + bounds.Right) / 2, Y: bounds.Top + 2, Hit: 12),
                (Name: "bottom", X: (bounds.Left + bounds.Right) / 2, Y: bounds.Bottom - 2, Hit: 15),
            };
            foreach (var edge in edges)
            {
                if (WindowFromPoint(new Point { X = edge.X, Y = edge.Y }) != hwnd)
                {
                    return $"{edge.Name} edge is covered by a child HWND";
                }

                long packedPoint = ((long)(ushort)edge.Y << 16) | (ushort)edge.X;
                long actualHit = SendMessage(hwnd, 0x0084, IntPtr.Zero, new IntPtr(packedPoint)).ToInt64();
                if (actualHit != edge.Hit)
                {
                    return $"{edge.Name} edge returned hit code {actualHit}, expected {edge.Hit}";
                }
            }
            return string.Empty;
        }
        finally
        {
            if (previousDpi != IntPtr.Zero)
            {
                _ = SetThreadDpiAwarenessContext(previousDpi);
            }
        }
    }
}
'@

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

function Find-CaptionButton([IntPtr]$Hwnd, [string]$Name) {
  $root = [System.Windows.Automation.AutomationElement]::FromHandle($Hwnd)
  $condition = [System.Windows.Automation.PropertyCondition]::new(
    [System.Windows.Automation.AutomationElement]::NameProperty,
    $Name
  )
  $button = $root.FindFirst([System.Windows.Automation.TreeScope]::Descendants, $condition)
  if ($null -eq $button -or $button.Current.ControlType -ne [System.Windows.Automation.ControlType]::Button) {
    throw "Missing caption button: $Name"
  }
  if ($button.Current.IsOffscreen -or $button.Current.BoundingRectangle.IsEmpty) {
    throw "Caption button is not visible: $Name"
  }
  return $button
}

function Invoke-CaptionButton([IntPtr]$Hwnd, [string]$Name) {
  $button = Find-CaptionButton $Hwnd $Name
  $pattern = $button.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern)
  $pattern.Invoke()
}

$rootDir = Resolve-RootDir
if ([string]::IsNullOrWhiteSpace($AppDir)) {
  $AppDir = Join-Path $rootDir "desktop/windows/.build/app/Nexus"
}

$appExe = Join-Path $AppDir $ExecutableName
if (-not (Test-Path $appExe)) {
  throw "Missing Windows app executable: $appExe"
}

$nexusctlExe = Join-Path $AppDir "Resources/bin/nexusctl.exe"
if (-not (Test-Path $nexusctlExe)) {
  throw "Missing bundled nexusctl executable: $nexusctlExe"
}

& $nexusctlExe --help *> $null
if ($LASTEXITCODE -ne 0) {
  throw "Bundled nexusctl --help failed with exit code $LASTEXITCODE"
}

$nxsExpected = Resolve-Bool $ExpectNXSRuntime $false
$nxsExe = Join-Path $AppDir "Resources/bin/nxs.exe"
if ($nxsExpected) {
  if (-not (Test-Path $nxsExe)) {
    throw "Missing bundled nxs executable: $nxsExe"
  }
  & $nxsExe --version *> $null
  if ($LASTEXITCODE -ne 0) {
    throw "Bundled nxs --version failed with exit code $LASTEXITCODE"
  }
}

$logPath = Join-Path ([Environment]::GetFolderPath([System.Environment+SpecialFolder]::UserProfile)) ".nexus/app/logs/shell.log"
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
      ($current.Contains("event=main_window.route_load") -and $current.Contains("path=/launcher")) -and
      ($current.Contains("event=web.ready") -and $current.Contains("location_path=/launcher"))
  } $TimeoutSeconds "launcher web.ready"

  $sidecars = @(Find-SidecarProcess $process.Id $AppDir)
  if ($sidecars.Count -eq 0) {
    throw "Expected bundled nexus-server.exe sidecar process"
  }

  Write-Host "==> Validating window chrome"
  $process.Refresh()
  $mainWindowHandle = [IntPtr]$process.MainWindowHandle
  if ($mainWindowHandle -eq [IntPtr]::Zero) {
    throw "Expected Nexus main window handle"
  }
  $chromeError = [NexusWindowChromeProbe]::ValidateResizeBoundary($mainWindowHandle)
  if (-not [string]::IsNullOrEmpty($chromeError)) {
    throw "Invalid window chrome: $chromeError"
  }
  [void](Find-CaptionButton $mainWindowHandle "最小化")
  [void](Find-CaptionButton $mainWindowHandle "关闭")
  Invoke-CaptionButton $mainWindowHandle "最大化"
  Wait-Until {
    return [NexusWindowChromeProbe]::IsZoomed($mainWindowHandle)
  } 10 "window maximize"
  Invoke-CaptionButton $mainWindowHandle "还原"
  Wait-Until {
    return -not [NexusWindowChromeProbe]::IsZoomed($mainWindowHandle)
  } 10 "window restore"

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
