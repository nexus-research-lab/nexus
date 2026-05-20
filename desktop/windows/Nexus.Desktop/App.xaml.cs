using System.Windows;
using Nexus.Desktop.Diagnostics;
using Nexus.Desktop.Lifecycle;
using Nexus.Desktop.Runtime;
using Nexus.Desktop.Sidecar;
using Nexus.Desktop.Update;
using Nexus.Desktop.Window;

namespace Nexus.Desktop;

public partial class App : System.Windows.Application
{
    private readonly DesktopStartupTimeline startupTimeline = new();
    private DesktopSingleInstanceCoordinator? singleInstance;
    private SidecarSupervisor? sidecar;
    private DesktopUpdateChecker? updateChecker;
    private MainWindow? mainWindow;
    private DesktopWebRoute? pendingActivationRoute;
    private bool exitRequested;

    internal static bool IsExplicitExitRequested => Current is App app && app.exitRequested;

    internal static void RequestApplicationExit(int exitCode)
    {
        if (Current is App app)
        {
            app.exitRequested = true;
            app.Shutdown(exitCode);
            return;
        }

        Current?.Shutdown(exitCode);
    }

    protected override async void OnStartup(StartupEventArgs e)
    {
        base.OnStartup(e);
        startupTimeline.Mark("app.startup");
        singleInstance = new DesktopSingleInstanceCoordinator(startupTimeline);
        string activationMessage = DesktopProtocolRouter.ActivationMessage(e.Args);
        if (!singleInstance.IsPrimary)
        {
            await singleInstance.NotifyPrimaryAsync(activationMessage);
            RequestApplicationExit(0);
            return;
        }
        if (DesktopProtocolRouter.IsExitActivationMessage(activationMessage))
        {
            RequestApplicationExit(0);
            return;
        }
        singleInstance.StartServer(HandleActivationAsync);

        try
        {
            sidecar = new SidecarSupervisor(startupTimeline);
            SidecarRuntimeConfig runtime = await sidecar.StartAsync();
            startupTimeline.Mark("sidecar.ready");

            updateChecker = new DesktopUpdateChecker(startupTimeline);
            mainWindow = new MainWindow(runtime, startupTimeline, updateChecker);
            MainWindow = mainWindow;
            DesktopWebRoute launchRoute = pendingActivationRoute
                ?? DesktopProtocolRouter.RouteFromActivationMessage(activationMessage);
            pendingActivationRoute = null;
            await mainWindow.ShowRouteAsync(launchRoute);
            updateChecker.CheckOnLaunchIfNeeded(mainWindow);
        }
        catch (Exception exception)
        {
            ShowStartupError(exception);
        }
    }

    protected override void OnExit(ExitEventArgs e)
    {
        mainWindow?.DisposeWebView();
        singleInstance?.Dispose();
        sidecar?.Dispose();
        base.OnExit(e);
    }

    protected override void OnSessionEnding(System.Windows.SessionEndingCancelEventArgs e)
    {
        exitRequested = true;
        base.OnSessionEnding(e);
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
        System.Windows.MessageBox.Show(
            message,
            "Nexus 启动失败",
            MessageBoxButton.OK,
            MessageBoxImage.Error);
        RequestApplicationExit(1);
    }

    private Task HandleActivationAsync(string message)
    {
        if (DesktopProtocolRouter.IsExitActivationMessage(message))
        {
            Dispatcher.Invoke(() => RequestApplicationExit(0));
            return Task.CompletedTask;
        }

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
