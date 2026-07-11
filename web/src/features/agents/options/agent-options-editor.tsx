import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";

import {
  AgentOptionsEditorActions,
  type AgentOptionsEditorAction,
} from "./components/agent-options-editor-actions";
import { AgentOptionsEditorContent } from "./components/agent-options-editor-content";
import { AgentOptionsNav } from "./components/agent-options-nav";
import type {
  AgentOptionsDialogEditorProps,
  AgentOptionsInlineEditorProps,
} from "./agent-options-editor-model";
import { useAgentOptionsEditorController } from "./editor/use-agent-options-editor-controller";

export function AgentOptionsInlineEditor({
  activeTab,
  contentMaxWidthClassName,
  onTabChange,
  ...formProps
}: AgentOptionsInlineEditorProps) {
  const controller = useAgentOptionsEditorController({
    ...formProps,
    activeTab,
    onTabChange,
  });

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <div className="min-h-0 flex-1 overflow-y-auto [overflow-anchor:none] [scrollbar-gutter:stable]">
        <div className={cn("mx-auto w-full px-6 py-5", contentMaxWidthClassName)}>
          <AgentOptionsEditorContent
            activeTab={controller.activeTab}
            {...controller.content}
            identityVariant="inline"
          />
        </div>
      </div>
      <div className="flex items-center justify-end gap-2 border-t dialog-divider px-6 py-3">
        <AgentOptionsEditorActions
          {...controller.actions}
          saveButtonSize="sm"
        />
      </div>
    </div>
  );
}

export function AgentOptionsDialogEditor({
  onCancel,
  ...formProps
}: AgentOptionsDialogEditorProps) {
  const { t } = useI18n();
  const controller = useAgentOptionsEditorController({
    ...formProps,
    onSaveSuccess: onCancel,
  });
  const cancelAction: AgentOptionsEditorAction = {
    label: t("common.cancel"),
    run: onCancel,
  };

  return (
    <>
      <div className="flex min-h-0 flex-1 overflow-hidden">
        <AgentOptionsNav
          activeTab={controller.activeTab}
          onTabChange={controller.onTabChange}
        />
        <div className="flex-1 overflow-y-auto bg-transparent p-6 [overflow-anchor:none] [scrollbar-gutter:stable]">
          <AgentOptionsEditorContent
            activeTab={controller.activeTab}
            {...controller.content}
            identityVariant="dialog"
          />
        </div>
      </div>
      <div className="dialog-footer px-5 py-3.5">
        <AgentOptionsEditorActions
          {...controller.actions}
          cancelAction={cancelAction}
          saveButtonSize="md"
        />
      </div>
    </>
  );
}
