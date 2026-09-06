// INPUT: 当前会话标题与明确的重命名命令。
// OUTPUT: 可空编辑草稿、编辑输入焦点与退出编辑后的动作焦点恢复。
// POS: 历史标题编辑生命周期；不持有菜单开关或会话切换命令。

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
  const triggerRef = useRef<HTMLButtonElement>(null);
  const wasEditingRef = useRef(false);
  const isEditing = draft !== null;

  useEffect(() => {
    if (isEditing) {
      inputRef.current?.focus();
    } else if (wasEditingRef.current) {
      triggerRef.current?.focus();
    }
    wasEditingRef.current = isEditing;
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
    triggerRef,
    setDraft,
    start,
    cancel,
    confirm,
  };
}
