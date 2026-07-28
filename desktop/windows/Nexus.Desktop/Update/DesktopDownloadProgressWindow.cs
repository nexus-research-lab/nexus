// INPUT: 更新版本与下载字节进度。
// OUTPUT: 非阻塞的原生下载进度窗口。
// POS: Windows 更新下载阶段的唯一可视反馈。

using System.Windows;
using System.Windows.Controls;
using WpfProgressBar = System.Windows.Controls.ProgressBar;

namespace Nexus.Desktop.Update;

internal sealed class DesktopDownloadProgressWindow : System.Windows.Window
{
    private readonly WpfProgressBar progressBar;
    private readonly TextBlock progressText;

    internal DesktopDownloadProgressWindow(System.Windows.Window owner, DesktopReleaseInfo release)
    {
        Title = "Nexus 正在下载更新";
        Owner = owner;
        Width = 520;
        Height = 230;
        ResizeMode = ResizeMode.NoResize;
        ShowInTaskbar = false;
        WindowStartupLocation = WindowStartupLocation.CenterOwner;

        progressBar = new WpfProgressBar
        {
            Height = 8,
            Minimum = 0,
            Maximum = 1,
            IsIndeterminate = true,
        };
        progressText = new TextBlock
        {
            Text = "正在连接下载服务…",
            HorizontalAlignment = System.Windows.HorizontalAlignment.Right,
        };

        Content = new StackPanel
        {
            Margin = new Thickness(28),
            Children =
            {
                new TextBlock
                {
                    Text = $"正在下载 Nexus {release.DisplayText}",
                    FontSize = 20,
                    FontWeight = FontWeights.SemiBold,
                    Margin = new Thickness(0, 0, 0, 28),
                },
                progressText,
                progressBar,
            },
        };
    }

    internal void Report(long receivedBytes, long? totalBytes)
    {
        Dispatcher.Invoke(() =>
        {
            progressText.Text = totalBytes is > 0
                ? $"{FormatBytes(receivedBytes)} / {FormatBytes(totalBytes.Value)}"
                : FormatBytes(receivedBytes);
            progressBar.IsIndeterminate = totalBytes is not > 0;
            if (totalBytes is > 0)
            {
                progressBar.Value = Math.Min(1, (double)receivedBytes / totalBytes.Value);
            }
        });
    }

    private static string FormatBytes(long bytes) =>
        $"{bytes / 1024d / 1024d:0.0} MB";
}
