"use client";

import { ChangeEvent, ClipboardEvent, useCallback, useState } from "react";

import {
  type I18nContextValue,
  useI18n,
} from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import type { MessageAttachment } from "@/types/conversation/message/attachment";

import {
  type ComposerAttachmentRejection,
  type ComposerAttachmentRejectionCode,
  ComposerAttachmentRejectedError,
} from "./composer-attachments";
import {
  appendLocalAttachments,
  buildLocalAttachmentBatch,
  buildPastedTextFile,
  type ComposerPasteAction,
  type ComposerPasteActionKind,
  ComposerLocalAttachment,
  projectComposerPasteAction,
} from "./composer-local-attachment-model";

interface UseComposerAttachmentsOptions {
  isGoalMode: boolean;
  onGoalAttachmentRejected: (message: string) => void;
  onPrepareAttachments: (files: File[]) => Promise<MessageAttachment[]>;
}

interface ComposerPasteActionContext {
  action: ComposerPasteAction;
  appendFiles: (files: File[]) => void;
  event: ClipboardEvent<HTMLTextAreaElement>;
  rejectGoalAttachment: () => void;
}

type ComposerPasteActionHandler = (
  context: ComposerPasteActionContext,
) => void;

const ATTACHMENT_REJECTION_MESSAGE_KEYS: Record<
  ComposerAttachmentRejectionCode,
  TranslationKey
> = {
  too_large: "composer.attachment_too_large",
  unsupported_format: "composer.attachment_format_unsupported",
};

const PASTE_ACTION_HANDLERS: Record<
  ComposerPasteActionKind,
  ComposerPasteActionHandler
> = {
  append_files: ({ action, appendFiles, event }) => {
    event.preventDefault();
    appendFiles(action.files);
  },
  append_text: ({ action, appendFiles, event }) => {
    event.preventDefault();
    appendFiles([buildPastedTextFile(action.text)]);
  },
  native: () => undefined,
  reject_goal: ({ event, rejectGoalAttachment }) => {
    event.preventDefault();
    rejectGoalAttachment();
  },
};

function formatAttachmentRejection(
  rejection: ComposerAttachmentRejection,
  translate: I18nContextValue["t"],
): string {
  return translate(ATTACHMENT_REJECTION_MESSAGE_KEYS[rejection.code], {
    name: rejection.fileName,
  });
}

function formatAttachmentPreparationError(
  error: unknown,
  translate: I18nContextValue["t"],
): string {
  if (error instanceof ComposerAttachmentRejectedError) {
    return formatAttachmentRejection(error.rejection, translate);
  }
  return error instanceof Error
    ? error.message
    : translate("composer.attachment_failed");
}

function formatFirstAttachmentRejection(
  rejection: ComposerAttachmentRejection | undefined,
  translate: I18nContextValue["t"],
): string | null {
  if (!rejection) {
    return null;
  }
  return formatAttachmentRejection(rejection, translate);
}

export function useComposerAttachments({
  isGoalMode,
  onGoalAttachmentRejected,
  onPrepareAttachments,
}: UseComposerAttachmentsOptions) {
  const { t } = useI18n();
  const [attachments, setAttachments] = useState<ComposerLocalAttachment[]>([]);
  const [attachmentError, setAttachmentError] = useState<string | null>(null);
  const [isPreparingAttachments, setIsPreparingAttachments] = useState(false);

  const clearAttachmentError = useCallback(() => {
    setAttachmentError(null);
  }, []);

  const clearAttachments = useCallback(() => {
    setAttachments([]);
  }, []);

  const appendAttachmentFiles = useCallback((files: File[]) => {
    if (files.length === 0) {
      return;
    }
    const batch = buildLocalAttachmentBatch(files);
    setAttachmentError(formatFirstAttachmentRejection(batch.rejections[0], t));
    setAttachments((current) => appendLocalAttachments(
      current,
      batch.attachments,
    ));
  }, [t]);

  const handleFileSelect = useCallback((event: ChangeEvent<HTMLInputElement>) => {
    const files = event.target.files;
    if (!files) {
      return;
    }

    appendAttachmentFiles(Array.from(files));
    event.currentTarget.value = "";
  }, [appendAttachmentFiles]);

  const handlePaste = useCallback((event: ClipboardEvent<HTMLTextAreaElement>) => {
    const action = projectComposerPasteAction(event.clipboardData, isGoalMode);
    PASTE_ACTION_HANDLERS[action.kind]({
      action,
      appendFiles: appendAttachmentFiles,
      event,
      rejectGoalAttachment: () => {
        onGoalAttachmentRejected(t("composer.goal_attachment_unsupported"));
      },
    });
  }, [
    appendAttachmentFiles,
    isGoalMode,
    onGoalAttachmentRejected,
    t,
  ]);

  const prepareAttachments = useCallback(async () => {
    if (attachments.length === 0) {
      return [] as MessageAttachment[];
    }
    setIsPreparingAttachments(true);
    setAttachmentError(null);
    try {
      return await onPrepareAttachments(attachments.map((attachment) => attachment.file));
    } catch (error) {
      setAttachmentError(formatAttachmentPreparationError(error, t));
      return null;
    } finally {
      setIsPreparingAttachments(false);
    }
  }, [
    attachments,
    onPrepareAttachments,
    t,
  ]);

  const removeAttachment = useCallback((id: string) => {
    setAttachments((prev) => prev.filter((item) => item.id !== id));
  }, []);

  return {
    attachmentError,
    attachments,
    clearAttachmentError,
    clearAttachments,
    handleFileSelect,
    handlePaste,
    isPreparingAttachments,
    prepareAttachments,
    removeAttachment,
  };
}
