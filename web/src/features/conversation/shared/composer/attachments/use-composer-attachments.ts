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
  buildLocalAttachment,
  buildPastedTextFile,
  ComposerLocalAttachment,
  getClipboardFiles,
  MAX_COMPOSER_ATTACHMENTS,
  PASTED_TEXT_ATTACHMENT_THRESHOLD,
} from "./composer-local-attachment-model";

interface UseComposerAttachmentsOptions {
  isGoalMode: boolean;
  onGoalAttachmentRejected: (message: string) => void;
  onPrepareAttachments?: (files: File[]) => Promise<MessageAttachment[]>;
}

const ATTACHMENT_REJECTION_MESSAGE_KEYS: Record<
  ComposerAttachmentRejectionCode,
  TranslationKey
> = {
  too_large: "composer.attachment_too_large",
  unsupported_format: "composer.attachment_format_unsupported",
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

    const nextAttachments: ComposerLocalAttachment[] = [];
    const rejections: ComposerAttachmentRejection[] = [];

    files.forEach((file) => {
      const { attachment, rejection } = buildLocalAttachment(file);
      if (rejection) {
        rejections.push(rejection);
        return;
      }
      if (attachment) {
        nextAttachments.push(attachment);
      }
    });

    const firstRejection = rejections[0];
    if (firstRejection) {
      setAttachmentError(formatAttachmentRejection(firstRejection, t));
    } else {
      setAttachmentError(null);
    }

    if (nextAttachments.length > 0) {
      setAttachments((prev) => [...prev, ...nextAttachments].slice(0, MAX_COMPOSER_ATTACHMENTS));
    }
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
    const pastedFiles = getClipboardFiles(event.clipboardData);
    if (pastedFiles.length === 0) {
      const pastedText = event.clipboardData.getData("text/plain");
      if (!isGoalMode && pastedText.length > PASTED_TEXT_ATTACHMENT_THRESHOLD) {
        event.preventDefault();
        appendAttachmentFiles([buildPastedTextFile(pastedText)]);
      }
      return;
    }

    event.preventDefault();
    if (isGoalMode) {
      onGoalAttachmentRejected(t("composer.goal_attachment_unsupported"));
      return;
    }
    appendAttachmentFiles(pastedFiles);
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
    if (!onPrepareAttachments) {
      setAttachmentError(t("composer.unsupported_attachment"));
      return null;
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
