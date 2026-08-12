"use client";

/**
 * INPUT: 当前会话草稿、投递能力、Goal/附件动作、人工介入与 runtime 状态。
 * OUTPUT: 带自身上缘羽化的稳定 Composer 壳，内容在普通输入与原位人工确认之间二选一。
 * POS: DM 与 Room 共用 Composer 的纯视图装配入口。
 */

import { memo } from "react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";

import { CONVERSATION_COMPOSER_LANE_CLASS_NAME } from "../conversation-panel-styles";
import { COMPOSER_ATTACHMENT_ACCEPT } from "./attachments/composer-attachments";
import { ComposerAttachmentList } from "./attachments/composer-local-attachments";
import { ComposerFooter } from "./components/footer/composer-footer";
import { ComposerInputRow } from "./components/composer-input-row";
import { ComposerPendingQueue } from "./components/pending-queue/composer-pending-queue";
import { LoopPickerDialog } from "./components/loop-picker/loop-picker-dialog";
import {
  MAX_COMPOSER_INPUT_LENGTH,
  type ComposerPanelProps,
} from "./composer-model";
import {
  COMPOSER_COMPACT_LANE_CLASS_NAME,
  COMPOSER_SHELL_CLASS_NAME,
} from "./composer-styles";
import { useComposerController } from "./controller/use-composer-controller";

