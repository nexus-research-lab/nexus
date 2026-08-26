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
using System.Text;
using System.Threading;

public static class NexusWindowChromeProbe
{
    private const long LayeredStyle = 0x00080000;
    private const long NoActivateStyle = 0x08000000;
    private const long ToolWindowStyle = 0x00000080;
    private const long TransparentStyle = 0x00000020;
    private const uint AlphaAttribute = 0x00000002;

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
    private static extern uint GetDpiForWindow(IntPtr hwnd);

    [DllImport("user32.dll")]
    private static extern bool SetCursorPos(int x, int y);

    [DllImport("user32.dll")]
    private static extern bool SetForegroundWindow(IntPtr hwnd);

    [DllImport("user32.dll")]
    private static extern void mouse_event(uint flags, uint dx, uint dy, uint data, UIntPtr extraInfo);

    [DllImport("user32.dll")]
    public static extern bool IsZoomed(IntPtr hwnd);

    [DllImport("user32.dll")]
    public static extern bool IsIconic(IntPtr hwnd);

    [DllImport("user32.dll")]
    private static extern bool ShowWindowAsync(IntPtr hwnd, int command);

    [DllImport("user32.dll")]
    private static extern bool EnumWindows(EnumWindowsCallback callback, IntPtr parameter);

    [DllImport("user32.dll")]
    private static extern uint GetWindowThreadProcessId(IntPtr hwnd, out uint processId);

    [DllImport("user32.dll", CharSet = CharSet.Unicode)]
    private static extern int GetClassName(IntPtr hwnd, StringBuilder className, int maximumCount);

    [DllImport("user32.dll")]
    private static extern bool IsWindowVisible(IntPtr hwnd);

    [DllImport("user32.dll")]
    private static extern bool GetLayeredWindowAttributes(
        IntPtr hwnd,
        out uint colorKey,
        out byte alpha,
        out uint flags);

    [DllImport("user32.dll", EntryPoint = "GetWindowLongW")]
    private static extern int GetWindowLong32(IntPtr hwnd, int index);

    [DllImport("user32.dll", EntryPoint = "GetWindowLongPtrW")]
    private static extern IntPtr GetWindowLongPointer64(IntPtr hwnd, int index);

    private delegate bool EnumWindowsCallback(IntPtr hwnd, IntPtr parameter);

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
                (Name: "top-left", X: bounds.Left + 2, Y: bounds.Top + 2, Hit: 13),
                (Name: "top-right", X: bounds.Right - 2, Y: bounds.Top + 2, Hit: 14),
                (Name: "bottom-left", X: bounds.Left + 2, Y: bounds.Bottom - 2, Hit: 16),
                (Name: "bottom-right", X: bounds.Right - 2, Y: bounds.Bottom - 2, Hit: 17),
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

    public static bool RestoreWindow(IntPtr hwnd)
    {
        return ShowWindowAsync(hwnd, 9);
    }

    public static string ValidateSuspendedCompositionInput(uint browserProcessId)
    {
        string error = string.Empty;
        _ = EnumWindows((hwnd, _) =>
        {
            if (!IsCompositionInputWindow(hwnd, browserProcessId) || !IsWindowVisible(hwnd))
            {
                return true;
            }

            long style = GetWindowLongPointer(hwnd, -20).ToInt64();
            if ((style & TransparentStyle) != 0)
            {
                return true;
            }

            error = $"visible composition window 0x{hwnd.ToInt64():X} still accepts desktop input";
            return false;
        }, IntPtr.Zero);
        return error;
    }

    private static bool IsCompositionInputWindow(IntPtr hwnd, uint browserProcessId)
    {
        _ = GetWindowThreadProcessId(hwnd, out uint processId);
        if (processId != browserProcessId)
        {
            return false;
        }

        var className = new StringBuilder(64);
        if (GetClassName(hwnd, className, className.Capacity) == 0 ||
            !string.Equals(className.ToString(), "Chrome_WidgetWin_1", StringComparison.Ordinal))
        {
            return false;
        }

        long style = GetWindowLongPointer(hwnd, -20).ToInt64();
        long requiredStyles = LayeredStyle | NoActivateStyle | ToolWindowStyle;
        return (style & requiredStyles) == requiredStyles &&
            GetLayeredWindowAttributes(hwnd, out _, out byte alpha, out uint flags) &&
            (flags & AlphaAttribute) != 0 &&
            alpha == 0;
    }

    private static IntPtr GetWindowLongPointer(IntPtr hwnd, int index)
    {
        return IntPtr.Size == 8
            ? GetWindowLongPointer64(hwnd, index)
            : new IntPtr(GetWindowLong32(hwnd, index));
    }

