import { useCallback, useEffect, useRef, useState, type MouseEvent } from "react";

interface UseConversationTitleEditorOptions {
  title: string;
  onRename: (title: string) => void;
}

export function useConversationTitleEditor({
  title,
  onRename,
}: UseConversationTitleEditorOptions) {
  const [draft, setDraft] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const isEditing = draft !== null;

  useEffect(() => {
    if (isEditing) {
      inputRef.current?.focus();
    }
  }, [isEditing]);

  const start = useCallback((event: MouseEvent) => {
    event.stopPropagation();
    setDraft(title.trim());
  }, [title]);
  const cancel = useCallback(() => setDraft(null), []);
  const confirm = useCallback(() => {
    const nextTitle = draft?.trim() ?? "";
    if (nextTitle && nextTitle !== title.trim()) {
      onRename(nextTitle);
    }
    setDraft(null);
  }, [draft, onRename, title]);

  return {
    draft: draft ?? "",
    isEditing,
    inputRef,
    setDraft,
    start,
    cancel,
    confirm,
  };
}
