using System.Net;
using System.Net.Sockets;

namespace Nexus.Desktop.Sidecar;

internal static class SidecarPortAllocator
{
    private const int DesktopLoopbackPort = 34343;

    public static int Allocate()
    {
        if (IsAvailable(DesktopLoopbackPort))
        {
            return DesktopLoopbackPort;
        }

        throw new InvalidOperationException($"Nexus 桌面端本地端口 {DesktopLoopbackPort} 已被占用，请关闭占用该端口的进程后重试。");
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
