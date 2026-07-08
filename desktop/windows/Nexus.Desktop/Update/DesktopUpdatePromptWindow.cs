using System.Windows;
using System.Windows.Controls;
using System.Windows.Input;

namespace Nexus.Desktop.Update;

internal sealed class DesktopUpdatePromptWindow : Window
{
    private const double WindowWidth = 640;
    private const double ContentMaxHeight = 300;
    private const double OuterPadding = 24;
    private const double SectionSpacing = 16;

    public DesktopUpdatePromptWindow(
        string message,
        string? releaseNotes,
        bool canDownloadInstaller)
    {
        PromptAction = UpdatePromptAction.Later;
        Title = "发现 Nexus 新版本";
        Width = WindowWidth;
        SizeToContent = SizeToContent.Height;
        MaxHeight = 640;
        ResizeMode = ResizeMode.NoResize;
        ShowInTaskbar = false;
        WindowStartupLocation = WindowStartupLocation.CenterOwner;

        var root = new Grid
        {
            Margin = new Thickness(OuterPadding),
        };
        root.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });
        root.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });
        root.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });

        var messageBlock = new TextBlock
        {
            Text = message,
            TextWrapping = TextWrapping.Wrap,
            FontSize = 14,
            LineHeight = 21,
        };
        Grid.SetRow(messageBlock, 0);
        root.Children.Add(messageBlock);

        UIElement? releaseNotesElement = CreateReleaseNotesElement(releaseNotes);
        if (releaseNotesElement is not null)
        {
            Grid.SetRow(releaseNotesElement, 1);
            root.Children.Add(releaseNotesElement);
        }

        var buttons = CreateButtonBar(canDownloadInstaller);
        Grid.SetRow(buttons, 2);
        root.Children.Add(buttons);

        Content = root;
        PreviewKeyDown += HandlePreviewKeyDown;
    }

    public UpdatePromptAction PromptAction { get; private set; }

    private static UIElement? CreateReleaseNotesElement(string? releaseNotes)
    {
        if (string.IsNullOrWhiteSpace(releaseNotes))
        {
            return null;
        }

        var panel = new StackPanel
        {
            Margin = new Thickness(0, SectionSpacing, 0, 0),
        };
        panel.Children.Add(new TextBlock
        {
            Text = "更新内容",
            FontSize = 13,
            FontWeight = FontWeights.SemiBold,
            Margin = new Thickness(0, 0, 0, 6),
        });

        // Windows MessageBox 不支持滚动正文，release notes 必须被限制在固定区域内。
        panel.Children.Add(new TextBox
        {
            Text = releaseNotes,
            IsReadOnly = true,
            IsTabStop = false,
            TextWrapping = TextWrapping.Wrap,
            AcceptsReturn = true,
            VerticalScrollBarVisibility = ScrollBarVisibility.Auto,
            HorizontalScrollBarVisibility = ScrollBarVisibility.Disabled,
            MaxHeight = ContentMaxHeight,
            Padding = new Thickness(8),
            BorderBrush = SystemColors.ControlDarkBrush,
            BorderThickness = new Thickness(1),
            Background = SystemColors.ControlBrush,
            Foreground = SystemColors.ControlTextBrush,
            FontSize = 12,
        });
        return panel;
    }

    private StackPanel CreateButtonBar(bool canDownloadInstaller)
    {
        var buttons = new StackPanel
        {
            Orientation = Orientation.Horizontal,
            HorizontalAlignment = HorizontalAlignment.Right,
            Margin = new Thickness(0, SectionSpacing, 0, 0),
        };

        if (canDownloadInstaller)
        {
            buttons.Children.Add(CreateButton(
                "下载并更新",
                true,
                () => CloseWith(UpdatePromptAction.DownloadAndInstall)));
            buttons.Children.Add(CreateButton(
                "打开下载页",
                false,
                () => CloseWith(UpdatePromptAction.OpenReleasePage)));
        }
        else
        {
            buttons.Children.Add(CreateButton(
                "打开下载页",
                true,
                () => CloseWith(UpdatePromptAction.OpenReleasePage)));
        }
        buttons.Children.Add(CreateButton(
            "稍后",
            false,
            () => CloseWith(UpdatePromptAction.Later),
            isCancel: true));
        return buttons;
    }

    private static Button CreateButton(
        string label,
        bool isDefault,
        System.Action action,
        bool isCancel = false)
    {
        var button = new Button
        {
            Content = label,
            IsDefault = isDefault,
            IsCancel = isCancel,
            MinWidth = 96,
            Padding = new Thickness(12, 4, 12, 4),
            Margin = new Thickness(8, 0, 0, 0),
        };
        button.Click += (_, _) => action();
        return button;
    }

    private void CloseWith(UpdatePromptAction action)
    {
        PromptAction = action;
        Close();
    }

    private void HandlePreviewKeyDown(object sender, KeyEventArgs e)
    {
        if (e.Key != Key.Escape)
        {
            return;
        }

        PromptAction = UpdatePromptAction.Later;
        Close();
        e.Handled = true;
    }
}
