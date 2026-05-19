namespace Nexus.Desktop.Runtime;

internal static class DesktopProtocolRouter
{
    private const string OAuthCallbackPath = "/capability/connectors/oauth/callback";

    public static string ActivationMessage(string[] args)
    {
        return args.FirstOrDefault(item => item.StartsWith("nexus:", StringComparison.OrdinalIgnoreCase))
            ?? string.Empty;
    }

    public static DesktopWebRoute RouteFromActivationMessage(string message)
    {
        if (string.IsNullOrWhiteSpace(message))
        {
            return DesktopWebRoute.Launcher;
        }

        if (!Uri.TryCreate(message.Trim(), UriKind.Absolute, out Uri? uri) ||
            !string.Equals(uri.Scheme, "nexus", StringComparison.OrdinalIgnoreCase))
        {
            return DesktopWebRoute.Launcher;
        }

        string host = uri.Host.ToLowerInvariant();
        string path = uri.AbsolutePath.TrimEnd('/');
        if (host is "launcher" or "open")
        {
            return DesktopWebRoute.Launcher;
        }
        if (host == "settings")
        {
            return DesktopWebRoute.Settings;
        }
        if (host == "connectors" && path.Equals("/oauth/callback", StringComparison.OrdinalIgnoreCase))
        {
            return DesktopWebRoute.FromPath($"{OAuthCallbackPath}{uri.Query}");
        }
        if (host == "capability")
        {
            return DesktopWebRoute.FromPath($"/capability{uri.AbsolutePath}{uri.Query}");
        }

        return DesktopWebRoute.Launcher;
    }
}
