"use client";

import { Settings } from "lucide-react";

import { AgentOptionsDialogEditor } from "@/features/agents/options/agent-options-editor";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  UiDialogBackdrop,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
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
        className="z-[9999] max-sm:p-2"
        closeOnBackdrop={false}
        labelledBy="agent-options-dialog-title"
        onClose={onClose}
      >
        <UiDialogShell
          className="h-[min(82dvh,760px)] max-sm:h-[calc(100dvh-16px)]"
          size="wide"
          style={{ maxWidth: "900px" }}
        >
          <UiDialogHeader
            className="max-sm:px-4 max-sm:py-3"
            closeLabel={t("agent_options.close_dialog")}
            icon={<Settings className="h-4 w-4" />}
            onClose={onClose}
            subtitle={<span className="max-sm:hidden">{header.subtitle}</span>}
            title={header.title}
            titleId="agent-options-dialog-title"
          />

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
