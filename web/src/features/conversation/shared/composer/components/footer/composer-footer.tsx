/**
 * INPUT: Composer 动作、运行态、输入元数据、Nexus 标注与提交投影。
 * OUTPUT: 普通模式按宽壳三列收敛；Goal 模式让控制/提交分栏并把状态隔离到下一行的 Footer。
 * POS: Composer 壳内唯一的底部动作与状态布局。
 */

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
      <div className="nexus-chat-composer-footer-leading flex min-w-0 items-center gap-2 text-2xs text-(--text-soft)">
        <ComposerFooterActions
          actionButtonRef={props.actionButtonRef}
          canCreateGoal={props.canCreateGoal}
          canUseLoop={props.canUseLoop}
          isActionMenuOpen={props.isActionMenuOpen}
          isGoalCreating={props.isGoalCreating}
          isGoalMode={props.isGoalMode}
          isPreparingAttachments={props.isPreparingAttachments}
          onActionMenuClose={props.onActionMenuClose}
          onActionMenuToggle={props.onActionMenuToggle}
          onAttachmentSelect={props.onAttachmentSelect}
          onGoalToggle={props.onGoalToggle}
          onLoopSelect={props.onLoopSelect}
        />
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
          isGoalCreating={props.isGoalCreating}
          isPreparingAttachments={props.isPreparingAttachments}
          runtimeActivity={props.runtimeActivity}
        />
      </div>
      <ComposerPoweredByNexus visible={props.showPoweredByNexus} />
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

function ComposerPoweredByNexus({ visible }: { visible: boolean }) {
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
      className="nexus-chat-composer-footer-brand whitespace-nowrap text-center text-[11px] font-medium leading-4 tracking-[0.01em]"
      data-composer-powered-by
    >
      Powered by Nexus
    </span>
  );
}
