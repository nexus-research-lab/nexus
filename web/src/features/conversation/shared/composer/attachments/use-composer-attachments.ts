"use client";

import { ChangeEvent, ClipboardEvent, useCallback, useState } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";

import { PreparedComposerAttachment } from "./composer-attachments";
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
  onPrepareAttachments?: (files: File[]) => Promise<PreparedComposerAttachment[]>;
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
    const rejectedFiles: string[] = [];

    files.forEach((file) => {
      const { attachment, rejection_reason: rejectionReason } = buildLocalAttachment(
        file,
        t("composer.attachment_format_unsupported"),
      );
      if (rejectionReason) {
        rejectedFiles.push(rejectionReason);
        return;
      }
      if (attachment) {
        nextAttachments.push(attachment);
      }
    });

    if (rejectedFiles.length > 0) {
      setAttachmentError(rejectedFiles[0] ?? t("composer.attachment_format_unsupported"));
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
      return [] as PreparedComposerAttachment[];
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
      setAttachmentError(error instanceof Error ? error.message : t("composer.attachment_failed"));
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
