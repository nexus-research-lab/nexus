/**
 * =====================================================
 * @File   ：launcher-console-types.ts
 * @Date   ：2026-04-16 16:22
 * @Author ：leemysw
 * 2026-04-16 16:22   Create
 * =====================================================
 */

import { MentionTargetItem } from "@/features/conversation/shared/mention-popover";
import { Agent } from "@/types/agent/agent";
import { SpotlightToken } from "@/types/app/launcher";
import { Conversation } from "@/types/conversation/conversation";
import { RoomAggregate } from "@/types/conversation/room";

export interface LauncherConsoleProps {
  agents: Agent[];
  conversations: Conversation[];
  rooms: RoomAggregate[];
  current_agent_id: string | null;
  on_open_main_agent_dm: (initial_prompt?: string) => void;
  on_select_agent: (agent_id: string) => void;
}

export interface RecentLauncherEntry {
  key: string;
  type: "dm" | "room";
  label: string;
  last_activity_at: number;
  agent_id?: string;
  room_id?: string;
  conversation_id?: string;
}

export interface LauncherMentionMatch {
  trigger: "@" | "#";
  filter: string;
  start_pos: number;
}

export interface HeroStageProps {
  current_agent_id: string | null;
  decorative_tokens: SpotlightToken[];
  mention_targets: MentionTargetItem[];
  on_enter_home: () => void;
  on_open_main_agent_dm: (initial_prompt?: string) => void;
  on_query_change: (value: string) => void;
  on_select_agent: (agent_id: string) => void;
  on_open_recent_entry: (entry: RecentLauncherEntry) => void;
  on_submit: (submitted_query: string) => boolean;
  query: string;
  recent_entries: RecentLauncherEntry[];
  is_query_loading: boolean;
}
