using System.Security.Cryptography;

namespace Nexus.Desktop.Sidecar;

internal static class DesktopSessionToken
{
    public static string Generate()
    {
        byte[] bytes = RandomNumberGenerator.GetBytes(32);
        return Convert.ToHexString(bytes).ToLowerInvariant();
    }
}
