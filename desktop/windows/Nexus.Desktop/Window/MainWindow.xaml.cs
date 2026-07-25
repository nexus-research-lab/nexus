using System.ComponentModel;
using System.Runtime.InteropServices;
using System.Windows;
using System.Windows.Interop;
using Microsoft.Web.WebView2.Wpf;
using Nexus.Desktop.Diagnostics;
using Nexus.Desktop.Lifecycle;
using Nexus.Desktop.Runtime;
using Nexus.Desktop.Sidecar;
using Nexus.Desktop.Update;
using Nexus.Desktop.WebView;

namespace Nexus.Desktop.Window;

public partial class MainWindow : System.Windows.Window
{
    private const double PreferredWindowWidth = 1280;
    private const double PreferredWindowHeight = 820;
    private const double PreferredMinimumWindowWidth = 360;
    private const double PreferredMinimumWindowHeight = 520;
    private const double CompactMinimumWindowWidth = 320;
    private const double CompactMinimumWindowHeight = 480;
    private const double ScreenPadding = 48;

    private readonly SidecarRuntimeConfig runtime;
    private readonly DesktopStartupTimeline startupTimeline;
    private readonly DesktopUpdateChecker updateChecker;
    private readonly DesktopTrayController trayController;
    private readonly DesktopWindowInteraction windowInteraction;
    private readonly System.Windows.Threading.DispatcherTimer webViewHealthProbeTimer;
    private WebViewHost? webViewHost;
    private bool closed;
    private bool exitRequested;
    private bool webViewRecreateInFlight;

    internal MainWindow(
        SidecarRuntimeConfig runtime,
        DesktopStartupTimeline startupTimeline,
        DesktopUpdateChecker updateChecker)
    {
        this.runtime = runtime;
        this.startupTimeline = startupTimeline;
        this.updateChecker = updateChecker;
        InitializeComponent();
        windowInteraction = new DesktopWindowInteraction(this);
        ConfigureInitialWindowBounds();
        ConfigureWebViewSurface(MainWebView);
        trayController = new DesktopTrayController(
            startupTimeline,
            RestoreFromTray,
            ReloadFromTray,
            ClearWebCacheFromTray,
            CheckForUpdatesFromTray,
            ExitFromTray);
        webViewHealthProbeTimer = new System.Windows.Threading.DispatcherTimer
        {
            Interval = TimeSpan.FromSeconds(60),
        };
        webViewHealthProbeTimer.Tick += (_, _) => RecoverVisibleWebView("periodic_visible");
        webViewHealthProbeTimer.Start();
    }

    protected override void OnSourceInitialized(EventArgs e)
    {
        base.OnSourceInitialized(e);
        ConfigureNativeWindowBackdrop(new WindowInteropHelper(this).Handle);
    }

    protected override void OnClosing(CancelEventArgs e)
    {
        if (!ShouldCloseForExit())
        {
            e.Cancel = true;
            HideToTray();
            return;
        }

        base.OnClosing(e);
    }

    protected override void OnClosed(EventArgs e)
    {
        closed = true;
        startupTimeline.Mark("main_window.closed");
        webViewHealthProbeTimer.Stop();
        trayController.Dispose();
        DisposeWebView();
        base.OnClosed(e);

        if (System.Windows.Application.Current?.Dispatcher.HasShutdownStarted == false)
        {
            System.Windows.Application.Current.Shutdown(0);
        }
    }

    protected override void OnActivated(EventArgs e)
    {
        base.OnActivated(e);
        _ = webViewHost?.RecoverAfterWindowShownAsync("activated");
    }

    protected override void OnStateChanged(EventArgs e)
    {
        base.OnStateChanged(e);
        if (WindowState != WindowState.Minimized)
        {
            _ = webViewHost?.RecoverAfterWindowShownAsync("state_changed");
        }
    }

    public async Task ShowLauncherAsync()
    {
        await ShowRouteAsync(DesktopWebRoute.Launcher);
    }

    public async Task ShowRouteAsync(DesktopWebRoute route)
    {
        if (closed)
        {
            return;
        }
        if (webViewHost is null)
        {
            startupTimeline.Mark("main_window.create_begin");
            webViewHost = CreateWebViewHost(GetOrCreateWebViewControl());
            ShowMainWindow();
            await webViewHost.InitializeAsync();
            startupTimeline.Mark("main_window.created");
        }
        else
        {
            ShowMainWindow();
        }
        await webViewHost.LoadRouteAsync(route);
    }

    public void DisposeWebView()
    {
        webViewHost?.Dispose();
        webViewHost = null;
        windowInteraction.UpdateRegions(DesktopWindowRegionSet.Empty);
    }

