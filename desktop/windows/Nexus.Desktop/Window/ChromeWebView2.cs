using System.Runtime.InteropServices;
using System.Windows;
using System.Windows.Media;
using System.Windows.Shell;
using System.Windows.Threading;
using Microsoft.Web.WebView2.Wpf;

namespace Nexus.Desktop.Window;

internal sealed class ChromeWebView2 : WebView2
{
    private const int RegionDifference = 4;
    private const int RegionError = 0;

    protected override HandleRef BuildWindowCore(HandleRef hwndParent)
    {
        HandleRef handle = base.BuildWindowCore(hwndParent);
        _ = Dispatcher.BeginInvoke(ApplyChromeRegion, DispatcherPriority.Loaded);
        return handle;
    }

    protected override void OnWindowPositionChanged(Rect rcBoundingBox)
    {
        base.OnWindowPositionChanged(rcBoundingBox);
        ApplyChromeRegion();
    }

    private void ApplyChromeRegion()
    {
        if (Handle == IntPtr.Zero || !GetClientRect(Handle, out NativeRect bounds))
        {
            return;
        }

        System.Windows.Window? window = System.Windows.Window.GetWindow(this);
        Thickness border = ResizeBorder(window);
        DpiScale dpi = VisualTreeHelper.GetDpi(this);
        int left = DevicePixels(border.Left, dpi.DpiScaleX);
        int top = DevicePixels(border.Top, dpi.DpiScaleY);
        int right = bounds.Right - DevicePixels(border.Right, dpi.DpiScaleX);
        int bottom = bounds.Bottom - DevicePixels(border.Bottom, dpi.DpiScaleY);
        if (right <= left || bottom <= top)
        {
            return;
        }

        int controlsWidth = DevicePixels(SystemParameters.WindowCaptionButtonWidth * 3, dpi.DpiScaleX);
        int controlsHeight = DevicePixels(SystemParameters.WindowCaptionButtonHeight, dpi.DpiScaleY);
        int controlsLeft = Math.Max(left, right - controlsWidth);
        int controlsBottom = Math.Min(bottom, controlsHeight);
        ApplyRegion(left, top, right, bottom, controlsLeft, controlsBottom);
    }

    private static Thickness ResizeBorder(System.Windows.Window? window)
    {
        if (window?.WindowState == WindowState.Maximized)
        {
            return new Thickness();
        }

        return window is null
            ? new Thickness()
            : WindowChrome.GetWindowChrome(window)?.ResizeBorderThickness ?? new Thickness();
    }

    private void ApplyRegion(
        int left,
        int top,
        int right,
        int bottom,
        int controlsLeft,
        int controlsBottom)
    {
        IntPtr visibleRegion = CreateRectRgn(left, top, right, bottom);
        IntPtr controlsRegion = CreateRectRgn(controlsLeft, 0, right, controlsBottom);
        if (visibleRegion == IntPtr.Zero || controlsRegion == IntPtr.Zero)
        {
            DeleteRegion(visibleRegion);
            DeleteRegion(controlsRegion);
            return;
        }

        try
        {
            // WebView2 是独立 HWND；只有从它的原生区域扣除宿主 chrome，WPF 按钮和缩放边界才能收到输入。
            if (CombineRgn(visibleRegion, visibleRegion, controlsRegion, RegionDifference) == RegionError)
            {
                return;
            }
            if (SetWindowRgn(Handle, visibleRegion, true) != 0)
            {
                visibleRegion = IntPtr.Zero;
            }
        }
        finally
        {
            DeleteRegion(visibleRegion);
            DeleteRegion(controlsRegion);
        }
    }

    private static int DevicePixels(double value, double scale) =>
        Math.Max(0, (int)Math.Ceiling(value * scale));

    private static void DeleteRegion(IntPtr region)
    {
        if (region != IntPtr.Zero)
        {
            _ = DeleteObject(region);
        }
    }

    [StructLayout(LayoutKind.Sequential)]
    private struct NativeRect
    {
        internal int Left;
        internal int Top;
        internal int Right;
        internal int Bottom;
    }

    [DllImport("user32.dll")]
    private static extern bool GetClientRect(IntPtr hwnd, out NativeRect rect);

    [DllImport("gdi32.dll")]
    private static extern IntPtr CreateRectRgn(int left, int top, int right, int bottom);

    [DllImport("gdi32.dll")]
    private static extern int CombineRgn(
        IntPtr destination,
        IntPtr source1,
        IntPtr source2,
        int combineMode);

    [DllImport("user32.dll")]
    private static extern int SetWindowRgn(IntPtr hwnd, IntPtr region, bool redraw);

    [DllImport("gdi32.dll")]
    private static extern bool DeleteObject(IntPtr value);
}
