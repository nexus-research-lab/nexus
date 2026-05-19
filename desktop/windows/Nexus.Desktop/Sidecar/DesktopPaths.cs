using System.IO;

namespace Nexus.Desktop.Sidecar;

internal static class DesktopPaths
{
    public static string ApplicationDataDirectory =>
        Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Nexus");

    public static string ConfigDirectory => Path.Combine(ApplicationDataDirectory, "config");

    public static string WorkspaceDirectory => Path.Combine(ApplicationDataDirectory, "workspace");

    public static string CacheDirectory => Path.Combine(ApplicationDataDirectory, "cache");

    public static string LogsDirectory => Path.Combine(ApplicationDataDirectory, "Logs");
}
