// INPUT: Windows application lifecycle events, sidecar startup, and state-root recovery.
// OUTPUT: A running Nexus shell or a safe startup failure with internal diagnostics preserved.
// POS: Windows application lifecycle boundary; raw startup causes never enter user-facing dialogs.

using System.Diagnostics;
using System.IO;
using System.Windows;
using Nexus.Desktop.Dialog;
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
    private bool exitCleanupComplete;
    private readonly CancellationTokenSource startupCancellation = new();
    private Task<SidecarRuntimeConfig>? sidecarStartup;

    internal static bool IsExitCleanupComplete => Current is App app && app.exitCleanupComplete;

    internal static bool IsExplicitExitRequested => Current is App app && app.exitRequested;

    internal static void RequestApplicationExit(int exitCode)
    {
        if (Current is App app)
        {
            if (app.exitRequested) return;
            app.exitRequested = true;
            app.startupCancellation.Cancel();
            // Leave OnClosing before the eventual Shutdown closes the window again.
            app.Dispatcher.BeginInvoke(new Action(async () => await app.ExitAsync(exitCode)));
            return;
        }

        Current?.Shutdown(exitCode);
    }

    private async Task ExitAsync(int exitCode)
    {
        startupTimeline.Mark("app.exit_begin");
        try
        {
            if (sidecarStartup is not null)
            {
                try { await sidecarStartup; }
                catch (Exception) { /* Startup failure/cancellation still requires owned-process cleanup. */ }
            }
            if (sidecar is not null) await sidecar.StopAsync();
            if (singleInstance is not null) await singleInstance.StopServerAsync();
        }
        catch (Exception exception)
        {
            startupTimeline.Mark("app.exit_cleanup_failed", new Dictionary<string, string> { ["error"] = exception.Message });
            DesktopDiagnosticsReport.WriteRuntimeIssue("shutdown-failure", exception.Message, null, startupTimeline);
            exitCode = 1;
        }
        finally
        {
            exitCleanupComplete = true;
            Shutdown(exitCode);
        }
    }

    protected override async void OnStartup(StartupEventArgs e)
    {
        if (DesktopStateRootMigration.TryRunHelper(e.Args))
        {
            Shutdown(0);
            return;
        }
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
            sidecarStartup = Task.Run(async () =>
            {
                sidecar = new SidecarSupervisor(startupTimeline, startupCancellation.Token);
                return await sidecar.StartAsync(startupCancellation.Token);
            });
            SidecarRuntimeConfig runtime = await sidecarStartup;
            if (exitRequested || Dispatcher.HasShutdownStarted)
            {
                return;
            }
            startupTimeline.Mark("sidecar.ready");
            CompletePendingStateRootMigration();

            updateChecker = new DesktopUpdateChecker(startupTimeline);
            await updateChecker.ClearStaleUpdateCacheIfNeededAsync();
            if (exitRequested) return;
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
            if (exitRequested || Dispatcher.HasShutdownStarted)
            {
                return;
            }
            if (RollbackPendingStateRootMigration(exception))
            {
                return;
            }
            ShowStartupError(exception);
        }
    }

    protected override void OnExit(ExitEventArgs e)
    {
        try
        {
            mainWindow?.DisposeWebView();
        }
        finally
        {
            sidecar?.Dispose(); // Nonblocking Job close also covers OS session shutdown.
            singleInstance?.Dispose(); // Release primary ownership only after backend cleanup.
            base.OnExit(e);
        }
    }

    protected override void OnSessionEnding(System.Windows.SessionEndingCancelEventArgs e)
    {
        exitRequested = true;
        exitCleanupComplete = true;
        startupCancellation.Cancel();
        sidecar?.Dispose();
        base.OnSessionEnding(e);
    }

    private void ShowStartupError(Exception exception)
    {
        startupTimeline.Mark("startup.failed", new Dictionary<string, string>
        {
            ["error"] = exception.Message,
        });
        string? diagnosticsPath = DesktopDiagnosticsReport.WriteStartupFailure(exception, startupTimeline);
        NexusDialogWindow.ShowMessage(
            null,
            DesktopFailureCopy.StartupTitle,
            DesktopFailureCopy.StartupMessage(diagnosticsPath is not null),
            "退出");
        RequestApplicationExit(1);
    }

    private void CompletePendingStateRootMigration()
    {
        try
        {
            string? previousRoot = DesktopStateRootStore.CompleteMigration();
            if (string.IsNullOrWhiteSpace(previousRoot))
            {
                return;
            }
            startupTimeline.Mark("state_root.migration_committed", new Dictionary<string, string>
            {
                ["previous_root"] = previousRoot,
                ["state_root"] = DesktopPaths.RootDirectory,
            });
            _ = Task.Run(() =>
            {
                try
                {
                    Directory.Delete(previousRoot, recursive: true);
                }
                catch (Exception exception) when (exception is IOException or UnauthorizedAccessException)
                {
                    Trace.WriteLine($"[Nexus State Root] old root cleanup failed: {exception.Message}");
                }
            });
        }
        catch (Exception exception)
        {
            startupTimeline.Mark("state_root.migration_commit_failed", new Dictionary<string, string>
            {
                ["error"] = exception.Message,
            });
        }
    }

    private bool RollbackPendingStateRootMigration(Exception startupError)
    {
        string? previousRoot = DesktopStateRootStore.PreviousRootDirectory;
        if (string.IsNullOrWhiteSpace(previousRoot))
        {
            return false;
        }
        try
        {
            DesktopStateRootStore.RollbackMigration(startupError.Message);
            DesktopStateRootMigration.ScheduleRelaunchAfterExit(previousRoot);
            startupTimeline.Mark("state_root.migration_rolled_back", new Dictionary<string, string>
            {
                ["error"] = startupError.Message,
                ["state_root"] = previousRoot,
            });
            RequestApplicationExit(0);
            return true;
        }
        catch (Exception rollbackError)
        {
            startupTimeline.Mark("state_root.migration_rollback_failed", new Dictionary<string, string>
            {
                ["error"] = rollbackError.Message,
            });
            return false;
        }
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
