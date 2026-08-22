using System.Runtime.InteropServices;
using System.Text;

namespace Nexus.Desktop.WebView;

internal readonly record struct WebViewCompositionWindowGuardResult(
    int TargetWindowCount,
    int OwnedWindowCount,
    int PassThroughWindowCount,
    int OwnerChangeCount,
    int StyleChangeCount);

// CompositionControl 的输入面是浏览器进程里的顶层 HWND；过滤条件必须同时锁定 PID、窗口类和透明分层样式，不能碰同机上的其他 Chromium 窗口。
internal sealed class WebViewCompositionWindowGuard
{
    private const int ExtendedStyleIndex = -20;
    private const int OwnerIndex = -8;
    private const long LayeredStyle = 0x00080000;
    private const long NoActivateStyle = 0x08000000;
    private const long ToolWindowStyle = 0x00000080;
    private const long TransparentStyle = 0x00000020;
    private const uint AlphaAttribute = 0x00000002;
    private const uint NoMove = 0x0002;
    private const uint NoSize = 0x0001;
    private const uint NoZOrder = 0x0004;
    private const uint NoActivate = 0x0010;
    private const uint FrameChanged = 0x0020;
    private const uint GetOwner = 4;
    private const string CompositionWindowClass = "Chrome_WidgetWin_1";

    private readonly uint browserProcessId;
    private readonly IntPtr ownerWindow;
    private readonly HashSet<IntPtr> passThroughWindows = [];

    public WebViewCompositionWindowGuard(uint browserProcessId, IntPtr ownerWindow)
    {
        this.browserProcessId = browserProcessId;
        this.ownerWindow = ownerWindow;
    }

    public uint BrowserProcessId => browserProcessId;

    public WebViewCompositionWindowGuardResult Synchronize(bool interactive)
    {
        // owner 约束最小化与虚拟桌面归属；鼠标穿透补上 Window.Hide 不会自动隐藏 owned window 的缺口。
        int styleChanges = interactive ? RestoreTrackedStyles() : 0;
        int targetWindows = 0;
        int ownedWindows = 0;
        int passThroughWindowCount = 0;
        int ownerChanges = 0;

        _ = EnumWindows((window, _) =>
        {
            if (!IsCompositionInputWindow(window))
            {
                return true;
            }

            targetWindows++;
            bool wasOwned = ownerWindow != IntPtr.Zero && GetWindow(window, GetOwner) == ownerWindow;
            if (wasOwned || TryAssignOwner(window))
            {
                ownedWindows++;
                if (!wasOwned)
                {
                    ownerChanges++;
                }
            }

            if (!interactive && TryEnablePassThrough(window, out bool changed))
            {
                passThroughWindowCount++;
                if (changed)
                {
                    styleChanges++;
                }
            }
            return true;
        }, IntPtr.Zero);

        return new WebViewCompositionWindowGuardResult(
            targetWindows,
            ownedWindows,
            passThroughWindowCount,
            ownerChanges,
            styleChanges);
    }

    private int RestoreTrackedStyles()
    {
        int changes = 0;
        foreach (IntPtr window in passThroughWindows.ToArray())
        {
            if (!IsWindow(window) || !BelongsToBrowserProcess(window))
            {
                passThroughWindows.Remove(window);
                continue;
            }

            long style = GetWindowLongPointer(window, ExtendedStyleIndex).ToInt64();
            if ((style & TransparentStyle) == 0)
            {
                passThroughWindows.Remove(window);
                continue;
            }
            if (SetPassThrough(window, enabled: false))
            {
                changes++;
                passThroughWindows.Remove(window);
            }
        }
        return changes;
    }

    private bool TryEnablePassThrough(IntPtr window, out bool changed)
    {
        changed = false;
        long style = GetWindowLongPointer(window, ExtendedStyleIndex).ToInt64();
        if ((style & TransparentStyle) != 0)
        {
            return true;
        }
        if (!SetPassThrough(window, enabled: true))
        {
            return false;
        }

        passThroughWindows.Add(window);
        changed = true;
        return true;
    }

