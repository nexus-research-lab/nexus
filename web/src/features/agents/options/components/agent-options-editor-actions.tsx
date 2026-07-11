import { cn } from "@/shared/ui/class-name";
import { UiButton } from "@/shared/ui/button/button";
import type { UiButtonSize } from "@/shared/ui/button/button-styles";

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
  return (
    <span
      className={cn(
        "max-w-[280px] truncate text-[12px]",
        feedback.tone === "success"
          ? "text-(--success)"
          : "text-(--destructive)",
      )}
      title={feedback.message}
    >
      {feedback.message}
    </span>
  );
}