    public static string ValidateDragBehavior(
        IntPtr hwnd,
        double offsetDips,
        string surface,
        bool shouldMove)
    {
        IntPtr previousDpi = SetThreadDpiAwarenessContext(new IntPtr(-4));
        try
        {
            if (!GetWindowRect(hwnd, out Rect before))
            {
                return "cannot read window bounds";
            }

            double scale = Math.Max(1, GetDpiForWindow(hwnd) / 96.0);
            int startX = (before.Left + before.Right) / 2;
            int startY = before.Top + (int)Math.Round(offsetDips * scale);
            int movement = (int)Math.Round(48 * scale);
            int deltaX = before.Left > movement ? -movement : movement;

            _ = SetForegroundWindow(hwnd);
            _ = SetCursorPos(startX, startY);
            mouse_event(0x0002, 0, 0, 0, UIntPtr.Zero);
            try
            {
                Thread.Sleep(100);
                _ = SetCursorPos(startX + deltaX, startY);
                Thread.Sleep(180);
            }
            finally
            {
                mouse_event(0x0004, 0, 0, 0, UIntPtr.Zero);
            }
            Thread.Sleep(250);

            if (!GetWindowRect(hwnd, out Rect after))
            {
                return "cannot read moved window bounds";
            }
            bool moved = Math.Abs(after.Left - before.Left) >= movement / 2;
            if (moved == shouldMove)
            {
                return string.Empty;
            }
            return shouldMove
                ? $"{surface} did not move the window: before={before.Left},{before.Top} after={after.Left},{after.Top}"
                : $"{surface} moved the window unexpectedly: before={before.Left},{before.Top} after={after.Left},{after.Top}";
        }
        finally
        {
            if (previousDpi != IntPtr.Zero)
            {
                _ = SetThreadDpiAwarenessContext(previousDpi);
            }
        }
    }

