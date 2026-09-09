/**
 * INPUT: Composer 动作、运行态、输入元数据、内核品牌标注与提交投影。
 * OUTPUT: 普通模式三列与居中品牌、Goal 模式控制/提交分栏及下一行状态；文本消费共享 Typography。
 * POS: Composer 壳内唯一的底部动作与状态布局。
 */

import type { AgentRuntimeKind } from "@/types/settings/preferences";

import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import { COMPOSER_FOOTER_CLASS_NAME } from "../../composer-styles";
import { ComposerSubmitButton } from "../composer-submit-button";
import { ComposerFooterActions } from "./composer-footer-actions";
import { ComposerContextUsage } from "./composer-context-usage";
import { ComposerFooterMetadata } from "./composer-footer-metadata";
import type { ComposerFooterProps } from "./composer-footer-model";
import {
  ComposerSessionControls,
} from "./composer-session-controls";
import {
  ComposerFooterStatus,
  ComposerGoalModeIndicator,
} from "./composer-footer-status";

export function ComposerFooter(props: ComposerFooterProps) {
  return (
    <div
      className={COMPOSER_FOOTER_CLASS_NAME}
      data-goal-mode={props.isGoalMode ? "true" : "false"}
    >
      <div className="nexus-chat-composer-footer-leading flex min-w-0 items-center gap-2">
        {props.showActionMenu ? (
          <ComposerFooterActions
            actionButtonRef={props.actionButtonRef}
            canCreateGoal={props.canCreateGoal}
            canUseWorkGraphDistillations={props.canUseWorkGraphDistillations}
            isActionMenuOpen={props.isActionMenuOpen}
            isGoalCreating={props.isGoalCreating}
            isGoalMode={props.isGoalMode}
            isPreparingAttachments={props.isPreparingAttachments}
            localDirectoriesController={props.localDirectoriesController}
            onActionMenuClose={props.onActionMenuClose}
            onActionMenuToggle={props.onActionMenuToggle}
            onAttachmentSelect={props.onAttachmentSelect}
            onGoalToggle={props.onGoalToggle}
            onWorkGraphDistillationsSelect={props.onWorkGraphDistillationsSelect}
            onLocalDirectorySelect={props.onLocalDirectorySelect}
            sessionSettingsController={props.sessionSettingsController}
            sessionSettingsDisabled={props.sessionSettingsDisabled}
          />
        ) : null}
        <ComposerSessionControls
          controller={props.sessionSettingsController}
          disabled={props.sessionSettingsDisabled}
          slot="leading"
        />
        <ComposerGoalModeIndicator
          extra={props.goalModeExtra}
          isCreating={props.isGoalCreating}
          onCancel={props.onCancelGoal}
          scopeLabel={props.goalScopeLabel}
          visible={props.isGoalMode}
        />
        <ComposerFooterStatus
          activeError={props.activeError}
          isGoalConfirming={props.isGoalConfirming}
          isGoalCreating={props.isGoalCreating}
          isPreparingAttachments={props.isPreparingAttachments}
          runtimeActivity={props.runtimeActivity}
        />
      </div>
      <ComposerPoweredByNexus runtimeKind={props.runtimeKind} />
      <div className="nexus-chat-composer-footer-trailing flex min-w-0 items-center justify-self-end gap-2 overflow-hidden">
        <ComposerContextUsage
          items={props.contextUsageItems}
          usage={props.contextUsage}
        />
        <ComposerSessionControls
          controller={props.sessionSettingsController}
          disabled={props.sessionSettingsDisabled}
          slot="trailing"
        />
        <ComposerFooterMetadata
          charCount={props.charCount}
          historyIndex={props.historyIndex}
          inputHistoryLength={props.inputHistoryLength}
          isNearLimit={props.isNearLimit}
          isOverLimit={props.isOverLimit}
          maxLength={props.maxLength}
        />
        <ComposerSubmitButton {...props.submit} />
      </div>
    </div>
  );
}

export function ComposerPoweredByNexus({ visible = true, runtimeKind = "nxs" }: { visible?: boolean; runtimeKind?: AgentRuntimeKind }) {
  if (!visible) {
    return (
      <span
        aria-hidden="true"
        className="nexus-chat-composer-footer-brand"
      />
    );
  }
  return (
    <span
      className={`nexus-chat-composer-footer-brand whitespace-nowrap text-center tracking-[0.01em] ${getUiTypographyClassName({ role: "caption", weight: "medium" })}`}
      data-composer-powered-by
    >
      Powered by {runtimeKind === "claude" ? "Claude" : "Nexus"}
    </span>
  );
}
