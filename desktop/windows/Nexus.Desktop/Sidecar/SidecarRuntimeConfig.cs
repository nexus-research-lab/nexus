namespace Nexus.Desktop.Sidecar;

public sealed record SidecarRuntimeConfig(
    int Port,
    string SessionToken,
    string AppVersion,
    string BuildNumber,
    string Platform)
{
    private const string OAuthCallbackPath = "capability/connectors/oauth/callback";

    public string AppMode => "desktop";
    public string WebBaseUrl => $"http://127.0.0.1:{Port}/";
    public string ApiBaseUrl => $"http://127.0.0.1:{Port}/nexus/v1";
    public string WebSocketUrl => $"ws://127.0.0.1:{Port}/nexus/v1/chat/ws";
    public string HealthUrl => $"http://127.0.0.1:{Port}/nexus/v1/health";
    public string OAuthRedirectUri => new Uri(new Uri(WebBaseUrl), OAuthCallbackPath).ToString();
}
