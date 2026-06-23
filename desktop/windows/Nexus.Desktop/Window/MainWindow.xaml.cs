using System.ComponentModel;
using System.Windows;
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
    private readonly SidecarRuntimeConfig runtime;
    private readonly DesktopStartupTimeline startupTimeline;
    private readonly DesktopUpdateChecker updateChecker;
    private readonly DesktopTrayController trayController;
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
        trayController = new DesktopTrayController(
            startupTimeline,
            RestoreFromTray,
            ReloadFromTray,
            CheckForUpdatesFromTray,
            ExitFromTray);
        webViewHealthProbeTimer = new System.Windows.Threading.DispatcherTimer
        {
            Interval = TimeSpan.FromSeconds(60),
        };
        webViewHealthProbeTimer.Tick += (_, _) => RecoverVisibleWebView("periodic_visible");
        webViewHealthProbeTimer.Start();
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
    }

    private WebViewHost CreateWebViewHost(WebView2 webView)
    {
        return new WebViewHost(webView, runtime, startupTimeline, RecreateWebViewAsync);
    }

    private WebView2 GetOrCreateWebViewControl()
    {
        foreach (UIElement child in WebViewContainer.Children)
        {
            if (child is WebView2 webView)
            {
                return webView;
            }
        }

        WebView2 nextWebView = new();
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
            WebView2 replacement = new();
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
}
