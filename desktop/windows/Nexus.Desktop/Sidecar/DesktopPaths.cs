using System.IO;

namespace Nexus.Desktop.Sidecar;

internal static class DesktopPaths
{
    public static string RootDirectory => DesktopStateRootStore.ActiveRootDirectory;

    public static string AppDirectory => Path.Combine(RootDirectory, "app");

    public static string UsersDirectory => Path.Combine(RootDirectory, "users");

    public static string SystemRuntimeDirectory =>
        Path.Combine(UsersDirectory, "__system__", "runtime");

    public static string DataDirectory => Path.Combine(AppDirectory, "data");

    public static string ApplicationDataDirectory => AppDirectory;

    public static string ConfigDirectory => Path.Combine(AppDirectory, "config");

    public static string WorkspaceDirectory => UsersDirectory;

    public static string ProjectsDirectory => Path.Combine(SystemRuntimeDirectory, "projects");

    public static string SystemRuntimeHomeDirectory => Path.Combine(SystemRuntimeDirectory, "home");

    public static string SystemRuntimeCacheDirectory => Path.Combine(SystemRuntimeDirectory, "cache");

    public static string SystemRuntimeLogsDirectory => Path.Combine(SystemRuntimeDirectory, "logs");

    public static string SystemRuntimeTempDirectory => Path.Combine(SystemRuntimeDirectory, "tmp");

    public static string CacheDirectory => Path.Combine(AppDirectory, "cache");

    public static string LogsDirectory => Path.Combine(AppDirectory, "logs");

    public static string DebugDirectory => Path.Combine(AppDirectory, "debug");

    public static string SidecarPIDFilePath => Path.Combine(RootDirectory, "NexusSidecar.pid.json");
}
