// INPUT: Durable sidecar identity and a live process observed through a held process handle.
// OUTPUT: Exact identity comparison independent of the new package's installation directory.
// POS: Recovery ownership receipt; incomplete legacy records do not authorize termination.
using System.IO;

namespace Nexus.Desktop.Sidecar;

internal sealed record SidecarProcessRecord(int Pid, string ExecutablePath, long StartTimeUtcTicks = 0, string? StateRoot = null)
{
    public bool Matches(int pid, long startTimeUtcTicks, string? executablePath, string stateRoot) =>
        Pid > 0 && Pid == pid && StartTimeUtcTicks > 0 && StartTimeUtcTicks == startTimeUtcTicks &&
        SamePath(ExecutablePath, executablePath) && SamePath(StateRoot, stateRoot);

    public static bool SamePath(string? left, string? right)
    {
        if (string.IsNullOrWhiteSpace(left) || string.IsNullOrWhiteSpace(right) ||
            !Path.IsPathFullyQualified(left) || !Path.IsPathFullyQualified(right)) return false;
        try { return string.Equals(Path.GetFullPath(left), Path.GetFullPath(right), StringComparison.OrdinalIgnoreCase); }
        catch (Exception error) when (error is ArgumentException or NotSupportedException or PathTooLongException) { return false; }
    }
}
