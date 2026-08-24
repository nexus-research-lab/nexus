// INPUT: Contacts 的创建/编辑 Agent 状态与保存、删除、关闭动作。
// OUTPUT: plain 标题和持续可切换的 Agent 设置工作台，不展示内部 ID 副标题。
// POS: Agent Options 的模态壳层，字段与事务全部委托给编辑器。
"use client";

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
  getAgentOptionsDialogTitle,
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
  const title = getAgentOptionsDialogTitle(state, t);

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
            appearance="plain"
            className="max-sm:px-4 max-sm:py-3"
            closeLabel={t("agent_options.close_dialog")}
            onClose={onClose}
            title={title}
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
