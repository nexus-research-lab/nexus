/**
 * =====================================================
 * @File   : agent-options-dialog.tsx
 * @Date   : 2026-04-15 17:38
 * @Author : leemysw
 * 2026-04-15 17:38   Create
 * =====================================================
 */

"use client";

import { useEffect } from "react";
import { createPortal } from "react-dom";
import { Settings, X } from "lucide-react";

import { AgentOptionsEditor } from "@/features/agents/options/agent-options-editor";
import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  DIALOG_HEADER_ICON_CLASS_NAME,
  DIALOG_HEADER_LEADING_CLASS_NAME,
  DIALOG_ICON_BUTTON_CLASS_NAME,
} from "@/shared/ui/dialog/dialog-styles";
import type {
  AgentIdentityDraft,
  AgentNameValidationResult,
  AgentOptions as AgentConfigOptions,
} from "@/types/agent/agent";

export interface AgentOptionsProps {
  agent_id?: string;
  mode: "create" | "edit";
  is_open: boolean;
  on_close: () => void;
  on_delete?: (agent_id: string) => void;
  on_save: (title: string, options: AgentConfigOptions, identity: AgentIdentityDraft) => void | Promise<void>;
  on_validate_name?: (name: string) => Promise<AgentNameValidationResult>;
  initial_title?: string;
  initial_options?: Partial<AgentConfigOptions>;
  initial_avatar?: string;
  initial_description?: string;
  initial_vibe_tags?: string[];
}

/** 中文注释：共享层只保留对话框壳体，真实编辑器和业务状态迁回 feature。 */
export function AgentOptions({
  agent_id,
  mode,
  is_open,
  on_close,
  on_delete,
  on_save,
  on_validate_name,
  initial_title = "",
  initial_options = {},
  initial_avatar = "",
  initial_description = "",
  initial_vibe_tags = [],
}: AgentOptionsProps) {
  const { t } = useI18n();

  useEffect(() => {
    if (!is_open) {
      return;
    }

    const handle_key_down = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        on_close();
      }
    };

    window.addEventListener("keydown", handle_key_down);
    return () => window.removeEventListener("keydown", handle_key_down);
  }, [is_open, on_close]);

  if (!is_open || typeof document === "undefined") {
    return null;
  }

  return createPortal(
    <div className="dialog-backdrop z-[9999]" role="dialog" aria-modal="true">
      <div className="dialog-shell radius-shell-xl flex h-[80vh] w-full max-w-[920px] flex-col overflow-hidden">
        <div className="dialog-header px-5 py-4">
          <div className={cn(DIALOG_HEADER_LEADING_CLASS_NAME, "min-w-0 flex-1 items-center")}>
            <div className={cn(DIALOG_HEADER_ICON_CLASS_NAME, "h-11 w-11 rounded-[16px] text-primary")}>
              <Settings className="h-5 w-5" />
            </div>
            <div className="min-w-0">
              <h2 className="dialog-title truncate text-[22px] font-black tracking-[-0.04em]">
                {mode === "create" ? t("agent_options.title_create") : initial_title}
              </h2>
              {mode === "edit" && agent_id ? (
                <p className="dialog-subtitle">{t("agent_options.id_prefix")}: {agent_id}</p>
              ) : (
                <p className="dialog-subtitle">{t("agent_options.subtitle_create")}</p>
              )}
            </div>
          </div>
          <button
            className={DIALOG_ICON_BUTTON_CLASS_NAME}
            aria-label={t("agent_options.close_dialog")}
            onClick={on_close}
            type="button"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <AgentOptionsEditor
          agent_id={agent_id}
          mode={mode}
          is_active={is_open}
          on_cancel={on_close}
          on_delete={on_delete}
          on_save={on_save}
          on_validate_name={on_validate_name}
          initial_title={initial_title}
          initial_options={initial_options}
          initial_avatar={initial_avatar}
          initial_description={initial_description}
          initial_vibe_tags={initial_vibe_tags}
          close_after_save
          show_cancel_button
        />
      </div>
    </div>,
    document.body,
  );
}

export { AgentOptionsEditor } from "@/features/agents/options/agent-options-editor";
