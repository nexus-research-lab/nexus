"use client";

import { memo } from "react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";

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
  COMPOSER_SHELL_CLASS_NAME,
} from "./composer-styles";
import { useComposerController } from "./controller/use-composer-controller";

const ComposerPanelView = memo((props: ComposerPanelProps) => {
  const { t } = useI18n();
  const { actions, attachments, mention, refs, state } =
    useComposerController(props);

  return (
    <section
      data-tour-anchor={props.tourAnchor}
      className={cn(
        "mx-auto w-full max-w-[1020px] border-t border-(--surface-canvas-border) bg-transparent",
        props.compact
          ? "px-2 pb-2 pt-2"
          : "px-3 pb-3 pt-3 sm:px-5 xl:px-6",
      )}
    >
      <input
        ref={refs.fileInputRef}
        accept={COMPOSER_ATTACHMENT_ACCEPT}
        aria-label={t("composer.choose_attachment_file")}
        className="hidden"
        multiple
        onChange={attachments.handleFileSelect}
        type="file"
      />
      {state.canUseLoop ? (
        <LoopPickerDialog
          isOpen={state.isLoopPickerOpen}
          onClose={() => actions.setIsLoopPickerOpen(false)}
          onSelect={actions.handleLoopSelect}
        />
      ) : null}

      <div className={COMPOSER_SHELL_CLASS_NAME}>
        <ComposerPendingQueue
          compact={props.compact}
          inputQueueItems={props.inputQueueItems}
          onDeleteQueuedMessage={props.onDeleteQueuedMessage}
          onGuideQueuedMessage={props.onGuideQueuedMessage}
          onReorderQueueMessages={props.onReorderQueueMessages}
        />

        <ComposerAttachmentList
          attachments={attachments.attachments}
          onRemove={attachments.removeAttachment}
          removeLabel={t("composer.remove_attachment")}
        />

        <ComposerInputRow
          input={{
            disabled: state.isTextareaLocked,
            onChange: actions.handleInputChange,
            onCompositionEnd: actions.handleCompositionEnd,
            onCompositionStart: actions.handleCompositionStart,
            onKeyDown: actions.handleKeyDown,
            onPaste: attachments.handlePaste,
            placeholder: state.resolvedPlaceholder,
            value: state.input,
          }}
          layout={{
            enterLabel: state.inlineEnterLabel,
            newLineLabel: t("composer.shift_enter_newline"),
            paddingClassName: state.composerInputRowPaddingClass,
            showShortcuts: state.shouldShowInlineShortcuts,
          }}
          mention={{
            active: mention.mentionActive,
            filter: mention.mentionFilter,
            items: mention.mentionTargetItems,
            onClose: mention.closeMention,
            onSelect: mention.selectMentionItem,
          }}
          submit={{
            enterLabel: state.inlineEnterLabel,
            isDisabled: state.isSendDisabled,
            isGoalCreating: state.isGoalCreating,
            isGoalMode: state.isGoalMode,
            isPreparingAttachments: state.isPreparingAttachments,
            onSend: actions.handleSend,
            onStop: props.onStop,
            sendLabel: state.sendButtonLabel,
            shouldStop: state.shouldShowStopButton,
            stopLabel: t("composer.stop_generation"),
          }}
          textareaRef={refs.textareaRef}
        />

        <ComposerFooter
          actionButtonRef={refs.actionButtonRef}
          activeError={state.activeError}
          canCreateGoal={state.canCreateGoal}
          canUseLoop={state.canUseLoop}
          charCount={state.charCount}
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
          runtimeKind={props.runtimeKind}
        />
      </div>
    </section>
  );
});

ComposerPanelView.displayName = "ComposerPanelView";

export function ComposerPanel(props: ComposerPanelProps) {
  return <ComposerPanelView {...props} />;
}
