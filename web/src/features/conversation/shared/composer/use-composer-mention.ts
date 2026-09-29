"use client";

import {
  useCallback,
  useMemo,
  useState,
} from "react";
import type { Dispatch, RefObject, SetStateAction } from "react";

import type { Agent } from "@/types/agent/agent";
import { useComposerDraftStore } from "./composer-draft-store";
import {
  findMentionTextMatch,
  insertMentionTarget,
  type MentionTargetItem,
  type MentionTextMatch,
} from "@/shared/ui/mention/mention-target-model";

const COMPOSER_MENTION_TRIGGERS = ["@"] as const;

interface UseComposerMentionOptions {
  draftScopeKey: string;
  input: string;
  isGoalMode: boolean;
  roomMembers: Pick<Agent, "agent_id" | "name" | "avatar">[];
  selectedTargetIDs: string[];
  setInput: Dispatch<SetStateAction<string>>;
  setSelectedTargetIDs: Dispatch<SetStateAction<string[]>>;
  textareaRef: RefObject<HTMLTextAreaElement | null>;
}

export function useComposerMention({
  draftScopeKey,
  input,
  isGoalMode,
  roomMembers,
  selectedTargetIDs,
  setInput,
  setSelectedTargetIDs,
  textareaRef,
}: UseComposerMentionOptions) {
  const mentionTargetItems = useMemo(
    () =>
      roomMembers.map<MentionTargetItem>((member) => ({
        id: member.agent_id,
        label: member.name,
        marker: member.name.charAt(0).toUpperCase(),
        avatar: member.avatar,
        subtitle: null,
      })),
    [roomMembers],
  );

  const [mentionMatch, setMentionMatch] = useState<MentionTextMatch | null>(null);
  const selectedNames = useComposerDraftStore((state) => state.drafts_by_scope[draftScopeKey]?.selectedTargetNames);
  const activeSelectedTargetIDs = useMemo(
    () => selectedTargetIDs.filter((agentID) => {
      // 成员目录刷新不能把已选择的 @ 目标静默降级为普通群消息。
      const label = selectedNames?.[agentID] ?? mentionTargetItems.find((item) => item.id === agentID)?.label;
      return label ? hasComposerMention(input, label) : false;
    }),
    [input, mentionTargetItems, selectedNames, selectedTargetIDs],
  );

  const mentionSegments = useMemo(() => splitComposerMentions(input, selectedTargetIDs.map(
    (id) => selectedNames?.[id] ?? mentionTargetItems.find((item) => item.id === id)?.label ?? "",
  )), [input, mentionTargetItems, selectedNames, selectedTargetIDs]);

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
    useComposerDraftStore.getState().update_composer_draft(draftScopeKey, (current) => ({ ...current, selectedTargetNames: { ...current.selectedTargetNames, [item.id]: item.label } }));
    setSelectedTargetIDs((current) => current.includes(item.id) ? current : [...current, item.id]);
    setMentionMatch(null);

    requestAnimationFrame(() => {
      // 用户已经继续输入时，不用上一帧的光标位置打断新内容。
      if (textareaRef.current?.value !== insertion.value) return;
      textareaRef.current?.setSelectionRange(
        insertion.cursorPosition,
        insertion.cursorPosition,
      );
      textareaRef.current?.focus();
    });
  }, [
    draftScopeKey,
    input,
    mentionMatch,
    setInput,
    setSelectedTargetIDs,
    textareaRef,
  ]);

  return {
    closeMention,
    mentionActive: Boolean(mentionMatch),
    mentionFilter: mentionMatch?.filter ?? "",
    mentionTargetItems,
    mentionSegments,
    selectedTargetIDs: activeSelectedTargetIDs,
    selectMentionItem,
    updateMentionForInput,
  };
}

export interface ComposerMentionSegment {
  text: string;
  mentioned: boolean;
}

/** 只装饰已选择目标的完整名称，保留原文长度，避免镜像与原生光标错位。 */
export function splitComposerMentions(input: string, labels: readonly string[]): ComposerMentionSegment[] {
  const names = [...new Set(labels.filter(Boolean))].sort((a, b) => b.length - a.length);
  if (!names.length) return [{ text: input, mentioned: false }];
  const escaped = names.map((name) => name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")).join("|");
  const pattern = new RegExp(`(?:^|\\s)(@(?:${escaped}))(?=$|\\s|[，。！？、,.!?;:：；])`, "giu");
  const segments: ComposerMentionSegment[] = [];
  let end = 0;
  for (const match of input.matchAll(pattern)) {
    const start = match.index + match[0].length - match[1].length;
    segments.push({ text: input.slice(end, start), mentioned: false });
    segments.push({ text: match[1], mentioned: true });
    end = start + match[1].length;
  }
  segments.push({ text: input.slice(end), mentioned: false });
  return segments;
}

function hasComposerMention(input: string, label: string): boolean {
  return splitComposerMentions(input, [label]).some((segment) => segment.mentioned);
}
