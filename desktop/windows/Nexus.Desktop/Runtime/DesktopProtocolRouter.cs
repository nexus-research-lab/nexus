namespace Nexus.Desktop.Runtime;

internal static class DesktopProtocolRouter
{
    private const string OAuthCallbackPath = "/capability/connectors/oauth/callback";
    private const string ExitCommandArgument = "--nexus-desktop-exit";
    private const string ExitActivationMessage = "NEXUS_DESKTOP_INTERNAL_EXIT";

    public static string ActivationMessage(string[] args)
    {
        if (args.Any(item => string.Equals(item, ExitCommandArgument, StringComparison.OrdinalIgnoreCase)))
        {
            return ExitActivationMessage;
        }

        return args.FirstOrDefault(item => item.StartsWith("nexus:", StringComparison.OrdinalIgnoreCase))
            ?? string.Empty;
    }

    public static bool IsExitActivationMessage(string message)
    {
        return string.Equals(message, ExitActivationMessage, StringComparison.Ordinal);
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
