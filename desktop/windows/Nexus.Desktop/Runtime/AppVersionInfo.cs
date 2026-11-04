using System.Reflection;

namespace Nexus.Desktop.Runtime;

internal static class AppVersionInfo
{
    public static bool IsPreview => Assembly.GetExecutingAssembly()
        .GetCustomAttributes<AssemblyMetadataAttribute>()
        .Any(attribute => attribute.Key == "NexusPreview" && attribute.Value == "true");

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
