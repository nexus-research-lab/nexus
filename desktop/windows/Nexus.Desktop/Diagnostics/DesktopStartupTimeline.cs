using System.Diagnostics;

namespace Nexus.Desktop.Diagnostics;

public sealed class DesktopStartupTimeline
{
    private readonly Stopwatch stopwatch = Stopwatch.StartNew();
    private readonly object syncRoot = new();
    private readonly string logPath = ResolveLogPath();
    private long lastMilliseconds;

    public void Mark(string name, IReadOnlyDictionary<string, string>? metadata = null)
    {
        long elapsed = stopwatch.ElapsedMilliseconds;
        long delta;
        lock (syncRoot)
        {
            delta = elapsed - lastMilliseconds;
            lastMilliseconds = elapsed;
        }

        string suffix = metadata is null || metadata.Count == 0
            ? string.Empty
            : " " + string.Join(" ", metadata.OrderBy(item => item.Key).Select(item => $"{item.Key}={item.Value}"));
        string line = $"[Nexus Startup] event={name} elapsed_ms={elapsed} delta_ms={delta}{suffix}";
        Trace.WriteLine(line);
        AppendLine(line);
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
            Environment.GetFolderPath(Environment.SpecialFolder.UserProfile),
            ".nexus",
            "logs",
            "shell.log");
    }
}
