import { useCallback, useEffect, useRef, useState } from "react";

import { useTextareaHeight } from "@/hooks/ui/use-textarea-height";

export function useUserMessageEditor({
  compact,
  content,
  onSubmit,
}: {
  compact: boolean;
  content: string;
  onSubmit?: (content: string) => void;
}) {
  const [isEditing, setIsEditing] = useState(false);
  const [draftContent, setDraftContent] = useState(content);
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  const normalizedDraftContent = draftContent.trim();
  const canSubmit = Boolean(normalizedDraftContent) && normalizedDraftContent !== content;

  useEffect(() => {
    if (!isEditing) {
      setDraftContent(content);
    }
  }, [content, isEditing]);

  useEffect(() => {
    if (!isEditing) {
      return;
    }
    const textarea = textareaRef.current;
    textarea?.focus();
    textarea?.setSelectionRange(textarea.value.length, textarea.value.length);
  }, [isEditing]);

  useTextareaHeight(textareaRef, draftContent, {
    lineHeight: 24,
    maxHeight: 120,
    minHeight: compact ? 60 : 64,
    paddingY: compact ? 12 : 16,
  });

  const cancel = useCallback(() => {
    setDraftContent(content);
    setIsEditing(false);
  }, [content]);
  const submit = useCallback(() => {
    if (!onSubmit || !canSubmit) {
      cancel();
      return;
    }
    onSubmit(normalizedDraftContent);
    setIsEditing(false);
  }, [canSubmit, cancel, normalizedDraftContent, onSubmit]);

  return {
    canSubmit,
    cancel,
    draftContent,
    isEditing,
    setDraftContent,
    start: () => setIsEditing(true),
    submit,
    textareaRef,
  };
}
