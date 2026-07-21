using System.Diagnostics;
using System.Text.RegularExpressions;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Documents;
using System.Windows.Media;
using System.Windows.Navigation;

namespace Nexus.Desktop.Update;

internal static class DesktopReleaseNotesRenderer
{
    private static readonly Regex FencePattern = new(
        @"^\s*(```|~~~)(.*)$",
        RegexOptions.Compiled);
    private static readonly Regex HeadingPattern = new(
        @"^\s{0,3}(#{1,6})\s+(.+?)\s*#*\s*$",
        RegexOptions.Compiled);
    private static readonly Regex BulletPattern = new(
        @"^\s*[-*+]\s+(.+)$",
        RegexOptions.Compiled);
    private static readonly Regex OrderedPattern = new(
        @"^\s*(\d+)\.\s+(.+)$",
        RegexOptions.Compiled);
    private static readonly Regex InlinePattern = new(
        @"(`[^`]+`)|(" +
        @"\*\*.+?\*\*)|(__.+?__)|(" +
        @"\*.+?\*)|(_.+?_)|(" +
        @"\[[^\]]+\]\([^)]+\))",
        RegexOptions.Compiled);

    // 更新说明来自远端 Release，只做本地排版，不执行 HTML 或脚本。
    public static FlowDocument Render(string markdown)
    {
        string normalized = markdown
            .Replace("\r\n", "\n", StringComparison.Ordinal)
            .Replace('\r', '\n')
            .Trim();
        string[] lines = normalized.Split('\n');
        var document = new FlowDocument
        {
            PagePadding = new Thickness(8, 6, 8, 6),
            ColumnWidth = double.PositiveInfinity,
            FontFamily = new FontFamily("Segoe UI"),
            FontSize = 12,
            Foreground = SystemColors.ControlTextBrush,
        };

        for (int index = 0; index < lines.Length;)
        {
            string line = lines[index];
            string trimmed = line.Trim();
            if (trimmed.Length == 0)
            {
                index++;
                continue;
            }

            Match fence = FencePattern.Match(trimmed);
            if (fence.Success)
            {
                string marker = fence.Groups[1].Value;
                index++;
                var codeLines = new List<string>();
                while (index < lines.Length)
                {
                    if (lines[index].TrimStart().StartsWith(marker, StringComparison.Ordinal))
                    {
                        index++;
                        break;
                    }
                    codeLines.Add(lines[index]);
                    index++;
                }
                AddCodeBlock(document, string.Join(Environment.NewLine, codeLines));
                continue;
            }

            Match heading = HeadingPattern.Match(trimmed);
            if (heading.Success)
            {
                string text = heading.Groups[2].Value.Trim().TrimEnd('#').Trim();
                AddParagraph(
                    document,
                    text,
                    fontSize: HeadingFontSize(heading.Groups[1].Value.Length),
                    fontWeight: FontWeights.SemiBold,
                    paragraphSpacing: 10);
                index++;
                continue;
            }

            Match bullet = BulletPattern.Match(trimmed);
            if (bullet.Success)
            {
                AddParagraph(
                    document,
                    bullet.Groups[1].Value,
                    prefix: "• ",
                    margin: new Thickness(16, 2, 0, 2),
                    textIndent: -12);
                index++;
                continue;
            }

            Match ordered = OrderedPattern.Match(trimmed);
            if (ordered.Success)
            {
                AddParagraph(
                    document,
                    ordered.Groups[2].Value,
                    prefix: $"{ordered.Groups[1].Value}. ",
                    margin: new Thickness(20, 2, 0, 2),
                    textIndent: -16);
                index++;
                continue;
            }

            if (trimmed.StartsWith(">", StringComparison.Ordinal))
            {
                AddParagraph(
                    document,
                    trimmed[1..].Trim(),
                    prefix: "│ ",
                    foreground: SystemColors.GrayTextBrush,
                    margin: new Thickness(16, 2, 0, 6),
                    textIndent: -12);
                index++;
                continue;
            }

            if (IsHorizontalRule(trimmed))
            {
                AddParagraph(
                    document,
                    "────────",
                    foreground: SystemColors.GrayTextBrush,
                    paragraphSpacing: 6);
                index++;
                continue;
            }

            var paragraphLines = new List<string>();
            while (index < lines.Length)
            {
                string paragraphLine = lines[index];
                string paragraphTrimmed = paragraphLine.Trim();
                if (paragraphTrimmed.Length == 0 || IsBlockStart(paragraphTrimmed))
                {
                    break;
                }
                paragraphLines.Add(paragraphTrimmed);
                index++;
            }

            if (paragraphLines.Count == 0)
            {
                index++;
                continue;
            }
            AddParagraph(document, string.Join(" ", paragraphLines));
        }

        return document;
    }

