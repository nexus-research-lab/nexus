"use client";

import {
  useCallback,
  useMemo,
  useState,
} from "react";
import type { Dispatch, RefObject, SetStateAction } from "react";

import type { Agent } from "@/types/agent/agent";

import type { MentionTargetItem } from "../mention-popover";

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
        subtitle: null,
        kind: "agent",
      })),
    [roomMembers],
  );

  const [mentionActive, setMentionActive] = useState(false);
  const [mentionFilter, setMentionFilter] = useState("");
  const [mentionStartPos, setMentionStartPos] = useState(-1);

  const closeMention = useCallback(() => {
    setMentionActive(false);
  }, []);

  const updateMentionForInput = useCallback((value: string) => {
    if (isGoalMode || roomMembers.length === 0) {
      setMentionActive(false);
      return;
    }

    const cursorPos = textareaRef.current?.selectionStart ?? value.length;
    const beforeCursor = value.slice(0, cursorPos);
    const atIndex = beforeCursor.lastIndexOf("@");

    if (atIndex >= 0) {
      const charBeforeAt = atIndex > 0 ? beforeCursor[atIndex - 1] : " ";
      if (charBeforeAt === " " || charBeforeAt === "\n" || atIndex === 0) {
        const filterText = beforeCursor.slice(atIndex + 1);
        if (!filterText.includes(" ")) {
          setMentionActive(true);
          setMentionFilter(filterText);
          setMentionStartPos(atIndex);
          return;
        }
      }
    }

    setMentionActive(false);
  }, [
    roomMembers.length,
    isGoalMode,
    textareaRef,
  ]);

  const selectMentionItem = useCallback((item: MentionTargetItem) => {
    const selectedMember = roomMembers.find((member) => member.agent_id === item.id);
    if (!selectedMember) {
      return;
    }

    const before = input.slice(0, mentionStartPos);
    const cursorPos = textareaRef.current?.selectionStart ?? input.length;
    const after = input.slice(cursorPos);
    const nextInput = `${before}@${selectedMember.name} ${after}`;
    setInput(nextInput);
    setMentionActive(false);

    requestAnimationFrame(() => {
      const newCursor = mentionStartPos + selectedMember.name.length + 2;
      textareaRef.current?.setSelectionRange(newCursor, newCursor);
      textareaRef.current?.focus();
    });
  }, [
    roomMembers,
    input,
    mentionStartPos,
    setInput,
    textareaRef,
  ]);

  return {
    closeMention,
    mentionActive,
    mentionFilter,
    mentionTargetItems,
    selectMentionItem,
    updateMentionForInput,
  };
}