    private WebViewHost CreateWebViewHost(WebView2CompositionControl webView)
    {
        return new WebViewHost(
            webView,
            runtime,
            startupTimeline,
            RecreateWebViewAsync,
            windowInteraction.UpdateRegions);
    }

    private WebView2CompositionControl GetOrCreateWebViewControl()
    {
        foreach (UIElement child in WebViewContainer.Children)
        {
            if (child is WebView2CompositionControl webView)
            {
                return webView;
            }
        }

        WebView2CompositionControl nextWebView = new();
        ConfigureWebViewSurface(nextWebView);
        WebViewContainer.Children.Add(nextWebView);
        return nextWebView;
    }

    private async Task RecreateWebViewAsync(DesktopWebRoute route, string trigger, string reason)
    {
        if (!Dispatcher.CheckAccess())
        {
            await Dispatcher.InvokeAsync(() =>
            {
                _ = RecreateWebViewAsync(route, trigger, reason);
            });
            return;
        }
        if (closed || webViewRecreateInFlight)
        {
            return;
        }

        webViewRecreateInFlight = true;
        try
        {
            startupTimeline.Mark(WebViewRecreateEventName(trigger, "begin"), new Dictionary<string, string>
            {
                ["path"] = route.Path,
                ["reason"] = reason,
                ["trigger"] = trigger,
            });
            await Task.Delay(300);
            if (closed)
            {
                return;
            }

            DisposeWebView();
            WebViewContainer.Children.Clear();
            WebView2CompositionControl replacement = new();
            ConfigureWebViewSurface(replacement);
            WebViewContainer.Children.Add(replacement);
            webViewHost = CreateWebViewHost(replacement);
            await webViewHost.InitializeAsync();
            startupTimeline.Mark(WebViewRecreateEventName(trigger, "ready"), new Dictionary<string, string>
            {
                ["path"] = route.Path,
                ["reason"] = reason,
                ["trigger"] = trigger,
            });
            await webViewHost.LoadRouteAsync(route);
        }
        catch (Exception exception)
        {
            startupTimeline.Mark(WebViewRecreateEventName(trigger, "failed"), new Dictionary<string, string>
            {
                ["error"] = TrimMetadata(exception.Message),
                ["path"] = route.Path,
                ["reason"] = reason,
                ["trigger"] = trigger,
            });
        }
        finally
        {
            webViewRecreateInFlight = false;
        }
    }

    private static string WebViewRecreateEventName(string trigger, string phase)
    {
        if (string.Equals(trigger, "process_failed", StringComparison.OrdinalIgnoreCase))
        {
            return "webview.process_failed_recreate_" + phase;
        }
        string normalizedTrigger = string.IsNullOrWhiteSpace(trigger) ? "manual" : trigger.Trim().Replace('.', '_');
        return "webview." + normalizedTrigger + "_recreate_" + phase;
    }

    private bool ShouldCloseForExit()
    {
        return exitRequested || App.IsExplicitExitRequested;
    }

    private void HideToTray()
    {
        if (closed || !IsVisible)
        {
            return;
        }

        startupTimeline.Mark("main_window.hidden_to_tray");
        Hide();
    }

    private void RestoreFromTray()
    {
        if (!Dispatcher.CheckAccess())
        {
            Dispatcher.Invoke(RestoreFromTray);
            return;
        }
        if (closed)
        {
            return;
        }

        startupTimeline.Mark("main_window.restored_from_tray");
        ShowMainWindow();
        _ = webViewHost?.RecoverAfterWindowShownAsync("tray_restore");
    }

    private void ExitFromTray()
    {
        if (!Dispatcher.CheckAccess())
        {
            Dispatcher.Invoke(ExitFromTray);
            return;
        }

        exitRequested = true;
        App.RequestApplicationExit(0);
    }

    private void CheckForUpdatesFromTray()
    {
        if (!Dispatcher.CheckAccess())
        {
            Dispatcher.Invoke(CheckForUpdatesFromTray);
            return;
        }

        startupTimeline.Mark("tray.update_check_requested");
        _ = updateChecker.CheckNowAsync(this);
    }

    private void ReloadFromTray()
    {
        if (!Dispatcher.CheckAccess())
        {
            Dispatcher.Invoke(ReloadFromTray);
            return;
        }
        if (closed)
        {
            return;
        }

        ShowMainWindow();
        _ = webViewHost?.ReloadAsync("tray_reload");
    }

    private void ClearWebCacheFromTray()
    {
        if (!Dispatcher.CheckAccess())
        {
            Dispatcher.Invoke(ClearWebCacheFromTray);
            return;
        }
        if (closed)
        {
            return;
        }

        ShowMainWindow();
        _ = webViewHost?.ClearCacheAndReloadAsync("tray_clear_cache");
    }