const ComposerPanelView = memo((props: ComposerPanelProps) => {
  const { t } = useI18n();
  const {
    actions,
    attachments,
    mention,
    refs,
    sessionSettings,
    slashCommand,
    state,
  } = useComposerController(props);

  return (
    <section
      data-tour-anchor={props.tourAnchor}
      className={cn(
        "bg-transparent",
        props.compact
          ? `${COMPOSER_COMPACT_LANE_CLASS_NAME} px-4 pb-[max(0.75rem,env(safe-area-inset-bottom))] pt-2`
          : `${CONVERSATION_COMPOSER_LANE_CLASS_NAME} px-3 pb-2 pt-2 sm:px-5 xl:px-6`,
      )}
    >
      {props.showActionMenu !== false ? (
        <input
          ref={refs.fileInputRef}
          accept={COMPOSER_ATTACHMENT_ACCEPT}
          aria-label={t("composer.choose_attachment_file")}
          className="hidden"
          multiple
          onChange={attachments.handleFileSelect}
          type="file"
        />
      ) : null}
      {state.canUseLoop && !props.interactionSurface ? (
        <LoopPickerDialog
          isOpen={state.isLoopPickerOpen}
          onClose={() => actions.setIsLoopPickerOpen(false)}
          onSelect={actions.handleLoopSelect}
        />
      ) : null}

      <div
        className="nexus-chat-composer-edge relative isolate"
        data-composer-edge="true"
      >
        <div
          ref={refs.composerShellRef}
          className={COMPOSER_SHELL_CLASS_NAME}
          data-composer-surface={
            props.interactionSurface ? "interaction" : "input"
          }
        >
          {props.interactionSurface ?? (
            <>
              <ComposerPendingQueue
                compact={props.compact}
                inputQueueItems={props.inputQueueItems}
                onDeleteQueuedMessage={props.onDeleteQueuedMessage}
                onGuideQueuedMessage={props.onGuideQueuedMessage}
                onReorderQueueMessages={props.onReorderQueueMessages}
              />

              {props.showActionMenu !== false ? (
                <ComposerAttachmentList
                  attachments={attachments.attachments}
                  onRemove={attachments.removeAttachment}
                  previewResetKey={props.draftScopeKey}
                  removeLabel={t("composer.remove_attachment")}
                />
              ) : null}

              <ComposerInputRow
                input={{
                  disabled: state.isTextareaLocked,
                  onChange: actions.handleInputChange,
                  onCompositionEnd: actions.handleCompositionEnd,
                  onCompositionStart: actions.handleCompositionStart,
                  onKeyDown: actions.handleKeyDown,
                  onPaste: props.showActionMenu === false
                    ? ignoreComposerPaste
                    : attachments.handlePaste,
                  placeholder: state.resolvedPlaceholder,
                  value: state.input,
                }}
                layout={{
                  paddingClassName: state.composerInputRowPaddingClass,
                }}
                composerShellRef={refs.composerShellRef}
                mention={{
                  active: mention.mentionActive,
                  filter: mention.mentionFilter,
                  items: mention.mentionTargetItems,
                  onClose: mention.closeMention,
                  onSelect: mention.selectMentionItem,
                }}
                slashCommand={{
                  active: slashCommand.isOpen,
                  activeIndex: slashCommand.activeIndex,
                  commands: slashCommand.commands,
                  mode: slashCommand.mode,
                  modelError: slashCommand.modelError,
                  modelItems: slashCommand.modelItems,
                  modelLoading: slashCommand.modelLoading,
                  modelQuery: slashCommand.modelQuery,
                  modelSearchRef: slashCommand.modelSearchRef,
                  onModelQueryChange: slashCommand.onModelQueryChange,
                  onModelQueryKeyDown: slashCommand.onModelQueryKeyDown,
                  onClose: slashCommand.onClose,
                  onSelectModel: slashCommand.onSelectModel,
                  onSelectCommand: slashCommand.onSelectCommand,
                  onSelectSkill: slashCommand.onSelectSkill,
                  onSkillQueryChange: slashCommand.onSkillQueryChange,
                  onSkillQueryKeyDown: slashCommand.onSkillQueryKeyDown,
                  skillError: slashCommand.skillError,
                  skillItems: slashCommand.skillItems,
                  skillLoading: slashCommand.skillLoading,
                  skillQuery: slashCommand.skillQuery,
                  skillSearchRef: slashCommand.skillSearchRef,
                  status: slashCommand.status,
                }}
                textareaRef={refs.textareaRef}
              />

              <ComposerFooter
                actionButtonRef={refs.actionButtonRef}
                activeError={state.activeError}
                canCreateGoal={state.canCreateGoal}
                canUseLoop={state.canUseLoop}
                charCount={state.charCount}
                contextUsage={props.contextUsage}
                contextUsageItems={props.contextUsageItems}
                goalModeExtra={props.goalModeExtra ?? null}
                goalScopeLabel={props.goalScopeLabel}
                historyIndex={state.historyIndex}
                inputHistoryLength={state.inputHistoryLength}
                isActionMenuOpen={state.isActionMenuOpen}
                isGoalCreating={state.isGoalCreating}
                isGoalMode={state.isGoalMode}
                isNearLimit={state.isNearLimit}
                isOverLimit={state.isOverLimit}
                isPreparingAttachments={state.isPreparingAttachments}
                maxLength={MAX_COMPOSER_INPUT_LENGTH}
                onActionMenuClose={() => actions.setIsActionMenuOpen(false)}
                onActionMenuToggle={() => {
                  actions.setIsActionMenuOpen((current) => !current);
                }}
                onAttachmentSelect={actions.openAttachmentPicker}
                onCancelGoal={actions.cancelGoalInput}
                onGoalToggle={actions.toggleGoalInput}
                onLoopSelect={actions.openLoopPicker}
                runtimeActivity={state.runtimeActivity}
                sessionSettingsController={sessionSettings}
                sessionSettingsDisabled={
                  props.isLoading || state.runtimeActivity !== null
                }
                showActionMenu={props.showActionMenu !== false}
                showPoweredByNexus
                submit={{
                  isDisabled: state.isSendDisabled,
                  isGoalCreating: state.isGoalCreating,
                  isGoalMode: state.isGoalMode,
                  isPreparingAttachments: state.isPreparingAttachments,
                  onSend: actions.handleSend,
                  onStop: props.onStop,
                  sendLabel: state.sendButtonLabel,
                  shouldStop: state.shouldShowStopButton,
                  stopLabel: props.stopLabel ?? t("composer.stop_generation"),
                }}
              />
            </>
          )}
        </div>
      </div>
    </section>
  );
});

ComposerPanelView.displayName = "ComposerPanelView";

export function ComposerPanel(props: ComposerPanelProps) {
  return <ComposerPanelView {...props} />;
}

function ignoreComposerPaste(): void {}
