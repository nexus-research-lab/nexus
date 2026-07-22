using System.Windows;
using System.Windows.Controls;
using WpfButton = System.Windows.Controls.Button;
using WpfKey = System.Windows.Input.Key;
using WpfKeyEventArgs = System.Windows.Input.KeyEventArgs;
using WpfHorizontalAlignment = System.Windows.HorizontalAlignment;
using WpfOrientation = System.Windows.Controls.Orientation;
using WpfRichTextBox = System.Windows.Controls.RichTextBox;
using WpfSystemColors = System.Windows.SystemColors;
using WpfWindow = System.Windows.Window;

namespace Nexus.Desktop.Update;

internal sealed class DesktopUpdatePromptWindow : WpfWindow
{
    private const double WindowWidth = 640;
    private const double WindowMinHeight = 320;
    private const double WindowMaxHeight = 640;
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
        Height = Math.Min(
            WindowMaxHeight,
            Math.Max(WindowMinHeight, SystemParameters.WorkArea.Height - 80));
        SizeToContent = SizeToContent.Manual;
        MinHeight = WindowMinHeight;
        MaxHeight = WindowMaxHeight;
        ResizeMode = ResizeMode.NoResize;
        ShowInTaskbar = false;
        WindowStartupLocation = WindowStartupLocation.CenterOwner;

        var root = new Grid
        {
            Margin = new Thickness(OuterPadding),
        };
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
            root.RowDefinitions.Add(new RowDefinition { Height = new GridLength(1, GridUnitType.Star) });
            Grid.SetRow(releaseNotesElement, 1);
            root.Children.Add(releaseNotesElement);
        }

        root.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });
        var buttons = CreateButtonBar(canDownloadInstaller);
        Grid.SetRow(buttons, releaseNotesElement is null ? 1 : 2);
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

        var panel = new Grid
        {
            Margin = new Thickness(0, SectionSpacing, 0, 0),
        };
        panel.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });
        panel.RowDefinitions.Add(new RowDefinition { Height = new GridLength(1, GridUnitType.Star) });
        panel.Children.Add(new TextBlock
        {
            Text = "更新内容",
            FontSize = 13,
            FontWeight = FontWeights.SemiBold,
            Margin = new Thickness(0, 0, 0, 6),
        });

        var releaseNotesView = new WpfRichTextBox
        {
            Document = DesktopReleaseNotesRenderer.Render(releaseNotes),
            IsReadOnly = true,
            IsTabStop = false,
            VerticalAlignment = VerticalAlignment.Stretch,
            HorizontalAlignment = WpfHorizontalAlignment.Stretch,
            VerticalContentAlignment = VerticalAlignment.Stretch,
            HorizontalContentAlignment = WpfHorizontalAlignment.Stretch,
            MaxHeight = ContentMaxHeight,
            MinHeight = 80,
            Padding = new Thickness(8),
            BorderBrush = WpfSystemColors.ControlDarkBrush,
            BorderThickness = new Thickness(1),
            Background = WpfSystemColors.ControlBrush,
            Foreground = WpfSystemColors.ControlTextBrush,
            FontSize = 12,
        };
        ScrollViewer.SetVerticalScrollBarVisibility(releaseNotesView, ScrollBarVisibility.Auto);
        ScrollViewer.SetHorizontalScrollBarVisibility(releaseNotesView, ScrollBarVisibility.Disabled);
        Grid.SetRow(releaseNotesView, 1);
        panel.Children.Add(releaseNotesView);
        return panel;
    }

    private StackPanel CreateButtonBar(bool canDownloadInstaller)
    {
        var buttons = new StackPanel
        {
            Orientation = WpfOrientation.Horizontal,
            HorizontalAlignment = WpfHorizontalAlignment.Right,
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

    private static WpfButton CreateButton(
        string label,
        bool isDefault,
        System.Action action,
        bool isCancel = false)
    {
        var button = new WpfButton
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

    private void HandlePreviewKeyDown(object sender, WpfKeyEventArgs e)
    {
        if (e.Key != WpfKey.Escape)
        {
            return;
        }

        PromptAction = UpdatePromptAction.Later;
        Close();
        e.Handled = true;
    }
}
