// INPUT: Web 页面投影的 Header 拖动区与交互排除区。
// OUTPUT: WPF 窗口拖动、双击缩放和系统菜单手势。
// POS: 只解释窗口手势，不拥有 DOM 规则或窗口按钮。

using System.Windows;
using System.Windows.Input;
using Microsoft.Web.WebView2.Wpf;
using Nexus.Desktop.WebView;

namespace Nexus.Desktop.Window;

internal sealed class DesktopWindowInteraction
{
    private readonly System.Windows.Window window;
    private DesktopWindowRegionSet regions = DesktopWindowRegionSet.Empty;

    internal DesktopWindowInteraction(System.Windows.Window window)
    {
        this.window = window;
    }

    internal void Attach(WebView2CompositionControl webView)
    {
        webView.PreviewMouseLeftButtonDown += (_, args) =>
            HandleLeftButtonDown(webView, args);
        webView.PreviewMouseRightButtonUp += (_, args) =>
            HandleRightButtonUp(webView, args);
    }

    internal void UpdateRegions(DesktopWindowRegionSet nextRegions)
    {
        regions = nextRegions;
    }

    private void HandleLeftButtonDown(
        WebView2CompositionControl webView,
        MouseButtonEventArgs args)
    {
        if (!CanDragAt(args.GetPosition(webView)))
        {
            return;
        }

        args.Handled = true;
        if (args.ClickCount == 2)
        {
            ToggleWindowState();
            return;
        }
        window.DragMove();
    }

    private void HandleRightButtonUp(
        WebView2CompositionControl webView,
        MouseButtonEventArgs args)
    {
        System.Windows.Point point = args.GetPosition(webView);
        if (!CanDragAt(point))
        {
            return;
        }

        args.Handled = true;
        SystemCommands.ShowSystemMenu(window, webView.PointToScreen(point));
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
}
