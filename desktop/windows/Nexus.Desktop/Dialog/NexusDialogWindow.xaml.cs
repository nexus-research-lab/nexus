// INPUT: 对话框内容模型与可选 owner。
// OUTPUT: 模态动作 ID，并统一关闭、Escape、默认按钮和焦点行为。
// POS: Windows 原生反馈的唯一窗口实现；调用方只声明内容与动作。

using System.Windows;
using System.Windows.Automation;
using System.Windows.Documents;
using System.Windows.Input;

namespace Nexus.Desktop.Dialog;

internal enum NexusDialogActionTone
{
    Secondary,
    Primary,
}

internal sealed record NexusDialogAction(
    string Id,
    string Label,
    NexusDialogActionTone Tone = NexusDialogActionTone.Secondary,
    bool IsDefault = false,
    bool IsCancel = false);

internal sealed record NexusDialogOptions(
    string Title,
    string Message,
    IReadOnlyList<NexusDialogAction> Actions,
    string? DetailsTitle = null,
    FlowDocument? Details = null,
    double ContentWidth = 440,
    double BodyMaxHeight = 420);

internal sealed partial class NexusDialogWindow : System.Windows.Window
{
    private const double ShadowInset = 34;
    private const double NonBodyHeight = 240;
    private const string ConfirmActionID = "confirm";
    private const string CloseActionID = "close";

    private static readonly IReadOnlyDictionary<NexusDialogActionTone, string> ButtonStyleKeys =
        new Dictionary<NexusDialogActionTone, string>
        {
            [NexusDialogActionTone.Secondary] = "NexusDialogSecondaryButtonStyle",
            [NexusDialogActionTone.Primary] = "NexusDialogPrimaryButtonStyle",
        };

    private readonly string? cancelActionID;

    private NexusDialogWindow(NexusDialogOptions options)
    {
        InitializeComponent();
        Title = options.Title;
        AutomationProperties.SetName(this, options.Title);
        Rect workArea = SystemParameters.WorkArea;
        Width = Math.Min(
            options.ContentWidth + (ShadowInset * 2),
            Math.Max(320, workArea.Width - 16));
        MaxHeight = Math.Max(320, workArea.Height - 16);
        BodyScroll.MaxHeight = Math.Max(
            120,
            Math.Min(options.BodyMaxHeight, MaxHeight - NonBodyHeight));
        TitleText.Text = options.Title;
        MessageText.Text = options.Message;
        MessageText.Visibility = string.IsNullOrWhiteSpace(options.Message)
            ? Visibility.Collapsed
            : Visibility.Visible;
        cancelActionID = options.Actions.FirstOrDefault(action => action.IsCancel)?.Id;
        SetDetails(options);
        SetActions(options.Actions);
    }

    internal string? ResultActionID { get; private set; }

    internal static string? Show(System.Windows.Window? owner, NexusDialogOptions options)
    {
        var dialog = new NexusDialogWindow(options);
        if (owner?.IsVisible == true)
        {
            dialog.Owner = owner;
        }
        else
        {
            dialog.WindowStartupLocation = WindowStartupLocation.CenterScreen;
        }
        dialog.ShowDialog();
        return dialog.ResultActionID;
    }

    internal static void ShowMessage(
        System.Windows.Window? owner,
        string title,
        string message,
        string actionLabel = "知道了")
    {
        Show(owner, new NexusDialogOptions(
            title,
            message,
            [
                new NexusDialogAction(
                    CloseActionID,
                    actionLabel,
                    NexusDialogActionTone.Primary,
                    IsDefault: true),
            ]));
    }

    internal static bool Confirm(
        System.Windows.Window owner,
        string title,
        string message,
        string confirmLabel,
        string cancelLabel = "取消")
    {
        string? actionID = Show(owner, new NexusDialogOptions(
            title,
            message,
            [
                new NexusDialogAction("cancel", cancelLabel, IsCancel: true),
                new NexusDialogAction(
                    ConfirmActionID,
                    confirmLabel,
                    NexusDialogActionTone.Primary,
                    IsDefault: true),
            ]));
        return actionID == ConfirmActionID;
    }

    protected override void OnPreviewKeyDown(System.Windows.Input.KeyEventArgs e)
    {
        if (e.Key == Key.Escape)
        {
            CloseWith(cancelActionID);
            e.Handled = true;
            return;
        }
        base.OnPreviewKeyDown(e);
    }

    protected override void OnClosing(System.ComponentModel.CancelEventArgs e)
    {
        ResultActionID ??= cancelActionID;
        base.OnClosing(e);
    }

    private void SetDetails(NexusDialogOptions options)
    {
        if (options.Details is null)
        {
            return;
        }

        DetailsTitleText.Text = options.DetailsTitle ?? "详情";
        DetailsView.Document = options.Details;
        DetailsPanel.Visibility = Visibility.Visible;
    }

    private void SetActions(IReadOnlyList<NexusDialogAction> actions)
    {
        foreach (NexusDialogAction action in actions)
        {
            var button = new System.Windows.Controls.Button
            {
                Content = action.Label,
                IsDefault = action.IsDefault,
                Style = (Style)FindResource(ButtonStyleKeys[action.Tone]),
            };
            AutomationProperties.SetName(button, action.Label);
            button.Click += (_, _) => CloseWith(action.Id);
            ActionPanel.Children.Add(button);
        }
    }

    private void CloseWith(string? actionID)
    {
        ResultActionID = actionID;
        Close();
    }

    private void CloseDialog(object sender, RoutedEventArgs e) => CloseWith(cancelActionID);

    private void DragDialog(object sender, MouseButtonEventArgs e)
    {
        if (e.LeftButton == MouseButtonState.Pressed)
        {
            DragMove();
        }
    }
}
