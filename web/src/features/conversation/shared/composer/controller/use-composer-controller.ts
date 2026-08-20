/**
 * INPUT: Composer 视图能力、Session 草稿作用域、聊天历史作用域及投递动作。
 * OUTPUT: 草稿、附件、键盘、发送与 textarea 焦点的统一视图模型。
 * POS: Shared Composer 的顶层交互编排入口。
 */
import { useCallback, useEffect, useLayoutEffect, useRef } from "react";

import { useTextareaHeight } from "@/hooks/ui/use-textarea-height";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { Agent } from "@/types/agent/agent";
import type { CommandCatalogData } from "@/types/generated/protocol";
import {
  WORKGRAPH_DISTILLATION_INTENT_EVENT,
  type WorkGraphDistillationIntentDetail,
} from "@/features/conversation/shared/execution/workgraph-distillation-intent";

import { useComposerAttachments } from "../attachments/use-composer-attachments";
import {
  focusComposerInputAtEnd,
  type ComposerPanelProps,
} from "../composer-model";
import { COMPOSER_TEXTAREA_MAX_HEIGHT_PX } from "../composer-styles";
import { useComposerHistory } from "../use-composer-history";
import { useComposerMention } from "../use-composer-mention";
import { useComposerSlashCommand } from "../use-composer-slash-command";
import { buildComposerViewState } from "./composer-controller-model";
import { useComposerDraft } from "./use-composer-draft";
import { useComposerGoalActions } from "./use-composer-goal-actions";
import { useComposerKeyboard } from "./use-composer-keyboard";
import { useComposerLocalDirectories } from "./use-composer-local-directories";
import { useComposerMessageSubmit } from "./use-composer-message-submit";
import { useComposerSessionSettings } from "./use-composer-session-settings";

const EMPTY_ROOM_MEMBERS: Agent[] = [];
const EMPTY_COMMAND_CATALOG: CommandCatalogData = {
  commands: [],
  status: "unavailable",
};

