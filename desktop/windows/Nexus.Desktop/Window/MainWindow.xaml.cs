using System.ComponentModel;
using System.Windows;
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
    private WebViewHost? webViewHost;
    private bool closed;
    private bool exitRequested;

    internal MainWindow(
        SidecarRuntimeConfig runtime,
        DesktopStartupTimeline startupTimeline,
        DesktopUpdateChecker updateChecker)
    {
        this.runtime = runtime;
        this.startupTimeline = startupTimeline;
        this.updateChecker = updateChecker;
        InitializeComponent();
        trayController = new DesktopTrayController(startupTimeline, RestoreFromTray, CheckForUpdatesFromTray, ExitFromTray);
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
        trayController.Dispose();
        DisposeWebView();
        base.OnClosed(e);

        if (System.Windows.Application.Current?.Dispatcher.HasShutdownStarted == false)
        {
            System.Windows.Application.Current.Shutdown(0);
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
            webViewHost = new WebViewHost(MainWebView, runtime, startupTimeline);
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
}
