"use client";

import {
  Dispatch,
  RefObject,
  SetStateAction,
  useCallback,
  useMemo,
  useState,
} from "react";

import { Agent } from "@/types/agent/agent";

import { MentionTargetItem } from "./mention-popover";

interface UseComposerMentionOptions {
  input: string;
  isGoalMode: boolean;
  mentionUnavailableAgentIds: string[];
  roomMembers: Agent[];
  setInput: Dispatch<SetStateAction<string>>;
  textareaRef: RefObject<HTMLTextAreaElement | null>;
}

export function useComposerMention({
  input,
  isGoalMode,
  mentionUnavailableAgentIds,
  roomMembers,
  setInput,
  textareaRef,
}: UseComposerMentionOptions) {
  const availableRoomMembers = useMemo(() => {
    const unavailableIds = new Set(mentionUnavailableAgentIds);
    return roomMembers.filter(
      (member) => !unavailableIds.has(member.agent_id),
    );
  }, [mentionUnavailableAgentIds, roomMembers]);

  const mentionTargetItems = useMemo(
    () =>
      availableRoomMembers.map<MentionTargetItem>((member) => ({
        id: member.agent_id,
        label: member.name,
        subtitle: null,
        kind: "agent",
      })),
    [availableRoomMembers],
  );

  const [mentionActive, setMentionActive] = useState(false);
  const [mentionFilter, setMentionFilter] = useState("");
  const [mentionStartPos, setMentionStartPos] = useState(-1);

  const closeMention = useCallback(() => {
    setMentionActive(false);
  }, []);

  const updateMentionForInput = useCallback((value: string) => {
    if (isGoalMode || availableRoomMembers.length === 0) {
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
    availableRoomMembers.length,
    isGoalMode,
    textareaRef,
  ]);

  const selectMentionItem = useCallback((item: MentionTargetItem) => {
    const selectedMember = availableRoomMembers.find((member) => member.agent_id === item.id);
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
    availableRoomMembers,
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
