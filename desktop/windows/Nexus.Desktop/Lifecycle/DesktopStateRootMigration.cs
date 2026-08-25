using System.Diagnostics;
using System.IO;
using Nexus.Desktop.Sidecar;

namespace Nexus.Desktop.Lifecycle;

internal static class DesktopStateRootMigration
{
    private const string MigrateArgument = "--nexus-state-root-migrate";
    private const string RelaunchArgument = "--nexus-state-root-relaunch";
    private const string ParentPIDArgument = "--parent-pid";
    private const string BrowserPIDArgument = "--browser-pid";
    private const string SourceArgument = "--source";
    private const string TargetArgument = "--target";
    private const int ProcessExitTimeoutMilliseconds = 45_000;
    private static readonly object ScheduleLock = new();
    private static bool migrationScheduled;

    public static bool TryRunHelper(string[] arguments)
    {
        if (!arguments.Contains(MigrateArgument, StringComparer.OrdinalIgnoreCase)
            && !arguments.Contains(RelaunchArgument, StringComparer.OrdinalIgnoreCase))
        {
            return false;
        }

        string? source = ValueAfter(SourceArgument, arguments);
        Process? browserProcess = null;
        try
        {
            if (!int.TryParse(ValueAfter(ParentPIDArgument, arguments), out int parentPID))
            {
                throw new ArgumentException("状态根迁移参数无效。");
            }
            if (arguments.Contains(MigrateArgument, StringComparer.OrdinalIgnoreCase))
            {
                if (!int.TryParse(ValueAfter(BrowserPIDArgument, arguments), out int browserPID))
                {
                    throw new ArgumentException("WebView2 迁移参数无效。");
                }
                // 先固定进程句柄；宿主退出后 WebView2 仍可能短暂占用 UDF，且 PID 可能被复用。
                browserProcess = TryGetProcess(browserPID);
            }
            WaitForParentExit(parentPID);
            WaitForBrowserExit(browserProcess);
            if (arguments.Contains(MigrateArgument, StringComparer.OrdinalIgnoreCase))
            {
                string target = ValueAfter(TargetArgument, arguments)
                    ?? throw new ArgumentException("状态根迁移目标缺失。");
                source = source ?? throw new ArgumentException("状态根迁移来源缺失。");
                source = DesktopStateRootStore.NormalizePath(source);
                target = DesktopStateRootStore.NormalizePath(target);
                Validate(source, target);
                CopyStateRoot(source, target);
                DesktopStateRootStore.ActivateMigration(source, target);
            }
        }
        catch (Exception exception)
        {
            if (!string.IsNullOrWhiteSpace(source))
            {
                DesktopStateRootStore.RecordMigrationFailure(source, exception.Message);
            }
            Trace.WriteLine($"[Nexus State Root] migration helper failed: {exception.Message}");
        }
        finally
        {
            browserProcess?.Dispose();
        }

        try
        {
            RelaunchApplication();
        }
        catch (Exception exception)
        {
            Trace.WriteLine($"[Nexus State Root] relaunch failed: {exception.Message}");
        }
        return true;
    }

    public static string ScheduleMigration(string rawPath, uint browserProcessID)
    {
        lock (ScheduleLock)
        {
            if (migrationScheduled)
            {
                throw new InvalidOperationException("Nexus 数据目录迁移已经开始。");
            }
            string source = DesktopPaths.RootDirectory;
            string target = string.IsNullOrWhiteSpace(rawPath)
                ? DesktopStateRootStore.DefaultRootDirectory
                : DesktopStateRootStore.NormalizePath(rawPath);
            Validate(source, target);
            StartHelper([
                MigrateArgument,
                ParentPIDArgument,
                Environment.ProcessId.ToString(),
                BrowserPIDArgument,
                browserProcessID.ToString(),
                SourceArgument,
                source,
                TargetArgument,
                target,
            ]);
            migrationScheduled = true;
            return target;
        }
    }

    public static void ScheduleRelaunchAfterExit(string source)
    {
        StartHelper([
            RelaunchArgument,
            ParentPIDArgument,
            Environment.ProcessId.ToString(),
            SourceArgument,
            source,
        ]);
    }

    private static void StartHelper(IEnumerable<string> arguments)
    {
        string executablePath = Environment.ProcessPath
            ?? throw new InvalidOperationException("无法定位 Nexus 桌面应用可执行文件。");
        ProcessStartInfo startInfo = new()
        {
            FileName = executablePath,
            UseShellExecute = false,
            CreateNoWindow = true,
        };
        foreach (string argument in arguments)
        {
            startInfo.ArgumentList.Add(argument);
        }
        _ = Process.Start(startInfo)
            ?? throw new InvalidOperationException("无法启动 Nexus 状态根迁移进程。");
    }