    public static string ValidateClientAtOffset(IntPtr hwnd, double offsetDips, string surface)
    {
        IntPtr previousDpi = SetThreadDpiAwarenessContext(new IntPtr(-4));
        try
        {
            if (!GetWindowRect(hwnd, out Rect bounds))
            {
                return "cannot read window bounds";
            }

            double scale = Math.Max(1, GetDpiForWindow(hwnd) / 96.0);
            int x = (bounds.Left + bounds.Right) / 2;
            int y = bounds.Top + (int)Math.Round(offsetDips * scale);
            long packedPoint = ((long)(ushort)y << 16) | (ushort)x;
            long actualHit = SendMessage(
                hwnd,
                0x0084,
                IntPtr.Zero,
                new IntPtr(packedPoint)
            ).ToInt64();
            return actualHit == 1
                ? string.Empty
                : $"{surface} returned hit code {actualHit}, expected client code 1";
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

function New-AutomationNameCondition([string]$Name) {
  return [System.Windows.Automation.PropertyCondition]::new(
    [System.Windows.Automation.AutomationElement]::NameProperty,
    $Name
  )
}

function Find-VisibleControl(
  [System.Windows.Automation.AutomationElement]$Root,
  [System.Windows.Automation.Condition]$Condition,
  [System.Windows.Automation.ControlType]$ControlType,
  [string]$Description
) {
  $control = $Root.FindFirst([System.Windows.Automation.TreeScope]::Descendants, $Condition)
  if ($null -eq $control -or $control.Current.ControlType -ne $ControlType) {
    throw "Missing $Description"
  }
  if ($control.Current.IsOffscreen -or $control.Current.BoundingRectangle.IsEmpty) {
    throw "$Description is not visible"
  }
  return $control
}

function Find-CaptionButton([IntPtr]$Hwnd, [string]$Name) {
  $root = [System.Windows.Automation.AutomationElement]::FromHandle($Hwnd)
  return Find-VisibleControl `
    $root `
    (New-AutomationNameCondition $Name) `
    ([System.Windows.Automation.ControlType]::Button) `
    "caption button: $Name"
}

function Invoke-CaptionButton([IntPtr]$Hwnd, [string]$Name) {
  $button = Find-CaptionButton $Hwnd $Name
  $pattern = $button.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern)
  $pattern.Invoke()
}

function Find-MenuItem([IntPtr]$Hwnd, [string]$Name) {
  $root = [System.Windows.Automation.AutomationElement]::FromHandle($Hwnd)
  return Find-VisibleControl `
    $root `
    (New-AutomationNameCondition $Name) `
    ([System.Windows.Automation.ControlType]::MenuItem) `
    "native menu item: $Name"
}

function Find-ApplicationMenuItem([int]$ProcessId, [string]$Name) {
  $condition = [System.Windows.Automation.AndCondition]::new(
    [System.Windows.Automation.PropertyCondition]::new(
      [System.Windows.Automation.AutomationElement]::ProcessIdProperty,
      $ProcessId
    ),
    (New-AutomationNameCondition $Name)
  )
  return Find-VisibleControl `
    ([System.Windows.Automation.AutomationElement]::RootElement) `
    $condition `
    ([System.Windows.Automation.ControlType]::MenuItem) `
    "native popup menu item: $Name"
}

function Find-NexusDialog([int]$ProcessId) {
  $condition = [System.Windows.Automation.AndCondition]::new(
    [System.Windows.Automation.PropertyCondition]::new(
      [System.Windows.Automation.AutomationElement]::ProcessIdProperty,
      $ProcessId
    ),
    [System.Windows.Automation.PropertyCondition]::new(
      [System.Windows.Automation.AutomationElement]::AutomationIdProperty,
      "NexusDialogWindow"
    )
  )
  return Find-VisibleControl `
    ([System.Windows.Automation.AutomationElement]::RootElement) `
    $condition `
    ([System.Windows.Automation.ControlType]::Window) `
    "Nexus dialog"
}

function Test-FileMenuPopup([IntPtr]$Hwnd, [int]$ProcessId) {
  $fileMenu = Find-MenuItem $Hwnd "文件"
  $expand = $fileMenu.GetCurrentPattern([System.Windows.Automation.ExpandCollapsePattern]::Pattern)
  $expand.Expand()
  try {
    Start-Sleep -Milliseconds 350
    [void](Find-ApplicationMenuItem $ProcessId "关闭窗口")
    [void](Find-ApplicationMenuItem $ProcessId "退出 Nexus")
  } finally {
    $expand.Collapse()
    Start-Sleep -Milliseconds 250
  }
}

function Test-AboutDialog([IntPtr]$Hwnd, [int]$ProcessId) {
  $helpMenu = Find-MenuItem $Hwnd "帮助"
  $expand = $helpMenu.GetCurrentPattern([System.Windows.Automation.ExpandCollapsePattern]::Pattern)
  $expand.Expand()
  try {
    Start-Sleep -Milliseconds 350
    $aboutItem = Find-ApplicationMenuItem $ProcessId "关于 Nexus"
    $aboutItem.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern).Invoke()
    Wait-Until {
      try {
        [void](Find-NexusDialog $ProcessId)
        return $true
      } catch {
        return $false
      }
    } 5 "Nexus about dialog"
    $dialog = Find-NexusDialog $ProcessId
    $closeButton = Find-VisibleControl `
      $dialog `
      (New-AutomationNameCondition "知道了") `
      ([System.Windows.Automation.ControlType]::Button) `
      "Nexus dialog action: 知道了"
    $closeButton.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern).Invoke()
  } finally {
    $expand.Collapse()
    Start-Sleep -Milliseconds 250
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

$nexusctlExe = Join-Path $AppDir "Resources/bin/nexusctl.exe"
if (-not (Test-Path $nexusctlExe)) {
  throw "Missing bundled nexusctl executable: $nexusctlExe"
}

& $nexusctlExe --help *> $null
if ($LASTEXITCODE -ne 0) {
  throw "Bundled nexusctl --help failed with exit code $LASTEXITCODE"
}

$nexuscfgExe = Join-Path $AppDir "Resources/bin/nexuscfg.exe"
if (-not (Test-Path $nexuscfgExe)) {
  throw "Missing bundled nexuscfg executable: $nexuscfgExe"
}

& $nexuscfgExe --help *> $null
if ($LASTEXITCODE -ne 0) {
  throw "Bundled nexuscfg --help failed with exit code $LASTEXITCODE"
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
  Start-Sleep -Milliseconds 500
  $currentLog = Read-Log $logPath
  $currentLog = $currentLog.Substring(
    $currentLog.LastIndexOf("[$marker] smoke_start", [System.StringComparison]::Ordinal)
  )
  $browserProcessMatches = [regex]::Matches(
    $currentLog,
    'event=webview\.initialize_ready[^\r\n]*browser_process_id=(\d+)'
  )
  if ($browserProcessMatches.Count -eq 0) {
    throw "Missing initialized WebView2 browser process event"
  }
  $browserProcessId = [uint]$browserProcessMatches[$browserProcessMatches.Count - 1].Groups[1].Value
  $dragError = [NexusWindowChromeProbe]::ValidateDragBehavior(
    $mainWindowHandle,
    17,
    "native title bar",
    $true
  )
  if (-not [string]::IsNullOrEmpty($dragError)) {
    throw "Invalid window drag region: $dragError"
  }
  $clientError = [NexusWindowChromeProbe]::ValidateClientAtOffset(
    $mainWindowHandle,
    64,
    "Web content"
  )
  if (-not [string]::IsNullOrEmpty($clientError)) {
    throw "Invalid Web client region: $clientError"
  }
  $webDragError = [NexusWindowChromeProbe]::ValidateDragBehavior(
    $mainWindowHandle,
    64,
    "Web content",
    $false
  )
  if (-not [string]::IsNullOrEmpty($webDragError)) {
    throw "Invalid Web drag behavior: $webDragError"
  }
  $chromeError = [NexusWindowChromeProbe]::ValidateResizeBoundary($mainWindowHandle)
  if (-not [string]::IsNullOrEmpty($chromeError)) {
    throw "Invalid window chrome: $chromeError"
  }
  [void](Find-CaptionButton $mainWindowHandle "最小化")
  [void](Find-CaptionButton $mainWindowHandle "关闭")
  [void](Find-CaptionButton $mainWindowHandle "后退")
  [void](Find-CaptionButton $mainWindowHandle "前进")
  foreach ($menuName in @("文件", "编辑", "视图", "帮助")) {
    [void](Find-MenuItem $mainWindowHandle $menuName)
  }
  Invoke-CaptionButton $mainWindowHandle "最大化"
  Wait-Until {
    return [NexusWindowChromeProbe]::IsZoomed($mainWindowHandle)
  } 10 "window maximize"
  Invoke-CaptionButton $mainWindowHandle "还原"
  Wait-Until {
    return -not [NexusWindowChromeProbe]::IsZoomed($mainWindowHandle)
  } 10 "window restore"
  Test-FileMenuPopup $mainWindowHandle $process.Id
  Test-AboutDialog $mainWindowHandle $process.Id

  Write-Host "==> Validating minimized WebView input"
  Invoke-CaptionButton $mainWindowHandle "最小化"
  Wait-Until {
    return [NexusWindowChromeProbe]::IsIconic($mainWindowHandle)
  } 10 "window minimize"
  $minimizedInputError = [NexusWindowChromeProbe]::ValidateSuspendedCompositionInput($browserProcessId)
  if (-not [string]::IsNullOrEmpty($minimizedInputError)) {
    throw "Minimized WebView2 input still covers the desktop: $minimizedInputError"
  }
  if (-not [NexusWindowChromeProbe]::RestoreWindow($mainWindowHandle)) {
    throw "Could not restore Nexus after the minimized input check"
  }
  Wait-Until {
    $log = Read-Log $logPath
    $markerIndex = $log.LastIndexOf("[$marker] smoke_start", [System.StringComparison]::Ordinal)
    if ($markerIndex -lt 0) {
      return $false
    }
    $current = $log.Substring($markerIndex)
    return -not [NexusWindowChromeProbe]::IsIconic($mainWindowHandle) -and
      $current.Contains("reason=window_restored state=interactive")
  } 10 "window restore input activation"
  Write-Host "==> Closing app to tray"
  [void]$process.CloseMainWindow()
  Wait-Until {
    $log = Read-Log $logPath
    $markerIndex = $log.LastIndexOf("[$marker] smoke_start", [System.StringComparison]::Ordinal)
    if ($markerIndex -lt 0) {
      return $false
    }
    $current = $log.Substring($markerIndex)
    return $current.Contains("event=main_window.hidden_to_tray") -and
      $current.Contains("event=webview.visibility_changed") -and
      $current.Contains("reason=tray_hide state=suspended")
  } 10 "window hidden to tray"

  $process.Refresh()
  if ($process.HasExited) {
    throw "Expected window close to keep Nexus running in the tray"
  }
  $suspendedInputError = [NexusWindowChromeProbe]::ValidateSuspendedCompositionInput($browserProcessId)
  if (-not [string]::IsNullOrEmpty($suspendedInputError)) {
    throw "WebView2 input still covers the desktop: $suspendedInputError"
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

  Wait-Until {
    $browser = Get-Process -Id $browserProcessId -ErrorAction SilentlyContinue
    return $null -eq $browser -or $browser.ProcessName -ne "msedgewebview2"
  } $TimeoutSeconds "WebView2 browser process cleanup"
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
