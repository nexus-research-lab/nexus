using System.Windows;
using Nexus.Desktop.Diagnostics;
using Nexus.Desktop.Lifecycle;
using Nexus.Desktop.Runtime;
using Nexus.Desktop.Sidecar;
using Nexus.Desktop.Update;
using Nexus.Desktop.Window;

namespace Nexus.Desktop;

public partial class App : Application
{
    private readonly DesktopStartupTimeline startupTimeline = new();
    private DesktopSingleInstanceCoordinator? singleInstance;
    private SidecarSupervisor? sidecar;
    private DesktopUpdateChecker? updateChecker;
    private MainWindow? mainWindow;
    private DesktopWebRoute? pendingActivationRoute;

    protected override async void OnStartup(StartupEventArgs e)
    {
        base.OnStartup(e);
        startupTimeline.Mark("app.startup");
        singleInstance = new DesktopSingleInstanceCoordinator(startupTimeline);
        if (!singleInstance.IsPrimary)
        {
            await singleInstance.NotifyPrimaryAsync(DesktopProtocolRouter.ActivationMessage(e.Args));
            Shutdown(0);
            return;
        }
        singleInstance.StartServer(HandleActivationAsync);

        try
        {
            sidecar = new SidecarSupervisor(startupTimeline);
            SidecarRuntimeConfig runtime = await sidecar.StartAsync();
            startupTimeline.Mark("sidecar.ready");

            mainWindow = new MainWindow(runtime, startupTimeline);
            MainWindow = mainWindow;
            DesktopWebRoute launchRoute = pendingActivationRoute
                ?? DesktopProtocolRouter.RouteFromActivationMessage(DesktopProtocolRouter.ActivationMessage(e.Args));
            pendingActivationRoute = null;
            await mainWindow.ShowRouteAsync(launchRoute);
            updateChecker = new DesktopUpdateChecker(startupTimeline);
            updateChecker.CheckOnLaunchIfNeeded(mainWindow);
        }
        catch (Exception exception)
        {
            ShowStartupError(exception);
        }
    }

    protected override void OnExit(ExitEventArgs e)
    {
        singleInstance?.Dispose();
        sidecar?.Dispose();
        base.OnExit(e);
    }

    private void ShowStartupError(Exception exception)
    {
        startupTimeline.Mark("startup.failed", new Dictionary<string, string>
        {
            ["error"] = exception.Message,
        });
        string? diagnosticsPath = DesktopDiagnosticsReport.WriteStartupFailure(exception, startupTimeline);
        string message = diagnosticsPath is null
            ? exception.Message
            : $"{exception.Message}{Environment.NewLine}{Environment.NewLine}诊断文件：{diagnosticsPath}";
        MessageBox.Show(
            message,
            "Nexus 启动失败",
            MessageBoxButton.OK,
            MessageBoxImage.Error);
        Shutdown(1);
    }

    private Task HandleActivationAsync(string message)
    {
        DesktopWebRoute route = DesktopProtocolRouter.RouteFromActivationMessage(message);
        Dispatcher.Invoke(() =>
        {
            if (mainWindow is null)
            {
                pendingActivationRoute = route;
                return;
            }
            _ = mainWindow.ShowRouteAsync(route);
        });
        return Task.CompletedTask;
    }
}
