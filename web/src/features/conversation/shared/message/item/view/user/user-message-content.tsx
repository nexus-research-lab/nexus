import {
  useLayoutEffect,
  useRef,
  useState,
  type RefObject,
} from "react";
import { ChevronDown, Target } from "lucide-react";

import { useScrollAnchoredState } from "@/features/conversation/shared/timeline/scroll/use-scroll-anchored-state";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import type { MessageAttachment } from "@/types/conversation/message/attachment";
import type { AgentMention } from "@/types/conversation/message/entity";

import { ContentRenderer } from "../content/content-renderer";
import { MessageUserAttachments } from "./message-user-attachments";
import {
  isUserMessageContentCollapsible,
  USER_MESSAGE_COLLAPSED_HEIGHT,
  type UserMessagePresentation,
} from "./user-message-model";
import type { AgentMentionDirectory } from "../../../agent-mention-chip";

interface UserMessageContentProps {
  attachments: MessageAttachment[];
  agentMentions?: AgentMention[];
  agentMentionDirectory?: AgentMentionDirectory;
  content: string;
  onOpenAgentContact?: (agentId: string) => void;
  onOpenWorkspaceFile?: (path: string, workspaceAgentId?: string | null) => void;
  presentation: UserMessagePresentation;
  workspaceAgentId?: string | null;
}

export function UserMessageContent({
  attachments,
  agentMentions,
  agentMentionDirectory,
  content,
  onOpenAgentContact,
  onOpenWorkspaceFile,
  presentation,
  workspaceAgentId,
}: UserMessageContentProps) {
  const { t } = useI18n();
  const expansion = useScrollAnchoredState(false);
  const measuredContentRef = useRef<HTMLDivElement | null>(null);
  const [collapsible, setCollapsible] = useState(false);

  useLayoutEffect(() => {
    const element = measuredContentRef.current;
    if (!element) {
      return;
    }
    const measure = () => {
      setCollapsible(isUserMessageContentCollapsible(element.scrollHeight));
    };
    measure();
    if (typeof ResizeObserver === "undefined") {
      return;
    }
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    return () => observer.disconnect();
  }, [content]);

  return (
    <div
      className="nexus-chat-user-content-shell ml-auto flex w-fit max-w-full flex-col items-end rounded-[12px] bg-(--surface-message-user-background) px-3.5 py-2.5"
      ref={expansion.anchorRef as RefObject<HTMLDivElement>}
      data-goal-control={String(presentation.goal)}
    >
      {presentation.goal ? (
        <span className="mb-1.5 inline-flex items-center gap-1 self-start text-xs font-semibold text-(--text-muted)">
          <Target className="h-3.5 w-3.5" />
          {t("composer.goal_mode")}
        </span>
      ) : null}
      {presentation.hasContent ? (
        <>
          <div
            className={cn(
              "relative w-fit max-w-full self-end",
              collapsible && !expansion.isOpen && "overflow-hidden",
            )}
            style={collapsible && !expansion.isOpen
              ? {
                  WebkitMaskImage: "linear-gradient(to bottom, black calc(100% - 40px), transparent)",
                  maskImage: "linear-gradient(to bottom, black calc(100% - 40px), transparent)",
                  maxHeight: USER_MESSAGE_COLLAPSED_HEIGHT,
                }
              : undefined}
          >
            <div ref={measuredContentRef}>
              <ContentRenderer
                className={cn(
                  "nexus-chat-user-content w-fit max-w-[min(100%,760px)] self-end break-words text-left text-(--text-strong)",
                  presentation.contentClassName,
                )}
                content={content}
                agentMentions={agentMentions}
                agentMentionDirectory={agentMentionDirectory}
                onOpenAgentContact={onOpenAgentContact}
                onOpenWorkspaceFile={onOpenWorkspaceFile}
                renderLeadingSlashCommand
                workspaceAgentId={workspaceAgentId}
              />
            </div>
          </div>
          {collapsible ? (
            <button
              aria-expanded={expansion.isOpen}
              className="mt-1.5 inline-flex h-8 self-start items-center gap-1 rounded-[7px] px-1.5 text-sm font-medium text-(--text-muted) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
              onClick={expansion.toggle}
              type="button"
            >
              {expansion.isOpen ? t("message.show_less") : t("message.show_more")}
              <ChevronDown
                aria-hidden="true"
                className={cn(
                  "h-4 w-4 transition-transform duration-(--motion-duration-fast)",
                  expansion.isOpen && "rotate-180",
                )}
              />
            </button>
          ) : null}
        </>
      ) : null}
      <MessageUserAttachments
        attachments={attachments}
        onOpenWorkspaceFile={onOpenWorkspaceFile}
        workspaceAgentId={workspaceAgentId}
      />
    </div>
  );
}
