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
  is_goal_mode,
  on_goal_attachment_rejected,
  on_prepare_attachments,
}: UseComposerAttachmentsOptions) {
  const { t } = useI18n();
  const [attachments, setAttachments] = useState<ComposerLocalAttachment[]>([]);
  const [attachment_error, setAttachmentError] = useState<string | null>(null);
  const [is_preparing_attachments, setIsPreparingAttachments] = useState(false);

  const clear_attachment_error = useCallback(() => {
    setAttachmentError(null);
  }, []);

  const clear_attachments = useCallback(() => {
    setAttachments([]);
  }, []);

  const append_attachment_files = useCallback((files: File[]) => {
    if (files.length === 0) {
      return;
    }

    const next_attachments: ComposerLocalAttachment[] = [];
    const rejected_files: string[] = [];

    files.forEach((file) => {
      const { attachment, rejection_reason } = build_local_attachment(
        file,
        t("composer.attachment_format_unsupported"),
      );
      if (rejection_reason) {
        rejected_files.push(rejection_reason);
        return;
      }
      if (attachment) {
        next_attachments.push(attachment);
      }
    });

    if (rejected_files.length > 0) {
      setAttachmentError(rejected_files[0] ?? t("composer.attachment_format_unsupported"));
    } else {
      setAttachmentError(null);
    }

    if (next_attachments.length > 0) {
      setAttachments((prev) => [...prev, ...next_attachments].slice(0, MAX_COMPOSER_ATTACHMENTS));
    }
  }, [t]);

  const handle_file_select = useCallback((event: ChangeEvent<HTMLInputElement>) => {
    const files = event.target.files;
    if (!files) {
      return;
    }

    append_attachment_files(Array.from(files));
    event.currentTarget.value = "";
  }, [append_attachment_files]);

  const handle_paste = useCallback((event: ClipboardEvent<HTMLTextAreaElement>) => {
    const pasted_files = get_clipboard_files(event.clipboardData);
    if (pasted_files.length === 0) {
      const pasted_text = event.clipboardData.getData("text/plain");
      if (!is_goal_mode && pasted_text.length > PASTED_TEXT_ATTACHMENT_THRESHOLD) {
        event.preventDefault();
        append_attachment_files([build_pasted_text_file(pasted_text)]);
      }
      return;
    }

    event.preventDefault();
    if (is_goal_mode) {
      on_goal_attachment_rejected(t("composer.goal_attachment_unsupported"));
      return;
    }
    append_attachment_files(pasted_files);
  }, [
    append_attachment_files,
    is_goal_mode,
    on_goal_attachment_rejected,
    t,
  ]);

  const prepare_attachments = useCallback(async () => {
    if (attachments.length === 0) {
      return [] as PreparedComposerAttachment[];
    }
    if (!on_prepare_attachments) {
      setAttachmentError(t("composer.unsupported_attachment"));
      return null;
    }

    setIsPreparingAttachments(true);
    setAttachmentError(null);
    try {
      return await on_prepare_attachments(attachments.map((attachment) => attachment.file));
    } catch (error) {
      setAttachmentError(error instanceof Error ? error.message : t("composer.attachment_failed"));
      return null;
    } finally {
      setIsPreparingAttachments(false);
    }
  }, [
    attachments,
    on_prepare_attachments,
    t,
  ]);

  const remove_attachment = useCallback((id: string) => {
    setAttachments((prev) => prev.filter((item) => item.id !== id));
  }, []);

  return {
    attachment_error,
    attachments,
    clear_attachment_error,
    clear_attachments,
    handle_file_select,
    handle_paste,
    is_preparing_attachments,
    prepare_attachments,
    remove_attachment,
  };
}
