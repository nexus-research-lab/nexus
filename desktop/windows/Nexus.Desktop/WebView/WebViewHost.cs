using System.Text.Json;
using System.Diagnostics;
using System.IO;
using Microsoft.Web.WebView2.Core;
using Microsoft.Web.WebView2.Wpf;
using Nexus.Desktop.Bridge;
using Nexus.Desktop.Diagnostics;
using Nexus.Desktop.Runtime;
using Nexus.Desktop.Sidecar;

namespace Nexus.Desktop.WebView;

internal sealed class WebViewHost : IDisposable
{
    private readonly WebView2 webView;
    private readonly SidecarRuntimeConfig runtime;
    private readonly DesktopStartupTimeline startupTimeline;
    private DesktopBridgeHandler? bridgeHandler;
    private bool disposed;

    public WebViewHost(
        WebView2 webView,
        SidecarRuntimeConfig runtime,
        DesktopStartupTimeline startupTimeline)
    {
        this.webView = webView;
        this.runtime = runtime;
        this.startupTimeline = startupTimeline;
    }

    public async Task InitializeAsync()
    {
        startupTimeline.Mark("webview.initialize_begin");
        string userDataFolder = Path.Combine(DesktopPaths.CacheDirectory, "WebView2", "main");
        Directory.CreateDirectory(userDataFolder);
        webView.DefaultBackgroundColor = System.Drawing.Color.Transparent;

        var options = new CoreWebView2EnvironmentOptions
        {
            AdditionalBrowserArguments = "--disable-renderer-backgrounding --disable-background-timer-throttling --disable-backgrounding-occluded-windows",
        };
        CoreWebView2Environment environment = await CoreWebView2Environment.CreateAsync(null, userDataFolder, options);
        await webView.EnsureCoreWebView2Async(environment);

        CoreWebView2 core = webView.CoreWebView2;
        core.Settings.AreDefaultContextMenusEnabled = false;
        core.Settings.AreDevToolsEnabled = true;
        core.Settings.IsStatusBarEnabled = false;
        core.Settings.IsZoomControlEnabled = false;
        core.Settings.IsGeneralAutofillEnabled = false;
        core.Settings.IsPasswordAutosaveEnabled = false;

        InstallDesktopSessionCookie(core);
        await core.AddScriptToExecuteOnDocumentCreatedAsync(DesktopRuntimeScript.Make(runtime));
        await core.AddScriptToExecuteOnDocumentCreatedAsync(DesktopBridgeScript.Make());

        bridgeHandler = new DesktopBridgeHandler(core, runtime, startupTimeline, OpenRouteAsync);
        core.WebMessageReceived += async (_, args) => await HandleWebMessageAsync(args);
        core.NavigationStarting += HandleNavigationStarting;
        core.NavigationCompleted += (_, _) => startupTimeline.Mark("webview.navigation_completed");
        core.NewWindowRequested += HandleNewWindowRequested;
        core.ProcessFailed += (_, args) =>
        {
            startupTimeline.Mark("webview.process_failed", new Dictionary<string, string>
            {
                ["kind"] = args.ProcessFailedKind.ToString(),
            });
            DesktopDiagnosticsReport.WriteRuntimeIssue(
                prefix: "webview-process-failed",
                reason: args.ProcessFailedKind.ToString(),
                runtime: runtime,
                startupTimeline: startupTimeline,
                details: new Dictionary<string, object?>
                {
                    ["process_failed_kind"] = args.ProcessFailedKind.ToString(),
                });
        };
        startupTimeline.Mark("webview.initialize_ready");
    }

    public Task LoadRouteAsync(DesktopWebRoute route)
    {
        ObjectDisposedException.ThrowIf(disposed, this);
        Uri url = route.ToUri(runtime);
        startupTimeline.Mark("main_window.route_load", new Dictionary<string, string>
        {
            ["path"] = route.Path,
        });
        webView.Source = url;
        return Task.CompletedTask;
    }

    public void Dispose()
    {
        if (disposed)
        {
            return;
        }
        disposed = true;
        try
        {
            webView.CoreWebView2?.Stop();
        }
        catch (InvalidOperationException)
        {
        }
        webView.Dispose();
    }

    private async Task HandleWebMessageAsync(CoreWebView2WebMessageReceivedEventArgs args)
    {
        using JsonDocument document = JsonDocument.Parse(args.WebMessageAsJson);
        JsonElement root = document.RootElement;
        string channel = root.TryGetProperty("channel", out JsonElement channelElement)
            ? channelElement.GetString() ?? string.Empty
            : string.Empty;

        if (!root.TryGetProperty("payload", out JsonElement payload))
        {
            return;
        }

        switch (channel)
        {
            case "nexusDesktopLifecycle":
                HandleLifecycleMessage(payload);
                break;
            case "nexusDesktop":
                if (bridgeHandler is not null)
                {
                    await bridgeHandler.HandleAsync(payload);
                }
                break;
        }
    }

