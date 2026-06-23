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
import { useAgentOptionsEditorController } from "@/features/agents/options/use-agent-options-editor-controller";
import type { AgentOptionsEditorProps } from "@/features/agents/options/agent-options-editor-model";

export type { AgentOptionsEditorProps } from "@/features/agents/options/agent-options-editor-model";

// ==================== 主组件 ====================

/** AgentOptions 表单主体 */
export function AgentOptionsEditor(props: AgentOptionsEditorProps) {
  const controller = useAgentOptionsEditorController(props);
  const content = (
    <>
      {controller.active_tab === "identity" && (
        <AgentOptionsIdentityTab {...controller.identity_props} />
      )}

      {controller.active_tab === "advanced" && (
        <AgentOptionsAdvancedTab {...controller.advanced_props} />
      )}

      {controller.active_tab === "skills" && (
        <AgentOptionsSkillsTab
          agent_id={controller.skills_agent_id}
          is_visible={controller.is_active && controller.active_tab === "skills"}
        />
      )}
    </>
  );

  if (controller.variant === "inline") {
    const save_feedback = controller.save_feedback ? (
      <span
        className={cn(
          "max-w-[280px] truncate text-[12px]",
          controller.save_feedback.tone === "success" ? "text-(--success)" : "text-(--destructive)",
        )}
        title={controller.save_feedback.message}
      >
        {controller.save_feedback.message}
      </span>
    ) : null;
    const save_button = (
      <>
        {save_feedback}
        <UiButton
          onClick={() => {
            void controller.handle_save();
          }}
          disabled={!controller.can_save}
          size="sm"
          tone={controller.can_save ? "primary" : "default"}
          type="button"
          variant="surface"
        >
          {controller.save_button_label}
        </UiButton>
      </>
    );

    return (
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
        {!controller.hide_inline_nav ? (
          <AgentOptionsNav
            active_tab={controller.active_tab}
            on_tab_change={controller.set_active_tab}
            variant="inline"
            trailing={save_button}
          />
        ) : null}

        <div className="min-h-0 flex-1 overflow-y-auto [overflow-anchor:none] [scrollbar-gutter:stable]">
          <div
            className={cn(
              "w-full px-6 py-5",
              controller.content_max_width_class_name,
              "mx-auto"
            )}
          >
            {content}
          </div>
        </div>

        {controller.can_delete || (controller.show_cancel_button && controller.on_cancel) || controller.hide_inline_nav ? (
          <div className="flex items-center justify-end gap-2 border-t dialog-divider px-6 py-3">
            {controller.can_delete ? (
              <UiButton
                class_name="mr-auto"
                onClick={controller.handle_delete}
                tone="danger"
                type="button"
                variant="surface"
              >
                {controller.delete_agent_label}
              </UiButton>
            ) : null}
            {controller.show_cancel_button && controller.on_cancel ? (
              <UiButton
                onClick={controller.on_cancel}
                type="button"
                variant="surface"
              >
                {controller.cancel_label}
              </UiButton>
            ) : null}
            {controller.hide_inline_nav ? save_button : null}
          </div>
        ) : null}
      </div>
    );
  }

  return (
    <>
      <div className="flex min-h-0 flex-1 overflow-hidden">
        <AgentOptionsNav
          active_tab={controller.active_tab}
          on_tab_change={controller.set_active_tab}
        />

        <div className="flex-1 overflow-y-auto bg-transparent p-6 [overflow-anchor:none] [scrollbar-gutter:stable]">
          {content}
        </div>
      </div>

      <div className="dialog-footer px-5 py-3.5">
        {controller.can_delete ? (
          <UiButton
            class_name="mr-auto"
            onClick={controller.handle_delete}
            tone="danger"
            type="button"
            variant="surface"
          >
            {controller.delete_agent_label}
          </UiButton>
        ) : null}
        {controller.show_cancel_button && controller.on_cancel ? (
          <UiButton
            onClick={controller.on_cancel}
            type="button"
            variant="surface"
          >
            {controller.cancel_label}
          </UiButton>
        ) : null}
        <UiButton
          onClick={() => {
            void controller.handle_save();
          }}
          disabled={!controller.can_save}
          tone={controller.can_save ? "primary" : "default"}
          type="button"
          variant="surface"
        >
          {controller.save_button_label}
        </UiButton>
        {controller.save_feedback ? (
          <span
            className={cn(
              "max-w-[260px] truncate text-[12px]",
              controller.save_feedback.tone === "success" ? "text-(--success)" : "text-(--destructive)",
            )}
            title={controller.save_feedback.message}
          >
            {controller.save_feedback.message}
          </span>
        ) : null}
      </div>
    </>
  );
}
