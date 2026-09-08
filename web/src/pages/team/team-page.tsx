import { Send } from "lucide-react";
import { FormEvent, useEffect, useRef, useState } from "react";

import { useTeamRoom } from "@/features/team/use-team-room";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiTextarea } from "@/shared/ui/form/form-control";
import { UiMarkdownContent } from "@/shared/ui/markdown/markdown-content";
import { WorkspacePageFrame } from "@/shared/ui/workspace/frame/workspace-page-frame";

export function TeamPage() {
  const { t } = useI18n();
  const room = useTeamRoom();
  const [draft, setDraft] = useState("");
  const endRef = useRef<HTMLDivElement>(null);
  const errorMessage = room.error ? t(TEAM_ERROR_KEYS[room.error]) : null;

  useEffect(() => {
    endRef.current?.scrollIntoView({ block: "end" });
  }, [room.messages.length]);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (await room.send(draft)) {
      setDraft("");
    }
  };

  return (
    <WorkspacePageFrame contentPaddingClassName="p-0">
      <header className="flex h-16 shrink-0 items-center border-b border-(--divider-subtle-color) px-5">
        <div className="min-w-0">
          <h1 className="truncate text-base font-semibold">
            {room.bootstrap?.room.name ?? t("team.general")}
          </h1>
          <p className="text-xs text-(--text-soft)">{t("team.shared_room")}</p>
        </div>
      </header>

      <div className="min-h-0 flex-1 overflow-y-auto px-4 py-5 sm:px-8">
        {room.isLoading ? (
          <div className="flex h-full items-center justify-center text-sm text-(--text-soft)">
            {t("team.loading")}
          </div>
        ) : room.messages.length === 0 ? (
          <div className="flex h-full items-center justify-center text-sm text-(--text-soft)">
            {t("team.empty")}
          </div>
        ) : (
          <ol className="mx-auto flex w-full max-w-3xl flex-col gap-5">
            {room.messages.map((message) => (
              <li className="flex gap-3" key={message.id}>
                <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-(--surface-interactive-hover-background) text-sm font-semibold">
                  {(message.author_display_name || message.author_username || "?").slice(0, 1).toUpperCase()}
                </span>
                <div className="min-w-0 flex-1">
                  <div className="mb-1 flex items-baseline gap-2">
                    <span className="text-sm font-semibold">
                      {message.author_display_name || message.author_username}
                    </span>
                    <time className="text-xs text-(--text-soft)" dateTime={message.created_at}>
                      {new Date(message.created_at).toLocaleTimeString([], {
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
            <Send className="h-4 w-4" />
            {t("team.send")}
          </UiButton>
        </div>
        {errorMessage ? (
          <p className="mx-auto mt-2 w-full max-w-3xl text-xs text-destructive">
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
