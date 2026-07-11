"use client";

import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ChangeEvent,
  type KeyboardEvent,
  type SyntheticEvent,
} from "react";

import {
  findMentionTextMatch,
  insertMentionTarget,
  isMentionNavigationKey,
  type MentionTargetItem,
  type MentionTextMatch,
  type MentionTrigger,
} from "@/shared/ui/mention/mention-target-model";

import type { LauncherMentionTarget } from "../console/launcher-console-types";

interface UseLauncherQueryInputOptions {
  mentionTargets: LauncherMentionTarget[];
  onQueryChange: (value: string) => void;
  onSubmit: (submittedQuery: string) => boolean;
  query: string;
}

const LAUNCHER_MENTION_TRIGGERS = ["@", "#"] as const;
const TARGET_KIND_BY_TRIGGER: Readonly<Record<MentionTrigger, LauncherMentionTarget["kind"]>> = {
  "@": "agent",
  "#": "room",
};

export function useLauncherQueryInput({
  mentionTargets,
  onQueryChange,
  onSubmit,
  query,
}: UseLauncherQueryInputOptions) {
  const inputRef = useRef<HTMLInputElement>(null);
  const composingRef = useRef(false);
  const [value, setValue] = useState(query);
  const [mentionMatch, setMentionMatch] = useState<MentionTextMatch | null>(null);

  const visibleMentionTargets = useMemo(() => {
    if (!mentionMatch) {
      return [];
    }
    const targetKind = TARGET_KIND_BY_TRIGGER[mentionMatch.trigger];
    return mentionTargets.filter((item) => item.kind === targetKind);
  }, [mentionMatch, mentionTargets]);

  const syncMentionMatch = useCallback((nextValue: string, cursorPosition: number) => {
    setMentionMatch(findMentionTextMatch(
      nextValue,
      cursorPosition,
      LAUNCHER_MENTION_TRIGGERS,
    ));
  }, []);
  const closeMention = useCallback(() => setMentionMatch(null), []);

  const handleChange = useCallback((event: ChangeEvent<HTMLInputElement>) => {
    const nextValue = event.target.value;
    setValue(nextValue);
    onQueryChange(nextValue);
    syncMentionMatch(
      nextValue,
      inputRef.current?.selectionStart ?? nextValue.length,
    );
  }, [onQueryChange, syncMentionMatch]);

  const selectMention = useCallback((item: MentionTargetItem) => {
    if (!mentionMatch) {
      return;
    }
    const insertion = insertMentionTarget(
      value,
      inputRef.current?.selectionStart ?? value.length,
      mentionMatch,
      item.label,
    );
    setValue(insertion.value);
    onQueryChange(insertion.value);
    setMentionMatch(null);
    requestAnimationFrame(() => {
      inputRef.current?.setSelectionRange(
        insertion.cursorPosition,
        insertion.cursorPosition,
      );
      inputRef.current?.focus();
    });
  }, [mentionMatch, onQueryChange, value]);

  const submit = useCallback(() => {
    const submittedQuery = value.trim();
    if (!submittedQuery || !onSubmit(submittedQuery)) {
      return;
    }
    setValue("");
    onQueryChange("");
    setMentionMatch(null);
  }, [onQueryChange, onSubmit, value]);

  const handleKeyDown = useCallback((event: KeyboardEvent<HTMLInputElement>) => {
    if (composingRef.current || event.nativeEvent.isComposing) {
      return;
    }
    if (
      mentionMatch
      && visibleMentionTargets.length > 0
      && isMentionNavigationKey(event.key)
    ) {
      return;
    }
    if (event.key === "Enter") {
      event.preventDefault();
      submit();
    }
  }, [mentionMatch, submit, visibleMentionTargets.length]);

  const handleSelect = useCallback((event: SyntheticEvent<HTMLInputElement>) => {
    const target = event.currentTarget;
    syncMentionMatch(
      target.value,
      target.selectionStart ?? target.value.length,
    );
  }, [syncMentionMatch]);

  const handleBlur = useCallback(() => {
    requestAnimationFrame(() => {
      if (document.activeElement !== inputRef.current) {
        setMentionMatch(null);
      }
    });
  }, []);

  useEffect(() => {
    setValue(query);
    if (!query) {
      setMentionMatch(null);
    }
  }, [query]);

  return {
    input: {
      onBlur: handleBlur,
      onChange: handleChange,
      onCompositionEnd: () => {
        composingRef.current = false;
      },
      onCompositionStart: () => {
        composingRef.current = true;
      },
      onKeyDown: handleKeyDown,
      onSelect: handleSelect,
      ref: inputRef,
      value,
    },
    mention: {
      close: closeMention,
      match: mentionMatch,
      select: selectMention,
      targets: visibleMentionTargets,
    },
    submit,
  };
}
