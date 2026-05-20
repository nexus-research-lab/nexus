using System.IO;
using System.Text.Json;
using Nexus.Desktop.Sidecar;

namespace Nexus.Desktop.Bridge;

internal static class DesktopPersistentStateStore
{
    private static readonly object SyncRoot = new();
    private static readonly string StatePath = Path.Combine(DesktopPaths.ConfigDirectory, "desktop-state.json");

    public static string? Get(string key)
    {
        string normalizedKey = NormalizeKey(key);
        lock (SyncRoot)
        {
            return ReadAll().TryGetValue(normalizedKey, out string? value) ? value : null;
        }
    }

    public static void Set(string key, string value)
    {
        string normalizedKey = NormalizeKey(key);
        lock (SyncRoot)
        {
            Dictionary<string, string> values = ReadAll();
            values[normalizedKey] = value;
            WriteAll(values);
        }
    }

    public static void Remove(string key)
    {
        string normalizedKey = NormalizeKey(key);
        lock (SyncRoot)
        {
            Dictionary<string, string> values = ReadAll();
            values.Remove(normalizedKey);
            WriteAll(values);
        }
    }

    private static string NormalizeKey(string key)
    {
        string normalized = key.Trim();
        if (string.IsNullOrWhiteSpace(normalized) ||
            normalized.Length > 128 ||
            normalized.Any(character => !(char.IsLetterOrDigit(character) || character is '.' or '_' or '-')))
        {
            throw new ArgumentException("Persistent state key is invalid.");
        }

        return normalized;
    }

    private static Dictionary<string, string> ReadAll()
    {
        try
        {
            if (!File.Exists(StatePath))
            {
                return new Dictionary<string, string>();
            }

            string raw = File.ReadAllText(StatePath);
            Dictionary<string, string>? values = JsonSerializer.Deserialize<Dictionary<string, string>>(raw);
            return values ?? new Dictionary<string, string>();
        }
        catch
        {
            return new Dictionary<string, string>();
        }
    }

    private static void WriteAll(Dictionary<string, string> values)
    {
        Directory.CreateDirectory(DesktopPaths.ConfigDirectory);
        string tempPath = $"{StatePath}.tmp";
        File.WriteAllText(tempPath, JsonSerializer.Serialize(values));
        if (File.Exists(StatePath))
        {
            File.Replace(tempPath, StatePath, null);
            return;
        }

        File.Move(tempPath, StatePath);
    }
}
