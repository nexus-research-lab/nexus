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
    private const int ResumeProbeRecreateThreshold = 2;

    private readonly WebView2 webView;
    private readonly SidecarRuntimeConfig runtime;
    private readonly DesktopStartupTimeline startupTimeline;
    private readonly Func<DesktopWebRoute, string, string, Task> recreateWebViewAsync;
    private DesktopBridgeHandler? bridgeHandler;
    private bool disposed;
    private bool resumeCheckInFlight;
    private int consecutiveResumeProbeFailures;
    private DateTimeOffset lastResumeCheckAt = DateTimeOffset.MinValue;
    private DesktopWebRoute lastRoute = DesktopWebRoute.Launcher;

    private sealed record ResumeProbeResult(bool IsReady, DesktopWebRoute? CurrentRoute, string Snapshot);

    public WebViewHost(
        WebView2 webView,
        SidecarRuntimeConfig runtime,
        DesktopStartupTimeline startupTimeline,
        Func<DesktopWebRoute, string, string, Task> recreateWebViewAsync)
    {
        this.webView = webView;
        this.runtime = runtime;
        this.startupTimeline = startupTimeline;
        this.recreateWebViewAsync = recreateWebViewAsync;
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
        await DesktopWebCacheInvalidator.ClearCachesIfNeededAsync(core, runtime, startupTimeline);
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
            _ = recreateWebViewAsync(lastRoute, "process_failed", args.ProcessFailedKind.ToString());
        };
        startupTimeline.Mark("webview.initialize_ready");
    }

    public Task LoadRouteAsync(DesktopWebRoute route)
    {
        ObjectDisposedException.ThrowIf(disposed, this);
        lastRoute = route;
        startupTimeline.Mark("main_window.route_load", new Dictionary<string, string>
        {
            ["path"] = route.Path,
        });
        NavigateToRoute(route);
        return Task.CompletedTask;
    }

    public async Task ReloadAsync(string reason)
    {
        if (disposed)
        {
            return;
        }
        if (webView.CoreWebView2 is null)
        {
            await recreateWebViewAsync(lastRoute, "manual_reload", reason);
            return;
        }

        try
        {
            ResumeProbeResult probe = await CaptureResumeProbeAsync();
            UpdateLastRouteFromProbe(probe);
            DesktopWebRoute route = probe.CurrentRoute ?? lastRoute;
            startupTimeline.Mark("webview.manual_reload", new Dictionary<string, string>
            {
                ["path"] = route.Path,
                ["reason"] = reason,
            });
            NavigateToRoute(route);
        }
        catch (Exception exception) when (exception is InvalidOperationException or ObjectDisposedException)
        {
            await recreateWebViewAsync(lastRoute, "manual_reload", exception.GetType().Name);
        }
    }

    public async Task ClearCacheAndReloadAsync(string reason)
    {
        if (disposed)
        {
            return;
        }
        if (webView.CoreWebView2 is null)
        {
            await recreateWebViewAsync(lastRoute, "manual_cache_clear", reason);
            return;
        }

        try
        {
            ResumeProbeResult probe = await CaptureResumeProbeAsync();
            UpdateLastRouteFromProbe(probe);
            DesktopWebRoute route = probe.CurrentRoute ?? lastRoute;
            await DesktopWebCacheInvalidator.ClearCachesManuallyAsync(
                webView.CoreWebView2,
                runtime,
                startupTimeline,
                reason);
            NavigateToRoute(route);
        }
        catch (Exception exception) when (exception is InvalidOperationException or ObjectDisposedException)
        {
            await recreateWebViewAsync(lastRoute, "manual_cache_clear", exception.GetType().Name);
        }
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

            ResumeProbeResult probe = await CaptureResumeProbeAsync();
            UpdateLastRouteFromProbe(probe);
            if (probe.IsReady)
            {
                consecutiveResumeProbeFailures = 0;
                startupTimeline.Mark("webview.resume_check_ready", new Dictionary<string, string>
                {
                    ["path"] = lastRoute.Path,
                    ["reason"] = reason,
                });
                return;
            }

            await HandleResumeProbeFailureAsync(reason, "empty_or_loading_root", probe);
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
            await HandleResumeProbeFailureAsync(reason, exception.GetType().Name);
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

    private async Task HandleResumeProbeFailureAsync(
        string reason,
        string probeResult,
        ResumeProbeResult? probe = null)
    {
        if (disposed || webView.CoreWebView2 is null)
        {
            return;
        }

        consecutiveResumeProbeFailures++;
        probe ??= await CaptureResumeProbeAsync();
        UpdateLastRouteFromProbe(probe);
        DesktopWebRoute recoveryRoute = probe.CurrentRoute ?? lastRoute;
        string probeSnapshot = probe.Snapshot;
        Dictionary<string, string> metadata = new()
        {
            ["path"] = recoveryRoute.Path,
            ["probe"] = TrimMetadata(probeResult),
            ["reason"] = reason,
            ["failure_count"] = consecutiveResumeProbeFailures.ToString(),
        };
        if (!string.IsNullOrWhiteSpace(probeSnapshot))
        {
            metadata["probe_snapshot"] = TrimMetadata(probeSnapshot);
        }

        if (consecutiveResumeProbeFailures < ResumeProbeRecreateThreshold)
        {
            startupTimeline.Mark("webview.resume_reload", metadata);
            NavigateToRoute(recoveryRoute);
            return;
        }

        string? diagnosticsPath = DesktopDiagnosticsReport.WriteRuntimeIssue(
            prefix: "webview-resume-failed",
            reason: probeResult,
            runtime: runtime,
            startupTimeline: startupTimeline,
            details: new Dictionary<string, object?>
            {
                ["route_path"] = recoveryRoute.Path,
                ["resume_reason"] = reason,
                ["probe_result"] = probeResult,
                ["failure_count"] = consecutiveResumeProbeFailures,
                ["probe_snapshot"] = TrimDiagnosticDetail(probeSnapshot),
            });
        if (!string.IsNullOrWhiteSpace(diagnosticsPath))
        {
            metadata["diagnostics_path"] = diagnosticsPath;
        }
        startupTimeline.Mark("webview.resume_probe_recreate", metadata);
        await recreateWebViewAsync(recoveryRoute, "resume_probe", probeResult);
    }

    private async Task<ResumeProbeResult> CaptureResumeProbeAsync()
    {
        if (disposed || webView.CoreWebView2 is null)
        {
            return new ResumeProbeResult(false, null, string.Empty);
        }

        try
        {
            string result = await webView.CoreWebView2.ExecuteScriptAsync(
                """
                (() => {
                  window.dispatchEvent(new Event("resize"));
                  document.documentElement.style.setProperty("--nexus-webview-resume", String(Date.now()));
                  if (document.body) {
                    document.body.getBoundingClientRect();
                  }
                  const root = document.getElementById("root");
                  return {
                    isReady: Boolean(root && root.childElementCount > 0 && document.readyState !== "loading"),
                    href: window.location.href,
                    path: `${window.location.pathname}${window.location.search}${window.location.hash}`,
                    readyState: document.readyState,
                    title: document.title,
                    hasRoot: Boolean(root),
                    rootChildren: root?.childElementCount ?? -1,
                    bodyChildren: document.body?.childElementCount ?? -1,
                    bodyTextLength: document.body?.innerText?.length ?? -1
                  };
                })();
                """);
            return ParseResumeProbeResult(result);
        }
        catch (Exception exception) when (exception is InvalidOperationException or ObjectDisposedException)
        {
            string snapshot = exception.GetType().Name + ": " + exception.Message;
            return new ResumeProbeResult(false, null, snapshot);
        }
    }

    private void UpdateLastRouteFromProbe(ResumeProbeResult probe)
    {
        if (probe.CurrentRoute is not null)
        {
            lastRoute = probe.CurrentRoute;
        }
    }

    private static ResumeProbeResult ParseResumeProbeResult(string rawResult)
    {
        try
        {
            using JsonDocument document = JsonDocument.Parse(rawResult);
            if (document.RootElement.ValueKind == JsonValueKind.String)
            {
                string snapshot = document.RootElement.GetString() ?? string.Empty;
                if (!snapshot.TrimStart().StartsWith("{", StringComparison.Ordinal))
                {
                    return new ResumeProbeResult(false, null, snapshot);
                }

                using JsonDocument nestedDocument = JsonDocument.Parse(snapshot);
                return ParseResumeProbeObject(nestedDocument.RootElement, snapshot);
            }

            return ParseResumeProbeObject(document.RootElement, document.RootElement.GetRawText());
        }
        catch (JsonException)
        {
            return new ResumeProbeResult(false, null, rawResult);
        }
    }

    private static ResumeProbeResult ParseResumeProbeObject(JsonElement root, string snapshot)
    {
        bool isReady = root.TryGetProperty("isReady", out JsonElement readyElement) &&
            readyElement.ValueKind is JsonValueKind.True;
        string path = JsonString(root, "path");
        return new ResumeProbeResult(isReady, RouteFromProbePath(path), snapshot);
    }

    private static DesktopWebRoute? RouteFromProbePath(string path)
    {
        string candidate = path.Trim();
        if (!candidate.StartsWith("/", StringComparison.Ordinal) ||
            candidate.StartsWith("//", StringComparison.Ordinal))
        {
            return null;
        }

        if (!Uri.TryCreate("http://nexus.local" + candidate, UriKind.Absolute, out Uri? uri))
        {
            return DesktopWebRoute.FromPath(candidate);
        }

        string? desktopRoute = QueryValue(uri.Query, "desktop_route");
        if (!string.IsNullOrWhiteSpace(desktopRoute))
        {
            return DesktopWebRoute.FromPath(desktopRoute);
        }

        string routePath = uri.AbsolutePath + uri.Query + uri.Fragment;
        return DesktopWebRoute.FromPath(routePath);
    }

    private static string JsonString(JsonElement root, string propertyName)
    {
        if (!root.TryGetProperty(propertyName, out JsonElement element) ||
            element.ValueKind != JsonValueKind.String)
        {
            return string.Empty;
        }
        return element.GetString() ?? string.Empty;
    }

    private static string? QueryValue(string query, string key)
    {
        string normalized = query.TrimStart('?');
        foreach (string part in normalized.Split('&', StringSplitOptions.RemoveEmptyEntries))
        {
            string[] pieces = part.Split('=', 2);
            string name = Uri.UnescapeDataString(pieces[0].Replace("+", " "));
            if (!string.Equals(name, key, StringComparison.Ordinal))
            {
                continue;
            }
            return pieces.Length == 2
                ? Uri.UnescapeDataString(pieces[1].Replace("+", " "))
                : string.Empty;
        }
        return null;
    }

    private static string TrimDiagnosticDetail(string value)
    {
        string normalized = value.Trim();
        const int maxLength = 4096;
        if (normalized.Length <= maxLength)
        {
            return normalized;
        }
        return normalized[..maxLength] + "...";
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
        string kind = JsonOptionalString(payload, "kind");
        switch (kind)
        {
            case "web.ready":
                HandleWebReadyMessage(payload);
                break;
            case "web.fatal":
                HandleWebFatalMessage(payload);
                break;
            case "web.health":
                HandleWebHealthMessage(payload);
                break;
            default:
                startupTimeline.Mark("web.lifecycle_ignored", new Dictionary<string, string>
                {
                    ["kind"] = TrimMetadata(kind),
                    ["surface"] = "main",
                });
                break;
        }
    }

    private void HandleWebReadyMessage(JsonElement payload)
    {
        string location = JsonOptionalString(payload, "location");
        if (!string.IsNullOrWhiteSpace(location))
        {
            lastRoute = DesktopWebRoute.FromPath(location);
        }
        string source = JsonOptionalString(payload, "source");
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

    private void HandleWebFatalMessage(JsonElement payload)
    {
        Dictionary<string, string> metadata = LifecycleMetadata(payload);
        string reason = metadata.TryGetValue("message", out string? message) && !string.IsNullOrWhiteSpace(message)
            ? message
            : "web.fatal";
        Dictionary<string, object?> details = new();
        foreach (KeyValuePair<string, string> pair in metadata)
        {
            details[pair.Key] = pair.Value;
        }

        string? diagnosticsPath = DesktopDiagnosticsReport.WriteRuntimeIssue(
            prefix: "web-fatal",
            reason: reason,
            runtime: runtime,
            startupTimeline: startupTimeline,
            details: details);
        if (!string.IsNullOrWhiteSpace(diagnosticsPath))
        {
            metadata["diagnostics_path"] = diagnosticsPath;
        }
        startupTimeline.Mark("web.fatal", metadata);
    }

    private void HandleWebHealthMessage(JsonElement payload)
    {
        Dictionary<string, string> metadata = LifecycleMetadata(payload);
        string status = metadata.TryGetValue("status", out string? value) ? value : "unknown";
        string eventName = status == "ready" ? "web.health" : "web.health_unhealthy";
        startupTimeline.Mark(eventName, metadata);
    }

    private static Dictionary<string, string> LifecycleMetadata(JsonElement payload)
    {
        Dictionary<string, string> metadata = new()
        {
            ["surface"] = "main",
        };
        foreach (string key in new[] { "kind", "source", "status", "message", "name", "stack", "component_stack" })
        {
            string value = JsonOptionalString(payload, key);
            if (!string.IsNullOrWhiteSpace(value))
            {
                metadata[key] = TrimMetadata(value);
            }
        }

        string location = JsonOptionalString(payload, "location");
        if (!string.IsNullOrWhiteSpace(location))
        {
            metadata["location_path"] = TrimMetadata(location);
        }
        if (payload.TryGetProperty("snapshot", out JsonElement snapshot) && snapshot.ValueKind == JsonValueKind.Object)
        {
            foreach (string key in new[] { "path", "ready_state", "title", "has_root", "root_children", "root_text_length", "body_children", "body_text_length" })
            {
                if (snapshot.TryGetProperty(key, out JsonElement value))
                {
                    metadata[$"snapshot_{key}"] = TrimMetadata(JsonValueText(value));
                }
            }
        }
        if (payload.TryGetProperty("performance", out JsonElement performance) &&
            performance.ValueKind == JsonValueKind.Object &&
            performance.TryGetProperty("ready_ms", out JsonElement readyMS))
        {
            metadata["web_ready_ms"] = TrimMetadata(JsonValueText(readyMS));
        }
        return metadata;
    }

    private static string JsonOptionalString(JsonElement payload, string propertyName)
    {
        return payload.TryGetProperty(propertyName, out JsonElement element)
            ? JsonValueText(element)
            : string.Empty;
    }

    private static string JsonValueText(JsonElement element)
    {
        return element.ValueKind switch
        {
            JsonValueKind.String => element.GetString() ?? string.Empty,
            JsonValueKind.True => "true",
            JsonValueKind.False => "false",
            JsonValueKind.Null => string.Empty,
            JsonValueKind.Undefined => string.Empty,
            _ => element.ToString(),
        };
    }

    private Task OpenRouteAsync(string route)
    {
        DesktopWebRoute nextRoute = DesktopWebRoute.FromPath(route);
        lastRoute = nextRoute;
        NavigateToRoute(nextRoute);
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
            DesktopWebRoute nextRoute = DesktopProtocolRouter.RouteFromActivationMessage(rawUrl);
            lastRoute = nextRoute;
            NavigateToRoute(nextRoute);
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

    private void NavigateToRoute(DesktopWebRoute route)
    {
        Uri url = route.ToUri(runtime);
        if (webView.CoreWebView2 is not CoreWebView2 core)
        {
            webView.Source = url;
            return;
        }

        InstallDesktopSessionCookie(core);
        core.Navigate(url.ToString());
    }
}
