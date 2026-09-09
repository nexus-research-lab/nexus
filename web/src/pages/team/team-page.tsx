// INPUT: Team room read model, controlled draft and explicit send command.
// OUTPUT: Human-message page with shared typography, Unicode initials and IME-safe submission.
// POS: Team presentation; transport and persistence remain in useTeamRoom.
import { Send } from "lucide-react";
import { FormEvent, useEffect, useRef, useState } from "react";

import { getInitials } from "@/lib/avatar";
import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { useTeamRoom } from "@/features/team/use-team-room";
import { isImeKeyboardEvent } from "@/shared/lib/browser/ime-keyboard-event";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiTextarea } from "@/shared/ui/form/form-control";
import { UiMarkdownContent } from "@/shared/ui/markdown/markdown-content";
import { WorkspacePageFrame } from "@/shared/ui/workspace/frame/workspace-page-frame";

export function TeamPage() {
  const { locale, t } = useI18n();
  const room = useTeamRoom();
  const [draft, setDraft] = useState("");
  const endRef = useRef<HTMLDivElement>(null);
  const errorMessage = room.error ? t(TEAM_ERROR_KEYS[room.error]) : null;

  useEffect(() => {
    endRef.current?.scrollIntoView({ block: "end" });
  }, [room.messages.length]);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!draft.trim() || room.isSending || !room.bootstrap) return;
    if (await room.send(draft)) {
      setDraft("");
    }
  };

  return (
    <WorkspacePageFrame contentPaddingClassName="p-0">
      <header className="flex h-16 shrink-0 items-center border-b border-(--divider-subtle-color) px-5">
        <div className="min-w-0">
          <h1 className={cn("truncate", getUiTypographyClassName({ role: "pageTitle", tone: "strong" }))}>
            {room.bootstrap?.room.name ?? t("team.general")}
          </h1>
          <p className={getUiTypographyClassName({ role: "caption", tone: "soft" })}>{t("team.shared_room")}</p>
        </div>
      </header>

      <div className="min-h-0 flex-1 overflow-y-auto px-4 py-5 sm:px-8">
        {room.isLoading ? (
          <div role="status" className={cn("flex h-full items-center justify-center", getUiTypographyClassName({ role: "supporting", tone: "soft" }))}>
            {t("team.loading")}
          </div>
        ) : room.messages.length === 0 && room.error !== "load" ? (
          <div className={cn("flex h-full items-center justify-center", getUiTypographyClassName({ role: "supporting", tone: "soft" }))}>
            {t("team.empty")}
          </div>
        ) : (
          <ol className="mx-auto flex w-full max-w-3xl flex-col gap-5">
            {room.messages.map((message) => (
              <li className="flex gap-3" key={message.id}>
                <span aria-hidden="true" className={cn("flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-(--surface-interactive-hover-background)", getUiTypographyClassName({ role: "supporting", weight: "semibold" }))}>
                  {getInitials(message.author_display_name || message.author_username, "?", 1)}
                </span>
                <div className="min-w-0 flex-1">
                  <div className="mb-1 flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
                    <span className={cn("min-w-0 break-words", getUiTypographyClassName({ role: "supporting", weight: "semibold", tone: "strong" }))}>
                      {message.author_display_name || message.author_username}
                    </span>
                    <time className={getUiTypographyClassName({ role: "caption", tone: "soft" })} dateTime={message.created_at}>
                      {new Date(message.created_at).toLocaleTimeString(locale, {
                        hour: "2-digit",
                        minute: "2-digit",
                      })}
                    </time>
                  </div>
                  <UiMarkdownContent
                    content={message.content.blocks.map((block) => block.text).join("\n\n")}
                  />
                </div>
              </li>
            ))}
          </ol>
        )}
        <div ref={endRef} />
      </div>

      <form className="shrink-0 border-t border-(--divider-subtle-color) p-3 sm:px-8" onSubmit={submit}>
        <div className="mx-auto flex w-full max-w-3xl items-end gap-2">
          <UiTextarea
            aria-label={t("team.message")}
            className="max-h-40 min-h-11 flex-1 resize-none"
            disabled={!room.bootstrap || room.isSending}
            onChange={(event) => setDraft(event.target.value)}
            onKeyDown={(event) => {
              if (event.defaultPrevented || isImeKeyboardEvent(event.nativeEvent)) return;
              if (event.key === "Enter" && !event.shiftKey) {
                event.preventDefault();
                event.currentTarget.form?.requestSubmit();
              }
            }}
            placeholder={t("team.message_placeholder")}
            rows={1}
            value={draft}
          />
          <UiButton
            aria-label={t("team.send")}
            disabled={!draft.trim() || room.isSending || !room.bootstrap}
            size="md"
            tone="primary"
            type="submit"
            variant="solid"
          >
            <Send aria-hidden="true" className="h-4 w-4" />
            {t("team.send")}
          </UiButton>
        </div>
        {errorMessage ? (
          <p role="alert" className={cn("mx-auto mt-2 w-full max-w-3xl", getUiTypographyClassName({ role: "caption", tone: "danger" }))}>
            {errorMessage}
          </p>
        ) : null}
      </form>
    </WorkspacePageFrame>
  );
}

const TEAM_ERROR_KEYS = {
  load: "team.error_load",
  send: "team.error_send",
  sync: "team.error_sync",
} as const;
