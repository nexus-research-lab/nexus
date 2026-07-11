"use client";

import { Settings } from "lucide-react";

import { AgentOptionsDialogEditor } from "@/features/agents/options/agent-options-editor";
import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  UiDialogBackdrop,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import {
  DIALOG_HEADER_ICON_CLASS_NAME,
  DIALOG_HEADER_LEADING_CLASS_NAME,
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

/** Contacts 创建与编辑共用同一编辑器，弹窗只负责共享模态骨架与标题。 */
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

  if (!isOpen) {
    return null;
  }

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        className="z-[9999]"
        closeOnBackdrop={false}
        labelledBy="agent-options-dialog-title"
        onClose={onClose}
      >
        <UiDialogShell className="h-[80vh] max-w-[920px]" size="wide">
          <UiDialogHeader
            className="px-5 py-4"
            closeLabel={t("agent_options.close_dialog")}
            onClose={onClose}
          >
            <div className={cn(DIALOG_HEADER_LEADING_CLASS_NAME, "min-w-0 flex-1 items-center")}>
              <div className={cn(DIALOG_HEADER_ICON_CLASS_NAME, "h-11 w-11 rounded-[16px] text-primary")}>
                <Settings className="h-5 w-5" />
              </div>
              <div className="min-w-0">
                <h2
                  className="dialog-title truncate text-[22px] font-black"
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
          </UiDialogHeader>

          <AgentOptionsDialogEditor
            agentId={agentId}
            initialAvatar={initialAvatar}
            initialDescription={initialDescription}
            initialOptions={initialOptions}
            initialTitle={initialTitle}
            initialVibeTags={initialVibeTags}
            isActive={isOpen}
            mode={mode}
            onCancel={onClose}
            onDelete={onDelete}
            onSave={onSave}
            onValidateName={onValidateName}
          />
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
