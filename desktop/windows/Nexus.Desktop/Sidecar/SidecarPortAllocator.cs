using System.Net;
using System.Net.Sockets;

namespace Nexus.Desktop.Sidecar;

internal static class SidecarPortAllocator
{
    public static int Allocate()
    {
        for (int attempt = 0; attempt < 80; attempt++)
        {
            int port = RandomNumberGeneratorCompat.Next(20000, 49152);
            if (IsAvailable(port))
            {
                return port;
            }
        }

        throw new InvalidOperationException("没有可用的本地端口。");
    }

    private static bool IsAvailable(int port)
    {
        try
        {
            using var listener = new TcpListener(IPAddress.Loopback, port);
            listener.Start();
            return true;
        }
        catch (SocketException)
        {
            return false;
        }
    }
}

internal static class RandomNumberGeneratorCompat
{
    public static int Next(int minValue, int maxValue)
    {
        return System.Security.Cryptography.RandomNumberGenerator.GetInt32(minValue, maxValue);
    }
}
