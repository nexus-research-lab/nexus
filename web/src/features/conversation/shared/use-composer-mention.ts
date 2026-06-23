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
  is_goal_mode: boolean;
  mention_unavailable_agent_ids: string[];
  room_members: Agent[];
  set_input: Dispatch<SetStateAction<string>>;
  textarea_ref: RefObject<HTMLTextAreaElement | null>;
}

export function useComposerMention({
  input,
  is_goal_mode,
  mention_unavailable_agent_ids,
  room_members,
  set_input,
  textarea_ref,
}: UseComposerMentionOptions) {
  const available_room_members = useMemo(() => {
    const unavailable_ids = new Set(mention_unavailable_agent_ids);
    return room_members.filter(
      (member) => !unavailable_ids.has(member.agent_id),
    );
  }, [mention_unavailable_agent_ids, room_members]);

  const mention_target_items = useMemo(
    () =>
      available_room_members.map<MentionTargetItem>((member) => ({
        id: member.agent_id,
        label: member.name,
        subtitle: null,
        kind: "agent",
      })),
    [available_room_members],
  );

  const [mention_active, set_mention_active] = useState(false);
  const [mention_filter, set_mention_filter] = useState("");
  const [mention_start_pos, set_mention_start_pos] = useState(-1);

  const close_mention = useCallback(() => {
    set_mention_active(false);
  }, []);

  const update_mention_for_input = useCallback((value: string) => {
    if (is_goal_mode || available_room_members.length === 0) {
      set_mention_active(false);
      return;
    }

    const cursor_pos = textarea_ref.current?.selectionStart ?? value.length;
    const before_cursor = value.slice(0, cursor_pos);
    const at_index = before_cursor.lastIndexOf("@");

    if (at_index >= 0) {
      const char_before_at = at_index > 0 ? before_cursor[at_index - 1] : " ";
      if (char_before_at === " " || char_before_at === "\n" || at_index === 0) {
        const filter_text = before_cursor.slice(at_index + 1);
        if (!filter_text.includes(" ")) {
          set_mention_active(true);
          set_mention_filter(filter_text);
          set_mention_start_pos(at_index);
          return;
        }
      }
    }

    set_mention_active(false);
  }, [
    available_room_members.length,
    is_goal_mode,
    textarea_ref,
  ]);

  const select_mention_item = useCallback((item: MentionTargetItem) => {
    const selected_member = available_room_members.find((member) => member.agent_id === item.id);
    if (!selected_member) {
      return;
    }

    const before = input.slice(0, mention_start_pos);
    const cursor_pos = textarea_ref.current?.selectionStart ?? input.length;
    const after = input.slice(cursor_pos);
    const next_input = `${before}@${selected_member.name} ${after}`;
    set_input(next_input);
    set_mention_active(false);

    requestAnimationFrame(() => {
      const new_cursor = mention_start_pos + selected_member.name.length + 2;
      textarea_ref.current?.setSelectionRange(new_cursor, new_cursor);
      textarea_ref.current?.focus();
    });
  }, [
    available_room_members,
    input,
    mention_start_pos,
    set_input,
    textarea_ref,
  ]);

  return {
    close_mention,
    mention_active,
    mention_filter,
    mention_target_items,
    select_mention_item,
    update_mention_for_input,
  };
}
