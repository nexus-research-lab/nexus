using System.Windows;
using Nexus.Desktop.Diagnostics;
using Nexus.Desktop.Runtime;
using Nexus.Desktop.Sidecar;
using Nexus.Desktop.WebView;

namespace Nexus.Desktop.Window;

public partial class MainWindow : System.Windows.Window
{
    private readonly SidecarRuntimeConfig runtime;
    private readonly DesktopStartupTimeline startupTimeline;
    private WebViewHost? webViewHost;

    public MainWindow(SidecarRuntimeConfig runtime, DesktopStartupTimeline startupTimeline)
    {
        this.runtime = runtime;
        this.startupTimeline = startupTimeline;
        InitializeComponent();
    }

    public async Task ShowLauncherAsync()
    {
        await ShowRouteAsync(DesktopWebRoute.Launcher);
    }

    public async Task ShowRouteAsync(DesktopWebRoute route)
    {
        if (webViewHost is null)
        {
            startupTimeline.Mark("main_window.create_begin");
            webViewHost = new WebViewHost(MainWebView, runtime, startupTimeline);
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
}
