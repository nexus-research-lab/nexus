"use client";

import { useEffect } from "react";
import { createPortal } from "react-dom";
import { Settings, X } from "lucide-react";

import { AgentOptionsEditor } from "@/features/agents/options/agent-options-editor";
import { cn } from "@/shared/ui/class-name";
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

export interface AgentOptionsDialogProps {
  agentId?: string;
  mode: "create" | "edit";
  isOpen: boolean;
  onClose: () => void;
  onDelete?: (agentId: string) => void;
  onSave: (
    title: string,
    options: AgentConfigOptions,
    identity: AgentIdentityDraft,
  ) => void | Promise<void>;
  onValidateName?: (name: string) => Promise<AgentNameValidationResult>;
  initialTitle?: string;
  initialOptions?: Partial<AgentConfigOptions>;
  initialAvatar?: string;
  initialDescription?: string;
  initialVibeTags?: string[];
}

/** Contacts 创建与编辑共用同一编辑器，弹窗只负责 Portal、键盘关闭和窗口骨架。 */
export function AgentOptionsDialog({
  agentId,
  mode,
  isOpen,
  onClose,
  onDelete,
  onSave,
  onValidateName,
  initialTitle = "",
  initialOptions = {},
  initialAvatar = "",
  initialDescription = "",
  initialVibeTags = [],
}: AgentOptionsDialogProps) {
  const { t } = useI18n();

  useEffect(() => {
    if (!isOpen) {
      return;
    }

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        onClose();
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [isOpen, onClose]);

  if (!isOpen || typeof document === "undefined") {
    return null;
  }

  return createPortal(
    <div
      aria-labelledby="agent-options-dialog-title"
      aria-modal="true"
      className="dialog-backdrop z-[9999]"
      data-modal-root="true"
      role="dialog"
    >
      <div className="dialog-shell surface-radius-md flex h-[80vh] w-full max-w-[920px] flex-col overflow-hidden">
        <div className="dialog-header px-5 py-4">
          <div className={cn(DIALOG_HEADER_LEADING_CLASS_NAME, "min-w-0 flex-1 items-center")}>
            <div className={cn(DIALOG_HEADER_ICON_CLASS_NAME, "h-11 w-11 rounded-[16px] text-primary")}>
              <Settings className="h-5 w-5" />
            </div>
            <div className="min-w-0">
              <h2
                className="dialog-title truncate text-[22px] font-black tracking-[-0.04em]"
                id="agent-options-dialog-title"
              >
                {mode === "create" ? t("agent_options.title_create") : initialTitle}
              </h2>
              {mode === "edit" && agentId ? (
                <p className="dialog-subtitle">{t("agent_options.id_prefix")}: {agentId}</p>
              ) : (
                <p className="dialog-subtitle">{t("agent_options.subtitle_create")}</p>
              )}
            </div>
          </div>
          <button
            className={DIALOG_ICON_BUTTON_CLASS_NAME}
            aria-label={t("agent_options.close_dialog")}
            onClick={onClose}
            type="button"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <AgentOptionsEditor
          agentId={agentId}
          mode={mode}
          isActive={isOpen}
          onCancel={onClose}
          onDelete={onDelete}
          onSave={onSave}
          onValidateName={onValidateName}
          initialTitle={initialTitle}
          initialOptions={initialOptions}
          initialAvatar={initialAvatar}
          initialDescription={initialDescription}
          initialVibeTags={initialVibeTags}
          closeAfterSave
          showCancelButton
        />
      </div>
    </div>,
    document.body,
  );
}
