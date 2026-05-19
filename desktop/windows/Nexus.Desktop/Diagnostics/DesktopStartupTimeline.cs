using System.Diagnostics;
using System.IO;

namespace Nexus.Desktop.Diagnostics;

public sealed class DesktopStartupTimeline
{
    private readonly Stopwatch stopwatch = Stopwatch.StartNew();
    private readonly object syncRoot = new();
    private readonly List<DesktopStartupEvent> events = [];
    private readonly string logPath = ResolveLogPath();
    private long lastMilliseconds;

    public void Mark(string name, IReadOnlyDictionary<string, string>? metadata = null)
    {
        long elapsed = stopwatch.ElapsedMilliseconds;
        long delta;
        Dictionary<string, string> eventMetadata = metadata is null
            ? []
            : new Dictionary<string, string>(metadata);
        lock (syncRoot)
        {
            delta = elapsed - lastMilliseconds;
            lastMilliseconds = elapsed;
            events.Add(new DesktopStartupEvent(name, elapsed, delta, eventMetadata));
        }

        string suffix = eventMetadata.Count == 0
            ? string.Empty
            : " " + string.Join(" ", eventMetadata.OrderBy(item => item.Key).Select(item => $"{item.Key}={item.Value}"));
        string line = $"[Nexus Startup] event={name} elapsed_ms={elapsed} delta_ms={delta}{suffix}";
        Trace.WriteLine(line);
        AppendLine(line);
    }

    public IReadOnlyList<DesktopStartupEvent> Snapshot()
    {
        lock (syncRoot)
        {
            return events
                .Select(item => item with { Metadata = new Dictionary<string, string>(item.Metadata) })
                .ToList();
        }
    }

    private void AppendLine(string line)
    {
        try
        {
            lock (syncRoot)
            {
                Directory.CreateDirectory(Path.GetDirectoryName(logPath)!);
                File.AppendAllText(logPath, $"{DateTimeOffset.Now:O} {line}{Environment.NewLine}");
            }
        }
        catch (Exception exception) when (exception is IOException or UnauthorizedAccessException)
        {
        }
    }

    private static string ResolveLogPath()
    {
        return Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
            "Nexus",
            "Logs",
            "shell.log");
    }
}

public sealed record DesktopStartupEvent(
    string Name,
    long ElapsedMilliseconds,
    long DeltaMilliseconds,
    IReadOnlyDictionary<string, string> Metadata);
