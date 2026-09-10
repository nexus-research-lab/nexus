// INPUT: Agent 编辑器可用动作、保存状态与失败恢复反馈。
// OUTPUT: 同一公共尺寸的删除/取消/保存动作行及完整、可播报的保存反馈。
// POS: Agent options 的动作组合；不拥有按钮视觉或保存事务。

import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { UiButton, type UiButtonSize } from "@/shared/ui/button/button";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
import { RecoverySummary } from "@/shared/ui/feedback/recovery-summary";

import type { SaveFeedback } from "../agent-options-editor-model";

export interface AgentOptionsEditorAction {
  label: string;
  run: () => void | Promise<void>;
}

interface AgentOptionsSaveAction extends AgentOptionsEditorAction {
  enabled: boolean;
}

interface AgentOptionsEditorActionsProps {
  cancelAction?: AgentOptionsEditorAction;
  deleteAction: AgentOptionsEditorAction | null;
  feedback: SaveFeedback | null;
  saveAction: AgentOptionsSaveAction;
  buttonSize: UiButtonSize;
}

export function AgentOptionsEditorActions({
  cancelAction,
  deleteAction,
  feedback,
  saveAction,
  buttonSize,
}: AgentOptionsEditorActionsProps) {
  return (
    <>
      <OptionalActionButton
        action={deleteAction}
        buttonSize={buttonSize}
        className="mr-auto"
        tone="danger"
      />
      <OptionalActionButton action={cancelAction} buttonSize={buttonSize} />
      <SaveFeedbackMessage feedback={feedback} />
      <UiButton
        disabled={!saveAction.enabled}
        onClick={() => {
          void saveAction.run();
        }}
        size={buttonSize}
        tone={saveAction.enabled ? "primary" : "default"}
        type="button"
        variant="surface"
      >
        {saveAction.label}
      </UiButton>
    </>
  );
}

function OptionalActionButton({
  action,
  buttonSize,
  className,
  tone,
}: {
  action?: AgentOptionsEditorAction | null;
  buttonSize: UiButtonSize;
  className?: string;
  tone?: "danger";
}) {
  if (!action) {
    return null;
  }
  return (
    <UiButton
      className={className}
      size={buttonSize}
      onClick={() => {
        void action.run();
      }}
      tone={tone}
      type="button"
      variant="surface"
    >
      {action.label}
    </UiButton>
  );
}

function SaveFeedbackMessage({
  feedback,
}: {
  feedback: SaveFeedback | null;
}) {
  if (!feedback) {
    return null;
  }
  if (feedback.tone !== "success") {
    return (
      <UiInlineNotice
        className="order-first"
        message={<RecoverySummary impact={feedback.impact} />}
        title={feedback.title}
        tone={feedback.tone === "warning" ? "warning" : "danger"}
      />
    );
  }
  return (
    <span
      aria-atomic="true"
      className={cn("min-w-0 break-words", getUiTypographyClassName({ role: "supporting", tone: "success" }))}
      role="status"
    >
      {feedback.message}
    </span>
  );
}
