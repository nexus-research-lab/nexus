// INPUT: Nexus light theme 的稳定语义色。
// OUTPUT: Windows 原生 chrome、菜单与对话框共用的冻结画刷。
// POS: Web token 到 WPF 控件的唯一颜色投影，不承载组件布局。

using System.Windows.Media;

namespace Nexus.Desktop.Theme;

public static class NexusNativeTheme
{
    public static SolidColorBrush CanvasBrush { get; } = CreateBrush(0xFF, 0xF9, 0xF9, 0xF7);
    public static SolidColorBrush OverlayBrush { get; } = CreateBrush(0xFF, 0xFF, 0xFF, 0xFF);
    public static SolidColorBrush OverlayBorderBrush { get; } = CreateBrush(0x1C, 0x0B, 0x0B, 0x0B);
    public static SolidColorBrush DividerSubtleBrush { get; } = CreateBrush(0x14, 0x0B, 0x0B, 0x0B);
    public static SolidColorBrush TextStrongBrush { get; } = CreateBrush(0xFF, 0x13, 0x13, 0x13);
    public static SolidColorBrush TextDefaultBrush { get; } = CreateBrush(0xFF, 0x38, 0x38, 0x35);
    public static SolidColorBrush TextMutedBrush { get; } = CreateBrush(0xFF, 0x5F, 0x5E, 0x5A);
    public static SolidColorBrush IconDefaultBrush { get; } = CreateBrush(0xFF, 0x52, 0x51, 0x4E);
    public static SolidColorBrush InteractiveHoverBrush { get; } = CreateBrush(0xFF, 0xF3, 0xF3, 0xF0);
    public static SolidColorBrush InteractiveActiveBrush { get; } = CreateBrush(0xFF, 0xF0, 0xEF, 0xEC);
    public static SolidColorBrush PrimaryBrush { get; } = CreateBrush(0xFF, 0x5B, 0x72, 0xFF);
    public static SolidColorBrush FocusBrush { get; } = CreateBrush(0x3D, 0x5B, 0x72, 0xFF);
    public static SolidColorBrush BrandActionBrush { get; } = CreateBrush(0xFF, 0x52, 0x66, 0xE6);
    public static SolidColorBrush BrandActionHoverBrush { get; } = CreateBrush(0xFF, 0x48, 0x59, 0xCF);
    public static SolidColorBrush DestructiveBrush { get; } = CreateBrush(0xFF, 0xDF, 0x5D, 0x62);
    public static System.Windows.Media.Color ShadowColor { get; } =
        System.Windows.Media.Color.FromRgb(0x0B, 0x0B, 0x0B);

    private static SolidColorBrush CreateBrush(byte alpha, byte red, byte green, byte blue)
    {
        var brush = new SolidColorBrush(
            System.Windows.Media.Color.FromArgb(alpha, red, green, blue));
        brush.Freeze();
        return brush;
    }
}
