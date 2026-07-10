"use client";

import { memo } from "react";
import { Send, StopCircle, Target } from "lucide-react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { LoadingOrb } from "@/shared/ui/feedback/loading-orb";

import { COMPOSER_ATTACHMENT_ACCEPT } from "./attachments/composer-attachments";
import { ComposerAttachmentList } from "./attachments/composer-local-attachments";
import { ComposerFooter } from "./components/composer-footer";
import { ComposerPendingQueue } from "./components/composer-pending-queue";
import { LoopPickerDialog } from "./components/loop-picker-dialog";
import {
  COMPOSER_SHORTCUT_KEY_CLASS_NAME,
  MAX_COMPOSER_INPUT_LENGTH,
  type ComposerPanelProps,
} from "./composer-model";
import {
  COMPOSER_DANGER_ACTION_BUTTON_CLASS_NAME,
  COMPOSER_PRIMARY_ACTION_BUTTON_CLASS_NAME,
  COMPOSER_SHELL_CLASS_NAME,
} from "./composer-styles";
import { useComposerController } from "./controller/use-composer-controller";
import { MentionTargetPopover } from "../mention-popover";

const ComposerPanelView = memo((props: ComposerPanelProps) => {
  const { t } = useI18n();
  const { actions, attachments, mention, refs, state } =
    useComposerController(props);
  const inputQueueItems = props.inputQueueItems ?? [];

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
          inputQueueItems={inputQueueItems}
          onDeleteQueuedMessage={props.onDeleteQueuedMessage}
          onGuideQueuedMessage={props.onGuideQueuedMessage}
          onReorderQueueMessages={props.onReorderQueueMessages}
        />

        <ComposerAttachmentList
          attachments={attachments.attachments}
          onRemove={attachments.removeAttachment}
          removeLabel={t("composer.remove_attachment")}
        />

        <div className={cn(
          "flex items-end gap-2",
          state.composerInputRowPaddingClass,
        )}>
          {mention.mentionActive && mention.mentionTargetItems.length > 0 ? (
            <MentionTargetPopover
              anchorRect={refs.textareaRef.current?.getBoundingClientRect() ?? null}
              filter={mention.mentionFilter}
              items={mention.mentionTargetItems}
              onClose={mention.closeMention}
              onSelect={mention.selectMentionItem}
              placement="above"
            />
          ) : null}

          <div className="relative min-w-0 flex-1">
            <textarea
              aria-label={t("composer.default_placeholder")}
              ref={refs.textareaRef}
              className={cn(
                "multiline-cursor soft-scrollbar min-h-6 w-full min-w-0 max-h-[200px] resize-none overflow-y-auto overscroll-contain bg-transparent text-[14px] leading-6 text-(--text-strong) outline-none shadow-none ring-0",
                "placeholder:text-(--text-soft)",
                "disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)",
                "focus:border-0 focus:bg-transparent focus:outline-none focus:ring-0 focus-visible:outline-none focus-visible:ring-0 focus-visible:shadow-none",
                state.shouldShowInlineShortcuts && "min-[760px]:pr-[210px]",
              )}
              disabled={state.isTextareaLocked}
              onChange={(event) => actions.handleInputChange(event.target.value)}
              onWheel={(event) => {
                const target = event.currentTarget;
                if (target.scrollHeight > target.clientHeight) {
                  event.stopPropagation();
                }
              }}
              onCompositionEnd={(event) => {
                actions.handleCompositionEnd(event.timeStamp);
              }}
              onCompositionStart={actions.handleCompositionStart}
              onKeyDown={actions.handleKeyDown}
              onPaste={attachments.handlePaste}
              placeholder={state.resolvedPlaceholder}
              rows={1}
              value={state.input}
            />
            {state.shouldShowInlineShortcuts ? (
              <div
                aria-hidden="true"
                className="pointer-events-none absolute right-0 top-1/2 hidden -translate-y-1/2 items-center gap-1.5 text-[11px] leading-none text-(--text-soft) min-[760px]:flex"
              >
                <span className="inline-flex items-center gap-1">
                  <kbd className={COMPOSER_SHORTCUT_KEY_CLASS_NAME}>Enter</kbd>
                  <span>{state.inlineEnterLabel}</span>
                </span>
                <span className="text-(--text-faint)">·</span>
                <span className="inline-flex items-center gap-1">
                  <kbd className={COMPOSER_SHORTCUT_KEY_CLASS_NAME}>Shift</kbd>
                  <span className="text-(--text-faint)">+</span>
                  <kbd className={COMPOSER_SHORTCUT_KEY_CLASS_NAME}>Enter</kbd>
                  <span>{t("composer.shift_enter_newline")}</span>
                </span>
              </div>
            ) : null}
          </div>

          {state.shouldShowStopButton ? (
            <button
              aria-label={t("composer.stop_generation")}
              className={COMPOSER_DANGER_ACTION_BUTTON_CLASS_NAME}
              onClick={props.onStop}
              type="button"
            >
              <StopCircle size={16} />
            </button>
          ) : (
            <button
              aria-label={state.sendButtonLabel}
              className={cn(
                COMPOSER_PRIMARY_ACTION_BUTTON_CLASS_NAME,
                "gap-1.5 min-[760px]:w-auto min-[760px]:px-3",
              )}
              disabled={state.isSendDisabled}
              onClick={() => void actions.handleSend()}
              type="button"
            >
              <span className="hidden text-[12px] font-semibold min-[760px]:inline">
                {state.inlineEnterLabel}
              </span>
              {state.isPreparingAttachments || state.isGoalCreating ? (
                <LoadingOrb frames={["·", "◦", "•", "◦"]} />
              ) : state.isGoalMode ? (
                <Target size={16} />
              ) : (
                <Send size={16} />
              )}
            </button>
          )}
        </div>

        <ComposerFooter
          actionButtonRef={refs.actionButtonRef}
          activeError={state.activeError}
          canCreateGoal={state.canCreateGoal}
          canUseLoop={state.canUseLoop}
          canStopGeneration={state.canStopGeneration}
          charCount={state.charCount}
          goalModeExtra={props.goalModeExtra ?? null}
          goalScopeLabel={props.goalScopeLabel ?? "会话 Goal"}
          historyIndex={state.historyIndex}
          inputHistoryLength={state.inputHistoryLength}
          isActionMenuOpen={state.isActionMenuOpen}
          isDispatching={state.isDispatching}
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
        />
      </div>
    </section>
  );
});

ComposerPanelView.displayName = "ComposerPanelView";

export function ComposerPanel(props: ComposerPanelProps) {
  return <ComposerPanelView {...props} />;
}
