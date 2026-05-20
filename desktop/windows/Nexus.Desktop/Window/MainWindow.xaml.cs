using System.Windows;
using Nexus.Desktop.Diagnostics;
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
    private WebViewHost? webViewHost;
    private bool closed;

    internal MainWindow(
        SidecarRuntimeConfig runtime,
        DesktopStartupTimeline startupTimeline,
        DesktopUpdateChecker updateChecker)
    {
        this.runtime = runtime;
        this.startupTimeline = startupTimeline;
        this.updateChecker = updateChecker;
        InitializeComponent();
    }

    protected override void OnClosed(EventArgs e)
    {
        closed = true;
        startupTimeline.Mark("main_window.closed");
        DisposeWebView();
        base.OnClosed(e);

        if (Application.Current?.Dispatcher.HasShutdownStarted == false)
        {
            Application.Current.Shutdown(0);
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
            webViewHost = new WebViewHost(MainWebView, runtime, startupTimeline, updateChecker, this);
            Show();
            Activate();
            await webViewHost.InitializeAsync();
            startupTimeline.Mark("main_window.created");
        }
        else
        {
            Show();
            Activate();
        }
        await webViewHost.LoadRouteAsync(route);
    }

    public void DisposeWebView()
    {
        webViewHost?.Dispose();
        webViewHost = null;
    }
}
