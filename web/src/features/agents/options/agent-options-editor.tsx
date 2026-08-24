// INPUT: Agent Options 创建/编辑来源、当前栏目和持久化动作。
// OUTPUT: 内联或模态设置工作台；模态底部使用 plain 动作区。
// POS: Agent Options 两种明确壳层的编辑器装配，不持有业务字段副本。
import { useEffect } from "react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { WORKSPACE_CONTENT_GUTTER_CLASS_NAME } from "@/shared/ui/layout/workspace-content-layout";
import { UiDialogFooter } from "@/shared/ui/dialog/dialog";

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
  onPersistenceStateChange,
  onTabChange,
  saveMode = "explicit",
  ...formProps
}: AgentOptionsInlineEditorProps) {
  const controller = useAgentOptionsEditorController({
    ...formProps,
    activeTab,
    onTabChange,
    saveMode,
  });
  const isIdentityTab = controller.activeTab === "identity";
  const persistenceMessage = controller.persistence.message;
  const persistencePhase = controller.persistence.phase;
  useEffect(() => {
    onPersistenceStateChange?.({
      message: persistenceMessage,
      phase: persistencePhase,
    });
  }, [onPersistenceStateChange, persistenceMessage, persistencePhase]);

  return (
    <div className="flex h-full min-h-0 flex-1 flex-col overflow-hidden">
      <div
        className={cn(
          "min-h-0 flex-1 [overflow-anchor:none] [scrollbar-gutter:stable]",
          isIdentityTab ? "overflow-hidden" : "overflow-y-auto",
        )}
      >
        <div
          className={cn(
            WORKSPACE_CONTENT_GUTTER_CLASS_NAME,
            "mx-auto flex w-full flex-col py-5",
            isIdentityTab ? "h-full min-h-0" : "min-h-full",
            contentMaxWidthClassName,
          )}
        >
          <AgentOptionsEditorContent
            activeTab={controller.activeTab}
            {...controller.content}
            identityVariant="inline"
          />
        </div>
      </div>
      {saveMode === "explicit" ? (
        <div className={cn(
          WORKSPACE_CONTENT_GUTTER_CLASS_NAME,
          "flex shrink-0 items-center justify-end gap-2 py-3",
        )}>
          <AgentOptionsEditorActions
            {...controller.actions}
            saveButtonSize="sm"
          />
        </div>
      ) : null}
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
      <div className="flex min-h-0 flex-1 overflow-hidden max-xl:flex-col">
        <AgentOptionsNav
          activeTab={controller.activeTab}
          onTabChange={controller.onTabChange}
        />
        <div className="min-h-0 flex-1 overflow-y-auto bg-transparent p-5 [overflow-anchor:none] [scrollbar-gutter:stable] max-sm:p-4">
          <AgentOptionsEditorContent
            activeTab={controller.activeTab}
            {...controller.content}
            identityVariant="dialog"
          />
        </div>
      </div>
      <UiDialogFooter appearance="plain">
        <AgentOptionsEditorActions
          {...controller.actions}
          cancelAction={cancelAction}
          saveButtonSize="md"
        />
      </UiDialogFooter>
    </>
  );
}