    private static void Validate(string source, string target)
    {
        source = DesktopStateRootStore.NormalizePath(source);
        target = DesktopStateRootStore.NormalizePath(target);
        DesktopStateRootStore.ValidateManagedRoot(source);
        DesktopStateRootStore.ValidateManagedRoot(target);
        if (DesktopStateRootStore.SameManagedRoot(source, target))
        {
            throw new ArgumentException("新目录与当前 Nexus 数据目录相同。");
        }
        if (DesktopStateRootStore.ManagedRootContains(source, target)
            || DesktopStateRootStore.ManagedRootContains(target, source))
        {
            throw new ArgumentException("新旧 Nexus 数据目录不能互相包含。");
        }
        if (!Directory.Exists(source))
        {
            throw new DirectoryNotFoundException("当前 Nexus 数据目录不存在。");
        }
        RequireEmptyOrMissingDirectory(target);
    }

    private static void RequireEmptyOrMissingDirectory(string target)
    {
        if (File.Exists(target))
        {
            throw new ArgumentException("目标路径已存在且不是目录。");
        }
        if (Directory.Exists(target) && Directory.EnumerateFileSystemEntries(target).Any())
        {
            throw new ArgumentException("目标目录必须为空。");
        }
    }

    private static void CopyStateRoot(string source, string target)
    {
        string parent = Path.GetDirectoryName(target)
            ?? throw new ArgumentException("目标目录缺少父目录。");
        Directory.CreateDirectory(parent);
        string staging = Path.Combine(
            parent,
            $".{Path.GetFileName(target)}.nexus-migration-{Guid.NewGuid():N}");
        try
        {
            CopyDirectory(source, staging);
            if (Directory.Exists(target))
            {
                Directory.Delete(target);
            }
            Directory.Move(staging, target);
        }
        catch
        {
            TryDeleteDirectory(staging);
            throw;
        }
    }

    private static void CopyDirectory(string source, string target)
    {
        DirectoryInfo sourceDirectory = new(source);
        Directory.CreateDirectory(target);
        foreach (FileSystemInfo entry in sourceDirectory.EnumerateFileSystemInfos())
        {
            string destination = Path.Combine(target, entry.Name);
            if (!string.IsNullOrWhiteSpace(entry.LinkTarget))
            {
                if (entry is DirectoryInfo)
                {
                    Directory.CreateSymbolicLink(destination, entry.LinkTarget);
                }
                else
                {
                    File.CreateSymbolicLink(destination, entry.LinkTarget);
                }
                continue;
            }
            if (entry is DirectoryInfo directory)
            {
                CopyDirectory(directory.FullName, destination);
                Directory.SetLastWriteTimeUtc(destination, directory.LastWriteTimeUtc);
                continue;
            }
            FileInfo file = (FileInfo)entry;
            file.CopyTo(destination, overwrite: false);
            File.SetLastWriteTimeUtc(destination, file.LastWriteTimeUtc);
            File.SetAttributes(destination, file.Attributes);
        }
        Directory.SetLastWriteTimeUtc(target, sourceDirectory.LastWriteTimeUtc);
    }

    private static void TryDeleteDirectory(string path)
    {
        try
        {
            if (Directory.Exists(path))
            {
                Directory.Delete(path, recursive: true);
            }
        }
        catch (Exception exception) when (exception is IOException or UnauthorizedAccessException)
        {
        }
    }

    private static void WaitForParentExit(int parentPID)
    {
        try
        {
            using Process parent = Process.GetProcessById(parentPID);
            if (!parent.WaitForExit(ProcessExitTimeoutMilliseconds))
            {
                throw new TimeoutException("Nexus 主进程未能及时退出，迁移已取消。");
            }
        }
        catch (ArgumentException)
        {
        }
    }

    private static Process? TryGetProcess(int processID)
    {
        try
        {
            return Process.GetProcessById(processID);
        }
        catch (ArgumentException)
        {
            return null;
        }
    }

    private static void WaitForBrowserExit(Process? browserProcess)
    {
        if (browserProcess is not null && !browserProcess.WaitForExit(ProcessExitTimeoutMilliseconds))
        {
            throw new TimeoutException("WebView2 未能及时释放数据目录，迁移已取消。");
        }
    }

    private static void RelaunchApplication()
    {
        string executablePath = Environment.ProcessPath
            ?? throw new InvalidOperationException("无法定位 Nexus 桌面应用可执行文件。");
        Process.Start(new ProcessStartInfo
        {
            FileName = executablePath,
            UseShellExecute = true,
        });
    }

    private static string? ValueAfter(string key, IReadOnlyList<string> arguments)
    {
        for (int index = 0; index < arguments.Count - 1; index++)
        {
            if (string.Equals(arguments[index], key, StringComparison.OrdinalIgnoreCase))
            {
                return arguments[index + 1];
            }
        }
        return null;
    }

}
