/**
 * INPUT: 会话内部链接、Agent 目录与联系人打开命令。
 * OUTPUT: 受控 Markdown 链接渲染槽和仅放行有效 mention 的 URL 转换。
 * POS: Conversation 对 Agent/handoff 与 Slash URI 的唯一渲染解释，普通链接交还共享层。
 */
import type { ReactNode } from "react";
import { defaultUrlTransform } from "react-markdown";

import { isSlashCommandHref } from "../slash-command-presentation";
import { SlashCommandToken } from "../slash-command-token";
import { AgentMentionChip, type AgentMentionDirectory } from "./agent-mention-chip";

export function createMessageMarkdownLinkRenderer(
  directory?: AgentMentionDirectory,
  onOpenAgentContact?: (agentId: string) => void,
) {
  return (href: string, children: ReactNode): ReactNode | null => {
    if (isSlashCommandHref(href)) {
      return <SlashCommandToken>{children}</SlashCommandToken>;
    }
    const mention = parseAgentMentionHref(href);
    if (!mention) {
      return null;
    }
    return (
      <AgentMentionChip
        agentId={mention.agentId}
        directory={directory}
        handoffId={mention.handoffId}
        onOpenAgentContact={onOpenAgentContact}
      >
        {children}
      </AgentMentionChip>
    );
  };
}

export function transformMessageMarkdownUrl(value: string): string {
  return parseAgentMentionHref(value) ? value : defaultUrlTransform(value);
}

function parseAgentMentionHref(
  href: string,
): { agentId: string; handoffId?: string } | null {
  const prefix = "agent-mention://";
  if (!href.startsWith(prefix)) {
    return null;
  }
  const target = href.slice(prefix.length);
  const queryIndex = target.indexOf("?");
  const encodedAgentId = queryIndex >= 0 ? target.slice(0, queryIndex) : target;
  const query = queryIndex >= 0 ? target.slice(queryIndex + 1) : "";
  try {
    const agentId = decodeURIComponent(encodedAgentId).trim();
    if (!agentId) {
      return null;
    }
    const handoffId = new URLSearchParams(query).get("handoff_id")?.trim();
    return { agentId, ...(handoffId ? { handoffId } : {}) };
  } catch {
    return null;
  }
}
