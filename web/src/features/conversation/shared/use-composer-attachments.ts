"use client";

import { ChangeEvent, ClipboardEvent, useCallback, useState } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";

import { PreparedComposerAttachment } from "./composer-attachments";
import {
  build_local_attachment,
  build_pasted_text_file,
  ComposerLocalAttachment,
  get_clipboard_files,
  MAX_COMPOSER_ATTACHMENTS,
  PASTED_TEXT_ATTACHMENT_THRESHOLD,
} from "./composer-local-attachment-model";

interface UseComposerAttachmentsOptions {
  is_goal_mode: boolean;
  on_goal_attachment_rejected: (message: string) => void;
  on_prepare_attachments?: (files: File[]) => Promise<PreparedComposerAttachment[]>;
}

export function useComposerAttachments({
  is_goal_mode: isGoalMode,
  on_goal_attachment_rejected: onGoalAttachmentRejected,
  on_prepare_attachments: onPrepareAttachments,
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
      const { attachment, rejection_reason: rejectionReason } = build_local_attachment(
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
    const pastedFiles = get_clipboard_files(event.clipboardData);
    if (pastedFiles.length === 0) {
      const pastedText = event.clipboardData.getData("text/plain");
      if (!isGoalMode && pastedText.length > PASTED_TEXT_ATTACHMENT_THRESHOLD) {
        event.preventDefault();
        appendAttachmentFiles([build_pasted_text_file(pastedText)]);
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
    attachment_error: attachmentError,
    attachments,
    clear_attachment_error: clearAttachmentError,
    clear_attachments: clearAttachments,
    handle_file_select: handleFileSelect,
    handle_paste: handlePaste,
    is_preparing_attachments: isPreparingAttachments,
    prepare_attachments: prepareAttachments,
    remove_attachment: removeAttachment,
  };
}
