// INPUT: Agent 编辑器可用动作、保存状态与失败恢复反馈。
// OUTPUT: 由共享 Button 原语组成的删除、取消、保存动作行。
// POS: Agent options 的动作组合；不拥有按钮视觉或保存事务。

import { cn } from "@/shared/ui/class-name";
import { UiButton, type UiButtonSize } from "@/shared/ui/button/button";
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
  saveButtonSize: UiButtonSize;
}

export function AgentOptionsEditorActions({
  cancelAction,
  deleteAction,
  feedback,
  saveAction,
  saveButtonSize,
}: AgentOptionsEditorActionsProps) {
  return (
    <>
      <OptionalActionButton
        action={deleteAction}
        className="mr-auto"
        tone="danger"
      />
      <OptionalActionButton action={cancelAction} />
      <SaveFeedbackMessage feedback={feedback} />
      <UiButton
        disabled={!saveAction.enabled}
        onClick={() => {
          void saveAction.run();
        }}
        size={saveButtonSize}
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
  className,
  tone,
}: {
  action?: AgentOptionsEditorAction | null;
  className?: string;
  tone?: "danger";
}) {
  if (!action) {
    return null;
  }
  return (
    <UiButton
      className={className}
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
      <div
        aria-atomic="true"
        aria-live="polite"
        className={cn(
          "order-first w-full rounded-[10px] border px-3 py-2.5 text-left",
          feedback.tone === "warning"
            ? "border-[color:color-mix(in_srgb,var(--warning)_22%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_7%,transparent)]"
            : "border-[color:color-mix(in_srgb,var(--destructive)_22%,transparent)] bg-[color:color-mix(in_srgb,var(--destructive)_7%,transparent)]",
        )}
        role="status"
      >
        <p className="break-words text-compact font-semibold text-(--text-default)">
          {feedback.title}
        </p>
        <RecoverySummary className="mt-1" impact={feedback.impact} />
      </div>
    );
  }
  return (
    <span
      className="max-w-[280px] truncate text-compact text-(--success)"
      title={feedback.message}
    >
      {feedback.message}
    </span>
  );
}
