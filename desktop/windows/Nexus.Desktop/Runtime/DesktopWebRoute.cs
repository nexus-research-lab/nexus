using Nexus.Desktop.Sidecar;

namespace Nexus.Desktop.Runtime;

public sealed record DesktopWebRoute(string Path, string Entry)
{
    private const string OAuthCallbackPath = "/capability/connectors/oauth/callback";

    public static DesktopWebRoute Launcher { get; } = new("/launcher", "app");
    public static DesktopWebRoute App { get; } = new("/app", "app");
    public static DesktopWebRoute Settings { get; } = new("/settings", "settings");

    public static DesktopWebRoute FromPath(string path)
    {
        string normalized = NormalizePath(path);
        if (normalized == Settings.Path)
        {
            return Settings;
        }
        if (normalized.StartsWith(OAuthCallbackPath, StringComparison.OrdinalIgnoreCase))
        {
            return new DesktopWebRoute(normalized, "oauth-callback");
        }
        return new DesktopWebRoute(normalized, "app");
    }

    public Uri ToUri(SidecarRuntimeConfig runtime)
    {
        string fileName = Entry.EndsWith(".html", StringComparison.OrdinalIgnoreCase) ? Entry : $"{Entry}.html";
        string query = $"desktop_route={Uri.EscapeDataString(Path)}";
        return new Uri(new Uri(runtime.WebBaseUrl), $"{fileName}?{query}");
    }

    private static string NormalizePath(string path)
    {
        string candidate = path.Trim();
        if (!candidate.StartsWith("/", StringComparison.Ordinal) || candidate.StartsWith("//", StringComparison.Ordinal))
        {
            return Launcher.Path;
        }
        return candidate;
    }
}
