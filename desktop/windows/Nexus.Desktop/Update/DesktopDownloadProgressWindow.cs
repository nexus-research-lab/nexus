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
        Title = "下载更新";
        Owner = owner;
        Width = 420;
        Height = 150;
        ResizeMode = ResizeMode.NoResize;
        ShowInTaskbar = false;
        WindowStartupLocation = WindowStartupLocation.CenterOwner;
        HorizontalContentAlignment = System.Windows.HorizontalAlignment.Stretch;
        VerticalContentAlignment = System.Windows.VerticalAlignment.Center;

        progressBar = new WpfProgressBar
        {
            Height = 6,
            Minimum = 0,
            Maximum = 1,
            IsIndeterminate = true,
        };
        progressText = new TextBlock
        {
            Text = "准备中…",
            FontSize = 12,
            HorizontalAlignment = System.Windows.HorizontalAlignment.Right,
            VerticalAlignment = System.Windows.VerticalAlignment.Center,
        };

        var header = new Grid
        {
            Margin = new Thickness(0, 0, 0, 12),
        };
        header.ColumnDefinitions.Add(new ColumnDefinition());
        header.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });

        var title = new TextBlock
        {
            Text = $"正在下载 Nexus {release.Version}",
            FontSize = 15,
            FontWeight = FontWeights.SemiBold,
            TextTrimming = TextTrimming.CharacterEllipsis,
            VerticalAlignment = System.Windows.VerticalAlignment.Center,
        };
        progressText.Margin = new Thickness(16, 0, 0, 0);
        Grid.SetColumn(progressText, 1);
        header.Children.Add(title);
        header.Children.Add(progressText);

        Content = new StackPanel
        {
            Margin = new Thickness(20),
            Children =
            {
                header,
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