    private bool SetPassThrough(IntPtr window, bool enabled)
    {
        long currentStyle = GetWindowLongPointer(window, ExtendedStyleIndex).ToInt64();
        long nextStyle = enabled
            ? currentStyle | TransparentStyle
            : currentStyle & ~TransparentStyle;
        if (nextStyle == currentStyle)
        {
            return true;
        }

        _ = SetWindowLongPointer(window, ExtendedStyleIndex, new IntPtr(nextStyle));
        _ = SetWindowPos(
            window,
            IntPtr.Zero,
            0,
            0,
            0,
            0,
            NoMove | NoSize | NoZOrder | NoActivate | FrameChanged);
        long appliedStyle = GetWindowLongPointer(window, ExtendedStyleIndex).ToInt64();
        return ((appliedStyle & TransparentStyle) != 0) == enabled;
    }

    private bool TryAssignOwner(IntPtr window)
    {
        if (ownerWindow == IntPtr.Zero || !IsWindow(ownerWindow) || GetWindow(window, GetOwner) != IntPtr.Zero)
        {
            return false;
        }

        _ = SetWindowLongPointer(window, OwnerIndex, ownerWindow);
        return GetWindow(window, GetOwner) == ownerWindow;
    }

    private bool IsCompositionInputWindow(IntPtr window)
    {
        if (!BelongsToBrowserProcess(window))
        {
            return false;
        }

        var className = new StringBuilder(64);
        if (GetClassName(window, className, className.Capacity) == 0 ||
            !string.Equals(className.ToString(), CompositionWindowClass, StringComparison.Ordinal))
        {
            return false;
        }

        long style = GetWindowLongPointer(window, ExtendedStyleIndex).ToInt64();
        long requiredStyles = LayeredStyle | NoActivateStyle | ToolWindowStyle;
        if ((style & requiredStyles) != requiredStyles)
        {
            return false;
        }

        return GetLayeredWindowAttributes(window, out _, out byte alpha, out uint flags) &&
            (flags & AlphaAttribute) != 0 &&
            alpha == 0;
    }

    private bool BelongsToBrowserProcess(IntPtr window)
    {
        _ = GetWindowThreadProcessId(window, out uint processId);
        return processId == browserProcessId;
    }

    private static IntPtr GetWindowLongPointer(IntPtr window, int index)
    {
        return IntPtr.Size == 8
            ? GetWindowLongPointer64(window, index)
            : new IntPtr(GetWindowLong32(window, index));
    }

    private static IntPtr SetWindowLongPointer(IntPtr window, int index, IntPtr value)
    {
        return IntPtr.Size == 8
            ? SetWindowLongPointer64(window, index, value)
            : new IntPtr(SetWindowLong32(window, index, value.ToInt32()));
    }

    private delegate bool EnumWindowsCallback(IntPtr window, IntPtr parameter);

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool EnumWindows(EnumWindowsCallback callback, IntPtr parameter);

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool IsWindow(IntPtr window);

    [DllImport("user32.dll")]
    private static extern uint GetWindowThreadProcessId(IntPtr window, out uint processId);

    [DllImport("user32.dll", CharSet = CharSet.Unicode)]
    private static extern int GetClassName(IntPtr window, StringBuilder className, int maximumCount);

    [DllImport("user32.dll")]
    private static extern IntPtr GetWindow(IntPtr window, uint command);

    [DllImport("user32.dll", SetLastError = true, EntryPoint = "GetWindowLongW")]
    private static extern int GetWindowLong32(IntPtr window, int index);

    [DllImport("user32.dll", SetLastError = true, EntryPoint = "GetWindowLongPtrW")]
    private static extern IntPtr GetWindowLongPointer64(IntPtr window, int index);

    [DllImport("user32.dll", SetLastError = true, EntryPoint = "SetWindowLongW")]
    private static extern int SetWindowLong32(IntPtr window, int index, int value);

    [DllImport("user32.dll", SetLastError = true, EntryPoint = "SetWindowLongPtrW")]
    private static extern IntPtr SetWindowLongPointer64(IntPtr window, int index, IntPtr value);

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool GetLayeredWindowAttributes(
        IntPtr window,
        out uint colorKey,
        out byte alpha,
        out uint flags);

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool SetWindowPos(
        IntPtr window,
        IntPtr insertAfter,
        int x,
        int y,
        int width,
        int height,
        uint flags);
}
