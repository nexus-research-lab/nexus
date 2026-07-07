using System.Net;
using System.Net.Sockets;
using Nexus.Desktop.Diagnostics;

namespace Nexus.Desktop.Sidecar;

internal static class SidecarPortAllocator
{
    private const int DesktopLoopbackPort = 34343;
    private const int PortRetryCount = 100;
    private const int PortRetryIntervalMilliseconds = 200;

    public static int Allocate(DesktopStartupTimeline? startupTimeline = null)
    {
        if (IsAvailable(DesktopLoopbackPort))
        {
            return DesktopLoopbackPort;
        }

        startupTimeline?.Mark("sidecar.port_wait_begin", new Dictionary<string, string>
        {
            ["port"] = DesktopLoopbackPort.ToString(),
        });
        // 旧 sidecar 正在响应退出信号时，给固定本地端口一个短暂释放窗口。
        for (int attempt = 1; attempt <= PortRetryCount; attempt++)
        {
            Thread.Sleep(PortRetryIntervalMilliseconds);
            if (IsAvailable(DesktopLoopbackPort))
            {
                startupTimeline?.Mark("sidecar.port_wait_ready", new Dictionary<string, string>
                {
                    ["attempt"] = attempt.ToString(),
                    ["port"] = DesktopLoopbackPort.ToString(),
                });
                return DesktopLoopbackPort;
            }
        }

        startupTimeline?.Mark("sidecar.port_unavailable", new Dictionary<string, string>
        {
            ["port"] = DesktopLoopbackPort.ToString(),
        });
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
