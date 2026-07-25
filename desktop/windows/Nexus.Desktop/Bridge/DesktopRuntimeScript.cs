using System.Text.Json;
using Nexus.Desktop.Sidecar;

namespace Nexus.Desktop.Bridge;

internal static class DesktopRuntimeScript
{
    public static string Make(SidecarRuntimeConfig runtime)
    {
        var payload = new Dictionary<string, object>
        {
            ["api_base_url"] = runtime.ApiBaseUrl,
            ["ws_url"] = runtime.WebSocketUrl,
            ["auth_token"] = runtime.SessionToken,
            ["app_mode"] = runtime.AppMode,
            ["app_version"] = runtime.AppVersion,
            ["build_number"] = runtime.BuildNumber,
            ["platform"] = runtime.Platform,
            ["oauth_redirect_uri"] = runtime.OAuthRedirectUri,
            ["desktop_window_controls_inset"] =
                Math.Max(120, System.Windows.SystemParameters.WindowCaptionButtonWidth * 3),
        };
        return $"window.__NEXUS_DESKTOP_RUNTIME__ = {JsonSerializer.Serialize(payload)};";
    }
}
