/**
 * =====================================================
 * @File   : agent-options-editor.tsx
 * @Date   : 2026-04-15 17:35
 * @Author : leemysw
 * 2026-04-15 17:35   Create
 * =====================================================
 */

"use client";

import { cn } from "@/lib/utils";
import { UiButton } from "@/shared/ui/button";
import { AgentOptionsNav } from "@/features/agents/options/components/agent-options-nav";
import { AgentOptionsIdentityTab } from "@/features/agents/options/components/agent-options-identity-tab";
import { AgentOptionsSkillsTab } from "@/features/agents/options/components/agent-options-skills-tab";
import { AgentOptionsAdvancedTab } from "@/features/agents/options/components/agent-options-advanced-tab";
import { useAgentOptionsEditorController } from "@/features/agents/options/editor/use-agent-options-editor-controller";
import type { AgentOptionsEditorProps } from "@/features/agents/options/agent-options-editor-model";

export type { AgentOptionsEditorProps } from "@/features/agents/options/agent-options-editor-model";

// ==================== 主组件 ====================

/** AgentOptions 表单主体 */
export function AgentOptionsEditor(props: AgentOptionsEditorProps) {
  const controller = useAgentOptionsEditorController(props);
  const content = (
    <>
      {controller.activeTab === "identity" && (
        <AgentOptionsIdentityTab {...controller.identityProps} />
      )}

      {controller.activeTab === "advanced" && (
        <AgentOptionsAdvancedTab {...controller.advancedProps} />
      )}

      {controller.activeTab === "skills" && (
        <AgentOptionsSkillsTab
          agentId={controller.skillsAgentId}
          isVisible={controller.isActive && controller.activeTab === "skills"}
        />
      )}
    </>
  );

  if (controller.variant === "inline") {
    const saveFeedback = controller.saveFeedback ? (
      <span
        className={cn(
          "max-w-[280px] truncate text-[12px]",
          controller.saveFeedback.tone === "success" ? "text-(--success)" : "text-(--destructive)",
        )}
        title={controller.saveFeedback.message}
      >
        {controller.saveFeedback.message}
      </span>
    ) : null;
    const saveButton = (
      <>
        {saveFeedback}
        <UiButton
          onClick={() => {
            void controller.handleSave();
          }}
          disabled={!controller.canSave}
          size="sm"
          tone={controller.canSave ? "primary" : "default"}
          type="button"
          variant="surface"
        >
          {controller.saveButtonLabel}
        </UiButton>
      </>
    );

    return (
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
        {!controller.hideInlineNav ? (
          <AgentOptionsNav
            activeTab={controller.activeTab}
            onTabChange={controller.setActiveTab}
            variant="inline"
            trailing={saveButton}
          />
        ) : null}

        <div className="min-h-0 flex-1 overflow-y-auto [overflow-anchor:none] [scrollbar-gutter:stable]">
          <div
            className={cn(
              "w-full px-6 py-5",
              controller.contentMaxWidthClassName,
              "mx-auto"
            )}
          >
            {content}
          </div>
        </div>

        {controller.canDelete || (controller.showCancelButton && controller.onCancel) || controller.hideInlineNav ? (
          <div className="flex items-center justify-end gap-2 border-t dialog-divider px-6 py-3">
            {controller.canDelete ? (
              <UiButton
                className="mr-auto"
                onClick={controller.handleDelete}
                tone="danger"
                type="button"
                variant="surface"
              >
                {controller.deleteAgentLabel}
              </UiButton>
            ) : null}
            {controller.showCancelButton && controller.onCancel ? (
              <UiButton
                onClick={controller.onCancel}
                type="button"
                variant="surface"
              >
                {controller.cancelLabel}
              </UiButton>
            ) : null}
            {controller.hideInlineNav ? saveButton : null}
          </div>
        ) : null}
      </div>
    );
  }

  return (
    <>
      <div className="flex min-h-0 flex-1 overflow-hidden">
        <AgentOptionsNav
          activeTab={controller.activeTab}
          onTabChange={controller.setActiveTab}
        />

        <div className="flex-1 overflow-y-auto bg-transparent p-6 [overflow-anchor:none] [scrollbar-gutter:stable]">
          {content}
        </div>
      </div>

      <div className="dialog-footer px-5 py-3.5">
        {controller.canDelete ? (
          <UiButton
            className="mr-auto"
            onClick={controller.handleDelete}
            tone="danger"
            type="button"
            variant="surface"
          >
            {controller.deleteAgentLabel}
          </UiButton>
        ) : null}
        {controller.showCancelButton && controller.onCancel ? (
          <UiButton
            onClick={controller.onCancel}
            type="button"
            variant="surface"
          >
            {controller.cancelLabel}
          </UiButton>
        ) : null}
        <UiButton
          onClick={() => {
            void controller.handleSave();
          }}
          disabled={!controller.canSave}
          tone={controller.canSave ? "primary" : "default"}
          type="button"
          variant="surface"
        >
          {controller.saveButtonLabel}
        </UiButton>
        {controller.saveFeedback ? (
          <span
            className={cn(
              "max-w-[260px] truncate text-[12px]",
              controller.saveFeedback.tone === "success" ? "text-(--success)" : "text-(--destructive)",
            )}
            title={controller.saveFeedback.message}
          >
            {controller.saveFeedback.message}
          </span>
        ) : null}
      </div>
    </>
  );
}
