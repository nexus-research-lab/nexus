/**
 * INPUT: 以 Slash 指令开头的普通消息文本。
 * OUTPUT: 可复用的命令前缀投影和 Markdown 内部链接。
 * POS: Composer 与用户消息共用的 Slash 指令纯投影；不改变原始文本。
 */
export const SLASH_COMMAND_HREF_PREFIX = "#nexus-slash-command=";

const LEADING_SLASH_COMMAND_PATTERN = /^\/[^\s/]+(?=$|\s)/u;

export interface SlashCommandPresentation {
  command: string;
  remainder: string;
}

export function projectLeadingSlashCommand(
  content: string,
): SlashCommandPresentation | null {
  const match = LEADING_SLASH_COMMAND_PATTERN.exec(content);
  if (!match) {
    return null;
  }
  return {
    command: match[0],
    remainder: content.slice(match[0].length),
  };
}

export function decorateLeadingSlashCommand(content: string): string {
  const presentation = projectLeadingSlashCommand(content);
  if (!presentation) {
    return content;
  }
  const label = presentation.command
    .replaceAll("\\", "\\\\")
    .replaceAll("[", "\\[")
    .replaceAll("]", "\\]");
  const name = encodeURIComponent(presentation.command.slice(1));
  return `[${label}](${SLASH_COMMAND_HREF_PREFIX}${name})${presentation.remainder}`;
}

export function isSlashCommandHref(href: string): boolean {
  if (!href.startsWith(SLASH_COMMAND_HREF_PREFIX)) {
    return false;
  }
  try {
    return Boolean(decodeURIComponent(
      href.slice(SLASH_COMMAND_HREF_PREFIX.length),
    ).trim());
  } catch {
    return false;
  }
}