    private static void AddParagraph(
        FlowDocument document,
        string text,
        string? prefix = null,
        double fontSize = 12,
        FontWeight? fontWeight = null,
        Brush? foreground = null,
        Thickness? margin = null,
        double textIndent = 0,
        double paragraphSpacing = 8)
    {
        var paragraph = new Paragraph
        {
            FontSize = fontSize,
            FontWeight = fontWeight ?? FontWeights.Normal,
            Foreground = foreground ?? SystemColors.ControlTextBrush,
            Margin = margin ?? new Thickness(0, 2, 0, paragraphSpacing),
            TextIndent = textIndent,
            LineHeight = 18,
        };
        if (!string.IsNullOrEmpty(prefix))
        {
            paragraph.Inlines.Add(new Run(prefix));
        }
        AddInline(paragraph, text);
        document.Blocks.Add(paragraph);
    }

    private static void AddCodeBlock(FlowDocument document, string text)
    {
        var paragraph = new Paragraph
        {
            FontFamily = new FontFamily("Consolas"),
            FontSize = 11,
            Background = SystemColors.ControlBrush,
            Padding = new Thickness(8),
            Margin = new Thickness(0, 4, 0, 8),
            LineHeight = 16,
        };
        paragraph.Inlines.Add(new Run(text));
        document.Blocks.Add(paragraph);
    }

    private static void AddInline(Paragraph paragraph, string text)
    {
        int cursor = 0;
        foreach (Match match in InlinePattern.Matches(text))
        {
            if (match.Index > cursor)
            {
                paragraph.Inlines.Add(new Run(text[cursor..match.Index]));
            }

            string token = match.Value;
            if (token.StartsWith('`') && token.EndsWith('`'))
            {
                paragraph.Inlines.Add(new Run(token[1..^1])
                {
                    FontFamily = new FontFamily("Consolas"),
                    FontSize = 11,
                    Background = SystemColors.ControlBrush,
                });
            }
            else if (token.StartsWith("**", StringComparison.Ordinal) && token.EndsWith("**", StringComparison.Ordinal) ||
                     token.StartsWith("__", StringComparison.Ordinal) && token.EndsWith("__", StringComparison.Ordinal))
            {
                paragraph.Inlines.Add(new Bold(new Run(token[2..^2])));
            }
            else if (token.Length > 2 &&
                     (token.StartsWith('*') || token.StartsWith('_')) &&
                     token.EndsWith(token[0]))
            {
                paragraph.Inlines.Add(new Italic(new Run(token[1..^1])));
            }
            else if (TryParseLink(token, out string label, out Uri? uri))
            {
                var hyperlink = new Hyperlink(new Run(label))
                {
                    NavigateUri = uri!,
                };
                hyperlink.RequestNavigate += HandleLinkRequest;
                paragraph.Inlines.Add(hyperlink);
            }
            else
            {
                paragraph.Inlines.Add(new Run(token));
            }
            cursor = match.Index + match.Length;
        }

        if (cursor < text.Length)
        {
            paragraph.Inlines.Add(new Run(text[cursor..]));
        }
    }

    private static void HandleLinkRequest(object sender, RequestNavigateEventArgs args)
    {
        if (args.Uri.Scheme is "http" or "https")
        {
            Process.Start(new ProcessStartInfo(args.Uri.ToString())
            {
                UseShellExecute = true,
            });
        }
        args.Handled = true;
    }

    private static bool TryParseLink(string token, out string label, out Uri? uri)
    {
        label = string.Empty;
        uri = null;
        int separator = token.IndexOf("](", StringComparison.Ordinal);
        if (!token.StartsWith('[') || separator < 0 || !token.EndsWith(')'))
        {
            return false;
        }

        label = token[1..separator];
        string target = token[(separator + 2)..^1];
        return Uri.TryCreate(target, UriKind.Absolute, out uri) &&
               uri is not null &&
               uri.Scheme is "http" or "https";
    }

    private static bool IsBlockStart(string line) =>
        FencePattern.IsMatch(line) ||
        HeadingPattern.IsMatch(line) ||
        BulletPattern.IsMatch(line) ||
        OrderedPattern.IsMatch(line) ||
        line.StartsWith(">", StringComparison.Ordinal) ||
        IsHorizontalRule(line);

    private static bool IsHorizontalRule(string line)
    {
        string compact = line.Replace(" ", string.Empty, StringComparison.Ordinal);
        return compact.Length >= 3 &&
               compact.All(character => character is '-' or '_' or '*');
    }

    private static double HeadingFontSize(int level) => level switch
    {
        1 => 18,
        2 => 16,
        3 => 14,
        _ => 13,
    };
}
