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
  AgentOptionsFormProps,
} from "../agent-options-editor-model";
import {
  type AgentOptionsDialogState,
  getAgentOptionsDialogHeader,
} from "./agent-options-dialog-model";

interface AgentOptionsDialogProps {
  onClose: () => void;
  onDelete: NonNullable<AgentOptionsFormProps["onDelete"]>;
  onSave: AgentOptionsFormProps["onSave"];
  onValidateName: NonNullable<AgentOptionsFormProps["onValidateName"]>;
  state: AgentOptionsDialogState;
}

/** Contacts 创建与编辑共用同一编辑器，弹窗只负责共享模态骨架与标题。 */
export function AgentOptionsDialog({
  onClose,
  onDelete,
  onSave,
  onValidateName,
  state,
}: AgentOptionsDialogProps) {
  const { t } = useI18n();

  if (state.kind === "closed") {
    return null;
  }
  const header = getAgentOptionsDialogHeader(state, t);

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
                  {header.title}
                </h2>
                <p className="dialog-subtitle">{header.subtitle}</p>
              </div>
            </div>
          </UiDialogHeader>

          <AgentOptionsDialogEditor
            isActive
            onCancel={onClose}
            onDelete={onDelete}
            onSave={onSave}
            onValidateName={onValidateName}
            source={state}
          />
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
