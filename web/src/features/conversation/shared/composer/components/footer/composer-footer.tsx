import { COMPOSER_FOOTER_CLASS_NAME } from "../../composer-styles";
import { ComposerFooterActions } from "./composer-footer-actions";
import { ComposerFooterMetadata } from "./composer-footer-metadata";
import type { ComposerFooterProps } from "./composer-footer-model";
import {
  ComposerFooterStatus,
  ComposerGoalModeIndicator,
} from "./composer-footer-status";

export function ComposerFooter(props: ComposerFooterProps) {
  return (
    <div className={COMPOSER_FOOTER_CLASS_NAME}>
      <div className="flex min-w-0 items-center gap-2 text-[10px] text-(--text-soft)">
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
        <ComposerGoalModeIndicator
          extra={props.goalModeExtra}
          isCreating={props.isGoalCreating}
          onCancel={props.onCancelGoal}
          scopeLabel={props.goalScopeLabel}
          visible={props.isGoalMode}
        />
        <ComposerFooterStatus
          activeError={props.activeError}
          canStopGeneration={props.canStopGeneration}
          isDispatching={props.isDispatching}
          isGoalCreating={props.isGoalCreating}
          isPreparingAttachments={props.isPreparingAttachments}
        />
      </div>
      <ComposerFooterMetadata
        charCount={props.charCount}
        historyIndex={props.historyIndex}
        inputHistoryLength={props.inputHistoryLength}
        isNearLimit={props.isNearLimit}
        isOverLimit={props.isOverLimit}
        maxLength={props.maxLength}
      />
    </div>
  );
}
