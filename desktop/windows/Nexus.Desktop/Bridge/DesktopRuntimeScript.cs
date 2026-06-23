using System.Text.Json;
using Nexus.Desktop.Sidecar;

namespace Nexus.Desktop.Bridge;

internal static class DesktopRuntimeScript
{
    public static string Make(SidecarRuntimeConfig runtime)
    {
        var payload = new Dictionary<string, string>
        {
            ["api_base_url"] = runtime.ApiBaseUrl,
            ["ws_url"] = runtime.WebSocketUrl,
            ["auth_token"] = runtime.SessionToken,
            ["app_mode"] = runtime.AppMode,
            ["app_version"] = runtime.AppVersion,
            ["build_number"] = runtime.BuildNumber,
            ["platform"] = runtime.Platform,
            ["oauth_redirect_uri"] = runtime.OAuthRedirectUri,
        };
        return $"window.__NEXUS_DESKTOP_RUNTIME__ = {JsonSerializer.Serialize(payload)};";
    }
}
