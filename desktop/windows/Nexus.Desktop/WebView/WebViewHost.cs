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
    private readonly Func<DesktopWebRoute, string, Task> recreateAfterProcessFailureAsync;
    private DesktopBridgeHandler? bridgeHandler;
    private bool disposed;
    private bool resumeCheckInFlight;
    private DateTimeOffset lastResumeCheckAt = DateTimeOffset.MinValue;
    private DesktopWebRoute lastRoute = DesktopWebRoute.Launcher;

    public WebViewHost(
        WebView2 webView,
        SidecarRuntimeConfig runtime,
        DesktopStartupTimeline startupTimeline,
        Func<DesktopWebRoute, string, Task> recreateAfterProcessFailureAsync)
    {
        this.webView = webView;
        this.runtime = runtime;
        this.startupTimeline = startupTimeline;
        this.recreateAfterProcessFailureAsync = recreateAfterProcessFailureAsync;
    }

    public async Task InitializeAsync()
    {
        startupTimeline.Mark("webview.initialize_begin");
        string userDataFolder = Path.Combine(DesktopPaths.CacheDirectory, "WebView2", "main");
        Directory.CreateDirectory(userDataFolder);
        webView.DefaultBackgroundColor = System.Drawing.Color.Transparent;

        var options = new CoreWebView2EnvironmentOptions
        {
            AdditionalBrowserArguments = string.Join(
                " ",
                "--disable-renderer-backgrounding",
                "--disable-background-timer-throttling",
                "--disable-backgrounding-occluded-windows",
                "--disable-features=CalculateNativeWinOcclusion"),
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
            Dictionary<string, string> metadata = new()
            {
                ["kind"] = args.ProcessFailedKind.ToString(),
                ["path"] = lastRoute.Path,
            };
            string? diagnosticsPath = DesktopDiagnosticsReport.WriteRuntimeIssue(
                prefix: "webview-process-failed",
                reason: args.ProcessFailedKind.ToString(),
                runtime: runtime,
                startupTimeline: startupTimeline,
                details: new Dictionary<string, object?>
                {
                    ["process_failed_kind"] = args.ProcessFailedKind.ToString(),
                    ["route_path"] = lastRoute.Path,
                });
            if (!string.IsNullOrWhiteSpace(diagnosticsPath))
            {
                metadata["diagnostics_path"] = diagnosticsPath;
            }
            startupTimeline.Mark("webview.process_failed", metadata);
            _ = recreateAfterProcessFailureAsync(lastRoute, args.ProcessFailedKind.ToString());
        };
        startupTimeline.Mark("webview.initialize_ready");
    }

    public Task LoadRouteAsync(DesktopWebRoute route)
    {
        ObjectDisposedException.ThrowIf(disposed, this);
        Uri url = route.ToUri(runtime);
        lastRoute = route;
        startupTimeline.Mark("main_window.route_load", new Dictionary<string, string>
        {
            ["path"] = route.Path,
        });
        webView.Source = url;
        return Task.CompletedTask;
    }

    public async Task RecoverAfterWindowShownAsync(string reason)
    {
        if (disposed || resumeCheckInFlight || webView.CoreWebView2 is null)
        {
            return;
        }

        DateTimeOffset now = DateTimeOffset.UtcNow;
        if (now - lastResumeCheckAt < TimeSpan.FromSeconds(5))
        {
            return;
        }

        lastResumeCheckAt = now;
        resumeCheckInFlight = true;
        try
        {
            webView.InvalidateVisual();
            webView.UpdateLayout();
            await Task.Delay(150);
            if (disposed || webView.CoreWebView2 is null)
            {
                return;
            }

            string result = await webView.CoreWebView2.ExecuteScriptAsync(
                """
                (() => {
                  window.dispatchEvent(new Event("resize"));
                  document.documentElement.style.setProperty("--nexus-webview-resume", String(Date.now()));
                  if (document.body) {
                    document.body.getBoundingClientRect();
                  }
                  const root = document.getElementById("root");
                  return Boolean(root && root.childElementCount > 0 && document.readyState !== "loading");
                })();
                """);
            if (IsTruthyScriptResult(result))
            {
                startupTimeline.Mark("webview.resume_check_ready", new Dictionary<string, string>
                {
                    ["path"] = lastRoute.Path,
                    ["reason"] = reason,
                });
                return;
            }

            ReloadAfterResumeProbe(reason, "empty_or_loading_root");
        }
        catch (Exception exception) when (exception is InvalidOperationException or ObjectDisposedException)
        {
            startupTimeline.Mark("webview.resume_check_skipped", new Dictionary<string, string>
            {
                ["error"] = TrimMetadata(exception.Message),
                ["path"] = lastRoute.Path,
                ["reason"] = reason,
            });
        }
        catch (Exception exception)
        {
            ReloadAfterResumeProbe(reason, exception.GetType().Name);
        }
        finally
        {
            resumeCheckInFlight = false;
        }
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

    private void ReloadAfterResumeProbe(string reason, string probeResult)
    {
        if (disposed || webView.CoreWebView2 is null)
        {
            return;
        }

        startupTimeline.Mark("webview.resume_reload", new Dictionary<string, string>
        {
            ["path"] = lastRoute.Path,
            ["probe"] = TrimMetadata(probeResult),
            ["reason"] = reason,
        });
        webView.CoreWebView2.Reload();
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
        DesktopWebRoute nextRoute = DesktopWebRoute.FromPath(route);
        lastRoute = nextRoute;
        webView.Source = nextRoute.ToUri(runtime);
        return Task.CompletedTask;
    }

    private static string TrimMetadata(string value)
    {
        string normalized = value.Trim();
        const int maxLength = 240;
        if (normalized.Length <= maxLength)
        {
            return normalized;
        }
        return normalized[..maxLength] + "...";
    }

    private static bool IsTruthyScriptResult(string value)
    {
        return string.Equals(value.Trim(), "true", StringComparison.OrdinalIgnoreCase);
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
