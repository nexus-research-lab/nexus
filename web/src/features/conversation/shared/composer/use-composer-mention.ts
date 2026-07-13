"use client";

import {
  useCallback,
  useMemo,
  useState,
} from "react";
import type { Dispatch, RefObject, SetStateAction } from "react";

import type { Agent } from "@/types/agent/agent";
import {
  findMentionTextMatch,
  insertMentionTarget,
  type MentionTargetItem,
  type MentionTextMatch,
} from "@/shared/ui/mention/mention-target-model";

const COMPOSER_MENTION_TRIGGERS = ["@"] as const;

interface UseComposerMentionOptions {
  input: string;
  isGoalMode: boolean;
  roomMembers: Agent[];
  setInput: Dispatch<SetStateAction<string>>;
  textareaRef: RefObject<HTMLTextAreaElement | null>;
}

export function useComposerMention({
  input,
  isGoalMode,
  roomMembers,
  setInput,
  textareaRef,
}: UseComposerMentionOptions) {
  const mentionTargetItems = useMemo(
    () =>
      roomMembers.map<MentionTargetItem>((member) => ({
        id: member.agent_id,
        label: member.name,
        marker: member.name.charAt(0).toUpperCase(),
        subtitle: null,
      })),
    [roomMembers],
  );

  const [mentionMatch, setMentionMatch] = useState<MentionTextMatch | null>(null);

  const closeMention = useCallback(() => {
    setMentionMatch(null);
  }, []);

  const updateMentionForInput = useCallback((value: string) => {
    if (isGoalMode || roomMembers.length === 0) {
      setMentionMatch(null);
      return;
    }
    const cursorPos = textareaRef.current?.selectionStart ?? value.length;
    setMentionMatch(findMentionTextMatch(
      value,
      cursorPos,
      COMPOSER_MENTION_TRIGGERS,
    ));
  }, [
    roomMembers.length,
    isGoalMode,
    textareaRef,
  ]);

  const selectMentionItem = useCallback((item: MentionTargetItem) => {
    if (!mentionMatch) {
      return;
    }
    const cursorPos = textareaRef.current?.selectionStart ?? input.length;
    const insertion = insertMentionTarget(input, cursorPos, mentionMatch, item.label);
    setInput(insertion.value);
    setMentionMatch(null);

    requestAnimationFrame(() => {
      textareaRef.current?.setSelectionRange(
        insertion.cursorPosition,
        insertion.cursorPosition,
      );
      textareaRef.current?.focus();
    });
  }, [
    input,
    mentionMatch,
    setInput,
    textareaRef,
  ]);

  return {
    closeMention,
    mentionActive: Boolean(mentionMatch),
    mentionFilter: mentionMatch?.filter ?? "",
    mentionTargetItems,
    selectMentionItem,
    updateMentionForInput,
  };
}
