// INPUT: Web 页面投影的 Header 拖动区与硬排除区。
// OUTPUT: WPF 窗口拖动与双击缩放手势。
// POS: 只解释窗口手势，不拥有 DOM 规则或窗口按钮。

using System.Runtime.InteropServices;
using System.Windows;
using System.Windows.Input;
using System.Windows.Interop;
using System.Windows.Threading;
using Microsoft.Web.WebView2.Wpf;
using Nexus.Desktop.WebView;

namespace Nexus.Desktop.Window;

internal sealed class DesktopWindowInteraction
{
    private const double DragActivationDistance = 4;
    private const int HitTestCaption = 2;
    private const int VirtualKeyLeftButton = 0x01;
    private const int WindowMessageNonClientLeftButtonDown = 0x00A1;

    private readonly DispatcherTimer dragTimer;
    private readonly System.Windows.Window window;
    private PendingDrag? pendingDrag;
    private DesktopWindowRegionSet regions = DesktopWindowRegionSet.Empty;

    internal DesktopWindowInteraction(System.Windows.Window window)
    {
        this.window = window;
        // WebView2 会接管后续 move；仅在候选按下期间采样物理指针，短按序列原样留给 Web。
        dragTimer = new DispatcherTimer(DispatcherPriority.Input, window.Dispatcher)
        {
            Interval = TimeSpan.FromMilliseconds(8),
        };
        dragTimer.Tick += (_, _) => TrackPendingDrag();
    }

    internal void Attach(WebView2CompositionControl webView)
    {
        webView.AddHandler(
            UIElement.PreviewMouseLeftButtonDownEvent,
            new System.Windows.Input.MouseButtonEventHandler(
                (_, args) => HandleLeftButtonDown(webView, args)),
            handledEventsToo: true);
    }

    internal void UpdateRegions(DesktopWindowRegionSet nextRegions)
    {
        regions = nextRegions;
    }

    private void HandleLeftButtonDown(
        WebView2CompositionControl webView,
        MouseButtonEventArgs args)
    {
        FinishPendingDrag();
        System.Windows.Point point = args.GetPosition(webView);
        if (!CanDragAt(point))
        {
            return;
        }

        if (args.ClickCount == 2)
        {
            args.Handled = true;
            ToggleWindowState();
            return;
        }

        pendingDrag = new PendingDrag(webView, point);
        dragTimer.Start();
    }

    private void TrackPendingDrag()
    {
        if (pendingDrag is not { } drag ||
            !IsLeftButtonPressed() ||
            !GetCursorPos(out NativePoint cursor))
        {
            FinishPendingDrag();
            return;
        }

        System.Windows.Point point = drag.WebView.PointFromScreen(
            new System.Windows.Point(cursor.X, cursor.Y));
        if (DragDistance(drag.Origin, point) < DragActivationDistance)
        {
            return;
        }

        FinishPendingDrag();
        IntPtr windowHandle = new WindowInteropHelper(window).Handle;
        if (windowHandle == IntPtr.Zero)
        {
            return;
        }

        // 越阈值后交还系统非客户区移动循环，避开 WPF 对浏览器按下状态的错误判断。
        _ = ReleaseCapture();
        _ = SendMessage(
            windowHandle,
            WindowMessageNonClientLeftButtonDown,
            new IntPtr(HitTestCaption),
            PackedScreenPoint(cursor));
    }

    private void FinishPendingDrag()
    {
        pendingDrag = null;
        dragTimer.Stop();
    }

    private bool CanDragAt(System.Windows.Point point)
    {
        return regions.DragRegions.Any(region => region.Contains(point))
            && !regions.NoDragRegions.Any(region => region.Contains(point));
    }

    private void ToggleWindowState()
    {
        if (window.WindowState == WindowState.Maximized)
        {
            SystemCommands.RestoreWindow(window);
            return;
        }
        SystemCommands.MaximizeWindow(window);
    }

    private static double DragDistance(
        System.Windows.Point origin,
        System.Windows.Point point)
    {
        double deltaX = point.X - origin.X;
        double deltaY = point.Y - origin.Y;
        return Math.Sqrt((deltaX * deltaX) + (deltaY * deltaY));
    }

    private static bool IsLeftButtonPressed() =>
        (GetAsyncKeyState(VirtualKeyLeftButton) & 0x8000) != 0;

    private static IntPtr PackedScreenPoint(NativePoint point)
    {
        int packed = (point.Y << 16) | (point.X & 0xFFFF);
        return new IntPtr(packed);
    }

    [DllImport("user32.dll")]
    private static extern short GetAsyncKeyState(int virtualKey);

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool GetCursorPos(out NativePoint point);

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool ReleaseCapture();

    [DllImport("user32.dll")]
    private static extern IntPtr SendMessage(
        IntPtr windowHandle,
        int message,
        IntPtr wordParameter,
        IntPtr longParameter);

    [StructLayout(LayoutKind.Sequential)]
    private struct NativePoint
    {
        internal int X;
        internal int Y;
    }

    private sealed record PendingDrag(
        WebView2CompositionControl WebView,
        System.Windows.Point Origin);
}