    private void ShowMainWindow()
    {
        Show();
        if (WindowState == WindowState.Minimized)
        {
            WindowState = WindowState.Normal;
        }
        Activate();
        Focus();
    }

    private void RecoverVisibleWebView(string reason)
    {
        if (closed || webViewHost is null || !IsVisible || WindowState == WindowState.Minimized)
        {
            return;
        }
        _ = webViewHost.RecoverAfterWindowShownAsync(reason);
    }

    private static string TrimMetadata(string value)
    {
        string normalized = value.Trim();
        const int maxLength = 240;
        if (normalized.Length <= maxLength)
        {
            return normalized;
        }
        return normalized[..maxLength] + "...";
    }

    private void ConfigureInitialWindowBounds()
    {
        Rect workArea = SystemParameters.WorkArea;
        double width = Math.Min(PreferredWindowWidth, Math.Max(320, workArea.Width - ScreenPadding));
        double height = Math.Min(PreferredWindowHeight, Math.Max(320, workArea.Height - ScreenPadding));
        MinWidth = AdaptiveMinimum(PreferredMinimumWindowWidth, CompactMinimumWindowWidth, width);
        MinHeight = AdaptiveMinimum(PreferredMinimumWindowHeight, CompactMinimumWindowHeight, height);
        Width = width;
        Height = height;
        startupTimeline.Mark("main_window.initial_bounds", new Dictionary<string, string>
        {
            ["height"] = MetadataDimension(height),
            ["min_height"] = MetadataDimension(MinHeight),
            ["min_width"] = MetadataDimension(MinWidth),
            ["width"] = MetadataDimension(width),
            ["work_area_height"] = MetadataDimension(workArea.Height),
            ["work_area_width"] = MetadataDimension(workArea.Width),
        });
    }

    private static double AdaptiveMinimum(double preferred, double compact, double current)
    {
        if (current >= preferred)
        {
            return preferred;
        }
        return Math.Min(current, compact);
    }

    private static string MetadataDimension(double value) => ((int)Math.Round(value)).ToString();

    private void MinimizeWindow(object sender, RoutedEventArgs e) => SystemCommands.MinimizeWindow(this);

    private void MaximizeWindow(object sender, RoutedEventArgs e) => SystemCommands.MaximizeWindow(this);

    private void RestoreWindow(object sender, RoutedEventArgs e) => SystemCommands.RestoreWindow(this);

    private void CloseWindow(object sender, RoutedEventArgs e) => SystemCommands.CloseWindow(this);

    private void ConfigureWebViewSurface(WebView2CompositionControl webView)
    {
        webView.DefaultBackgroundColor = System.Drawing.Color.Transparent;
        windowInteraction.Attach(webView);
    }

    private static void ConfigureNativeWindowBackdrop(IntPtr hwnd)
    {
        if (hwnd == IntPtr.Zero)
        {
            return;
        }

        TrySetDwmAttribute(hwnd, DwmWindowAttribute.SystemBackdropType, DwmSystemBackdropType.MainWindow);
        TrySetDwmAttribute(hwnd, DwmWindowAttribute.WindowCornerPreference, DwmWindowCornerPreference.Round);
        TrySetDwmAttribute(hwnd, DwmWindowAttribute.UseImmersiveDarkMode, 0);
        TrySetDwmAttribute(hwnd, DwmWindowAttribute.CaptionColor, DwmColor(0xF9, 0xF9, 0xF7));
        TrySetDwmAttribute(hwnd, DwmWindowAttribute.BorderColor, DwmColor(0xD8, 0xDD, 0xDA));
        TrySetDwmAttribute(hwnd, DwmWindowAttribute.TextColor, DwmColor(0x2C, 0x21, 0x17));
    }

    private static int DwmColor(byte red, byte green, byte blue) =>
        red | (green << 8) | (blue << 16);

    private static void TrySetDwmAttribute(IntPtr hwnd, int attribute, int value)
    {
        _ = DwmSetWindowAttribute(hwnd, attribute, ref value, sizeof(int));
    }

    [DllImport("dwmapi.dll")]
    private static extern int DwmSetWindowAttribute(IntPtr hwnd, int dwAttribute, ref int pvAttribute, int cbAttribute);

    private static class DwmWindowAttribute
    {
        internal const int UseImmersiveDarkMode = 20;
        internal const int WindowCornerPreference = 33;
        internal const int BorderColor = 34;
        internal const int CaptionColor = 35;
        internal const int TextColor = 36;
        internal const int SystemBackdropType = 38;
    }

    private static class DwmWindowCornerPreference
    {
        internal const int Round = 2;
    }

    private static class DwmSystemBackdropType
    {
        internal const int MainWindow = 2;
    }
}