export function useComposerController({
  commandCatalog = EMPTY_COMMAND_CATALOG,
  compact,
  defaultDeliveryPolicy,
  draftScopeKey,
  enableLoops = false,
  goalCreateDisabledReason = null,
  historyScopeKey,
  inputQueueItems,
  interactionIdentity = null,
  isLoading,
  localDirectorySessionKey,
  onCreateGoal,
  onCreateLoopGoal,
  onEnqueueMessage,
  onPrepareAttachments,
  onSendMessage,
  onStop,
  queueWhenSessionBusy = true,
  roomMembers = EMPTY_ROOM_MEMBERS,
  runtimeKind,
  runtimePhase,
  sessionSettings,
}: ComposerPanelProps) {
  const { t } = useI18n();
  const sessionSettingsController = useComposerSessionSettings(
    sessionSettings,
  );
  const localDirectories = useComposerLocalDirectories(
    localDirectorySessionKey,
  );
  const draft = useComposerDraft(draftScopeKey);
  const {
    applyPrompt,
    setAttachments,
    setActionMenuOpen,
    setGoalError,
    setInput,
    setLoopPickerOpen,
    setSelectedTargetIDs,
    state: draftState,
  } = draft;
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const actionButtonRef = useRef<HTMLButtonElement>(null);
  const composerShellRef = useRef<HTMLDivElement>(null);
  const focusTextareaAtEnd = useCallback(() => {
    requestAnimationFrame(() => {
      const textarea = textareaRef.current;
      if (textarea) {
        focusComposerInputAtEnd(textarea);
      }
    });
  }, []);
  useEffect(() => {
    const handleIntent = (event: Event) => {
      const detail = (event as CustomEvent<WorkGraphDistillationIntentDetail>).detail;
      if (!detail || detail.sessionKey !== localDirectorySessionKey) {
        return;
      }
      applyPrompt(detail.prompt, "message");
      focusTextareaAtEnd();
    };
    window.addEventListener(WORKGRAPH_DISTILLATION_INTENT_EVENT, handleIntent);
    return () => window.removeEventListener(
      WORKGRAPH_DISTILLATION_INTENT_EVENT,
      handleIntent,
    );
  }, [applyPrompt, focusTextareaAtEnd, localDirectorySessionKey]);
  const isGoalMode = draftState.inputMode === "goal";
  const attachments = useComposerAttachments({
    attachments: draftState.attachments,
    isGoalMode,
    onGoalAttachmentRejected: setGoalError,
    onPrepareAttachments,
    setAttachments,
  });
  const {
    attachmentError,
    clearAttachmentError,
  } = attachments;
  const mention = useComposerMention({
    input: draftState.input,
    isGoalMode,
    roomMembers,
    selectedTargetIDs: draftState.selectedTargetIDs,
    setInput,
    setSelectedTargetIDs,
    textareaRef,
  });
  const slashCommand = useComposerSlashCommand({
    catalog: commandCatalog,
    input: draftState.input,
    isGoalMode,
    runtimeKind,
    setInput,
    textareaRef,
  });
  const { closeMention, updateMentionForInput } = mention;
  const {
    close: closeSlashCommand,
    updateForInput: updateSlashCommandForInput,
  } = slashCommand;
  useLayoutEffect(() => {
    if (!interactionIdentity) {
      return;
    }
    closeMention();
    closeSlashCommand();
    setActionMenuOpen(false);
    setLoopPickerOpen(false);
  }, [
    interactionIdentity,
    closeMention,
    closeSlashCommand,
    setActionMenuOpen,
    setLoopPickerOpen,
  ]);
  const history = useComposerHistory({
    clearError: clearAttachmentError,
    input: draftState.input,
    onRecall: focusTextareaAtEnd,
    scopeKey: historyScopeKey,
    setInput,
  });

  useTextareaHeight(textareaRef, draftState.input, {
    minHeight: 24,
    maxHeight: COMPOSER_TEXTAREA_MAX_HEIGHT_PX,
  });

  const focusTextarea = useCallback(() => {
    requestAnimationFrame(() => textareaRef.current?.focus());
  }, []);
  useLayoutEffect(() => {
    const textarea = textareaRef.current;
    if (textarea) {
      focusComposerInputAtEnd(textarea);
    }
  }, [draftScopeKey, interactionIdentity]);

  const resetTextareaHeight = useCallback(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = "0px";
    }
  }, []);
  const submitMessage = useComposerMessageSubmit({
    attachmentCount: attachments.attachments.length,
    claimDraftSubmission: draft.claimMessageSubmission,
    clearAttachmentError,
    defaultDeliveryPolicy,
    input: draftState.input,
    isLoading,
    isPreparingAttachments: attachments.isPreparingAttachments,
    onEnqueueMessage,
    onSendMessage,
    prepareAttachments: attachments.prepareAttachments,
    queueItemCount: inputQueueItems.length,
    queueWhenSessionBusy,
    recordHistory: history.record,
    resetTextareaHeight,
    restoreFailedDraftSubmission: draft.restoreFailedMessageSubmission,
    runtimePhase,
    targetAgentIDs: mention.selectedTargetIDs,
  });
  const goal = useComposerGoalActions({
    closeMention: mention.closeMention,
    draft,
    enableLoops,
    fallbackErrorMessage: t("composer.goal_create_failed"),
    focusTextarea,
    goalCreateDisabledReason,
    onCreateGoal,
    onCreateLoopGoal,
  });
  const { submitGoal } = goal;
  const handleSend = useCallback(async () => {
    if (sessionSettingsController.saving || localDirectories.saving) {
      return;
    }
    if (isGoalMode) {
      await submitGoal();
    } else {
      await submitMessage();
    }
  }, [
    isGoalMode,
    localDirectories.saving,
    sessionSettingsController.saving,
    submitGoal,
    submitMessage,
  ]);
  const keyboard = useComposerKeyboard({
    historyIndex: history.index,
    historyItemCount: history.itemCount,
    isLoading,
    mentionActive: mention.mentionActive,
    onSlashCommandKeyDown: slashCommand.handleCommandKeyDown,
    onSend: handleSend,
    onStop,
    recallNext: history.recallNext,
    recallPrevious: history.recallPrevious,
    slashCommandActive: slashCommand.isOpen,
  });

  const handleInputChange = useCallback((value: string) => {
    setInput(value);
    if (attachmentError) {
      clearAttachmentError();
    }
    if (draftState.goalError) {
      setGoalError(null);
    }
    updateMentionForInput(value);
    updateSlashCommandForInput(value);
  }, [
    attachmentError,
    clearAttachmentError,
    draftState.goalError,
    setGoalError,
    setInput,
    updateSlashCommandForInput,
    updateMentionForInput,
  ]);
  const openAttachmentPicker = useCallback(() => {
    setActionMenuOpen(false);
    fileInputRef.current?.click();
  }, [setActionMenuOpen]);
  const openLocalDirectoryPicker = useCallback(() => {
    setActionMenuOpen(false);
    void localDirectories.chooseDirectory();
  }, [localDirectories, setActionMenuOpen]);

  const state = buildComposerViewState({
    attachmentCount: attachments.attachments.length,
    attachmentError,
    canCreateGoal: goal.canCreateGoal,
    canUseLoop: goal.canUseLoop,
    compact,
    copy: {
      defaultPlaceholder: t("composer.default_placeholder"),
      goalConfirm: t("composer.goal_confirm"),
      goalPlaceholder: t("composer.goal_placeholder"),
      sendMessage: t("composer.send_message"),
    },
    goalCreateBlockedReason: goal.blockedReason,
    goalError: draftState.goalError,
    historyIndex: history.index,
    historyItemCount: history.itemCount,
    input: draftState.input,
    inputMode: draftState.inputMode,
    isActionMenuOpen: draftState.isActionMenuOpen,
    isGoalConfirming: draftState.isGoalConfirming,
    isGoalCreating: draftState.isGoalCreating,
    isLoading,
    isLoopPickerOpen: draftState.isLoopPickerOpen,
    isPreparingAttachments: attachments.isPreparingAttachments,
    isSessionSettingsSaving:
      sessionSettingsController.saving || localDirectories.saving,
    hasStopAction: Boolean(onStop),
    queueItemCount: inputQueueItems.length,
    runtimePhase,
  });

  return {
    refs: {
      actionButtonRef,
      composerShellRef,
      fileInputRef,
      textareaRef,
    },
    sessionSettings: sessionSettingsController,
    localDirectories,
    state,
    attachments: {
      attachments: attachments.attachments,
      handleFileSelect: attachments.handleFileSelect,
      handlePaste: attachments.handlePaste,
      removeAttachment: attachments.removeAttachment,
    },
    mention: {
      closeMention: mention.closeMention,
      mentionActive: mention.mentionActive,
      mentionFilter: mention.mentionFilter,
      mentionTargetItems: mention.mentionTargetItems,
      selectMentionItem: mention.selectMentionItem,
    },
    slashCommand: {
      active: slashCommand.isOpen,
      activeIndex: slashCommand.activeIndex,
      commands: slashCommand.commands,
      mode: slashCommand.mode,
      isOpen: slashCommand.isOpen,
      modelError: slashCommand.modelError,
      modelItems: slashCommand.modelItems,
      modelLoading: slashCommand.modelLoading,
      modelQuery: slashCommand.modelQuery,
      modelSearchRef: slashCommand.modelSearchRef,
      onModelQueryChange: slashCommand.setModelQuery,
      onModelQueryKeyDown: slashCommand.handleModelSearchKeyDown,
      onClose: slashCommand.close,
      onSelectModel: slashCommand.selectModel,
      onSelectCommand: slashCommand.selectCommand,
      onSelectSkill: slashCommand.selectSkill,
      onSkillQueryChange: slashCommand.setSkillQuery,
      onSkillQueryKeyDown: slashCommand.handleSkillSearchKeyDown,
      skillError: slashCommand.skillError,
      skillItems: slashCommand.skillItems,
      skillLoading: slashCommand.skillLoading,
      skillQuery: slashCommand.skillQuery,
      skillSearchRef: slashCommand.skillSearchRef,
      status: slashCommand.status,
    },
    actions: {
      cancelGoalInput: goal.cancelGoalInput,
      handleCompositionEnd: keyboard.handleCompositionEnd,
      handleCompositionStart: keyboard.handleCompositionStart,
      handleInputChange,
      handleKeyDown: keyboard.handleKeyDown,
      handleLoopSelect: goal.handleLoopSelect,
      handleSend,
      openAttachmentPicker,
      openLocalDirectoryPicker,
      openLoopPicker: goal.openLoopPicker,
      setIsActionMenuOpen: setActionMenuOpen,
      setIsLoopPickerOpen: setLoopPickerOpen,
      toggleGoalInput: goal.toggleGoalInput,
    },
  };
}
