using System.Reflection;

namespace Nexus.Desktop.Runtime;

internal static class AppVersionInfo
{
    public static string Version =>
        Assembly.GetExecutingAssembly().GetCustomAttribute<AssemblyInformationalVersionAttribute>()?.InformationalVersion
        ?? "0.0.0";

    public static string BuildNumber =>
        Assembly.GetExecutingAssembly()
            .GetCustomAttributes<AssemblyMetadataAttribute>()
            .FirstOrDefault(attribute => attribute.Key == "NexusBuildNumber")
            ?.Value
        ?? "dev";
}
