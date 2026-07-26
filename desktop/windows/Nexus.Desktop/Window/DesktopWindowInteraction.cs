// INPUT: Web Header 的 app-region 几何与窗口命中测试消息。
// OUTPUT: Windows 原生 caption 命中结果。
// POS: CompositionControl 与 WindowChrome 之间的非客户区适配层。

using System.Windows;
using System.Windows.Interop;
using System.Windows.Shell;
using Microsoft.Web.WebView2.Wpf;
using Nexus.Desktop.WebView;
using Point = System.Windows.Point;
using Thickness = System.Windows.Thickness;

namespace Nexus.Desktop.Window;

internal sealed class DesktopWindowInteraction : IDisposable
{
    private const int HitTestCaption = 2;
    private const int WindowMessageNonClientHitTest = 0x0084;

    private readonly System.Windows.Window window;
    private DesktopWindowRegionSet regions = DesktopWindowRegionSet.Empty;
    private HwndSource? windowSource;
    private WebView2CompositionControl? webView;

    internal DesktopWindowInteraction(System.Windows.Window window)
    {
        this.window = window;
    }

    internal void AttachWindow(IntPtr windowHandle)
    {
        windowSource = HwndSource.FromHwnd(windowHandle);
        windowSource?.AddHook(HandleWindowMessage);
    }

    internal void SetWebView(WebView2CompositionControl nextWebView)
    {
        webView = nextWebView;
        regions = DesktopWindowRegionSet.Empty;
    }

    internal void ResetWebView()
    {
        webView = null;
        regions = DesktopWindowRegionSet.Empty;
    }

    internal void UpdateRegions(DesktopWindowRegionSet nextRegions)
    {
        regions = nextRegions;
    }

    public void Dispose()
    {
        windowSource?.RemoveHook(HandleWindowMessage);
        windowSource = null;
        ResetWebView();
    }

    private IntPtr HandleWindowMessage(
        IntPtr ignoredWindowHandle,
        int message,
        IntPtr ignoredWordParameter,
        IntPtr longParameter,
        ref bool handled)
    {
        if (message != WindowMessageNonClientHitTest ||
            webView is not { IsVisible: true } currentWebView)
        {
            return IntPtr.Zero;
        }

        Point point = WebViewPoint(currentWebView, longParameter);
        if (!IsInside(currentWebView, point) ||
            IsResizeBoundary(currentWebView, point) ||
            !CanDragAt(point))
        {
            return IntPtr.Zero;
        }

        handled = true;
        return new IntPtr(HitTestCaption);
    }

    private bool CanDragAt(Point point)
    {
        return regions.DragRegions.Any(region => region.Contains(point))
            && !regions.NoDragRegions.Any(region => region.Contains(point));
    }

    private bool IsResizeBoundary(WebView2CompositionControl currentWebView, Point point)
    {
        if (window.WindowState != WindowState.Normal)
        {
            return false;
        }

        Thickness border = WindowChrome.GetWindowChrome(window)?.ResizeBorderThickness
            ?? new Thickness();
        return point.X < border.Left
            || point.X >= currentWebView.ActualWidth - border.Right
            || point.Y >= currentWebView.ActualHeight - border.Bottom;
    }

    private static Point WebViewPoint(
        WebView2CompositionControl currentWebView,
        IntPtr packedScreenPoint)
    {
        long packed = packedScreenPoint.ToInt64();
        int screenX = unchecked((short)(packed & 0xFFFF));
        int screenY = unchecked((short)((packed >> 16) & 0xFFFF));
        return currentWebView.PointFromScreen(new Point(screenX, screenY));
    }

    private static bool IsInside(WebView2CompositionControl currentWebView, Point point)
    {
        return point.X >= 0
            && point.Y >= 0
            && point.X < currentWebView.ActualWidth
            && point.Y < currentWebView.ActualHeight;
    }
}
