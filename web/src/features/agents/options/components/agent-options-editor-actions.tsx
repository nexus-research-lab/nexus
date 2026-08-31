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
          {feedback.message}
        </p>
        <p className="mt-1 break-words text-xs leading-5 text-(--text-muted)">
          {feedback.impact}
        </p>
        <p className="mt-1 break-words text-xs font-medium leading-5 text-(--text-default)">
          {feedback.nextStep}
        </p>
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
