using System.IO;

namespace Nexus.Desktop.Sidecar;

internal static class DesktopPaths
{
    public static string RootDirectory =>
        Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.UserProfile), ".nexus");

    public static string DataDirectory => Path.Combine(RootDirectory, "data");

    public static string ApplicationDataDirectory => RootDirectory;

    public static string ConfigDirectory => Path.Combine(RootDirectory, "config");

    public static string WorkspaceDirectory => Path.Combine(RootDirectory, "workspace");

    public static string CacheDirectory => Path.Combine(RootDirectory, "cache");

    public static string LogsDirectory => Path.Combine(RootDirectory, "logs");
}