    private void HandleLifecycleMessage(JsonElement payload)
    {
        string kind = payload.TryGetProperty("kind", out JsonElement kindElement)
            ? kindElement.GetString() ?? string.Empty
            : string.Empty;
        if (kind != "web.ready")
        {
            return;
        }

        string location = payload.TryGetProperty("location", out JsonElement locationElement)
            ? locationElement.GetString() ?? string.Empty
            : string.Empty;
        string source = payload.TryGetProperty("source", out JsonElement sourceElement)
            ? sourceElement.GetString() ?? string.Empty
            : string.Empty;
        string reducedMotion = payload.TryGetProperty("reduced_motion", out JsonElement reducedMotionElement) &&
            reducedMotionElement.ValueKind is JsonValueKind.True or JsonValueKind.False
            ? reducedMotionElement.GetBoolean().ToString().ToLowerInvariant()
            : "unknown";
        startupTimeline.Mark("web.ready", new Dictionary<string, string>
        {
            ["location_path"] = string.IsNullOrWhiteSpace(location) ? "/" : location,
            ["reduced_motion"] = reducedMotion,
            ["source"] = source,
            ["surface"] = "main",
        });
    }

    private Task OpenRouteAsync(string route)
    {
        webView.Source = DesktopWebRoute.FromPath(route).ToUri(runtime);
        return Task.CompletedTask;
    }

    private void InstallDesktopSessionCookie(CoreWebView2 core)
    {
        if (!Uri.TryCreate(runtime.WebBaseUrl, UriKind.Absolute, out Uri? webBaseUri))
        {
            startupTimeline.Mark("webview.cookie_failed", new Dictionary<string, string>
            {
                ["reason"] = "invalid_web_base_url",
            });
            return;
        }

        startupTimeline.Mark("webview.cookie_begin", new Dictionary<string, string>
        {
            ["host"] = webBaseUri.Host,
        });
        CoreWebView2Cookie cookie = core.CookieManager.CreateCookie(
            "nexus_desktop_token",
            runtime.SessionToken,
            webBaseUri.Host,
            "/");
        core.CookieManager.AddOrUpdateCookie(cookie);
        startupTimeline.Mark("webview.cookie_ready", new Dictionary<string, string>
        {
            ["host"] = webBaseUri.Host,
        });
    }

    private void HandleNavigationStarting(object? sender, CoreWebView2NavigationStartingEventArgs args)
    {
        if (ShouldKeepInsideWebView(args.Uri))
        {
            return;
        }

        args.Cancel = true;
        HandleExternalNavigation(args.Uri);
    }

    private void HandleNewWindowRequested(object? sender, CoreWebView2NewWindowRequestedEventArgs args)
    {
        args.Handled = true;
        HandleExternalNavigation(args.Uri);
    }

    private bool ShouldKeepInsideWebView(string rawUrl)
    {
        if (!Uri.TryCreate(rawUrl, UriKind.Absolute, out Uri? uri))
        {
            return false;
        }

        if (!Uri.TryCreate(runtime.WebBaseUrl, UriKind.Absolute, out Uri? webBaseUrl))
        {
            return false;
        }

        return uri.Scheme == webBaseUrl.Scheme &&
            uri.Host == webBaseUrl.Host &&
            uri.Port == webBaseUrl.Port;
    }

    private void HandleExternalNavigation(string rawUrl)
    {
        if (!Uri.TryCreate(rawUrl, UriKind.Absolute, out Uri? uri))
        {
            startupTimeline.Mark("webview.navigation_blocked", new Dictionary<string, string>
            {
                ["reason"] = "invalid_uri",
            });
            return;
        }

        if (string.Equals(uri.Scheme, "nexus", StringComparison.OrdinalIgnoreCase))
        {
            webView.Source = DesktopProtocolRouter.RouteFromActivationMessage(rawUrl).ToUri(runtime);
            startupTimeline.Mark("webview.navigation_protocol_route", new Dictionary<string, string>
            {
                ["scheme"] = "nexus",
            });
            return;
        }

        if (uri.Scheme is "http" or "https" or "mailto")
        {
            Process.Start(new ProcessStartInfo
            {
                FileName = rawUrl,
                UseShellExecute = true,
            });
            startupTimeline.Mark("webview.navigation_external_opened", new Dictionary<string, string>
            {
                ["scheme"] = uri.Scheme,
            });
            return;
        }

        startupTimeline.Mark("webview.navigation_blocked", new Dictionary<string, string>
        {
            ["scheme"] = uri.Scheme,
        });
    }
}
