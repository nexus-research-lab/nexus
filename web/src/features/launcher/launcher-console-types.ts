import { MentionTargetItem } from "@/features/conversation/shared/mention-popover";
import {
  LauncherAgentSummary,
  LauncherConversationSummary,
  LauncherRoomSummary,
  SpotlightToken,
} from "@/types/app/launcher";

export interface LauncherConsoleProps {
  agents: LauncherAgentSummary[];
  rooms: LauncherRoomSummary[];
  conversations: LauncherConversationSummary[];
  currentAgentId: string | null;
  onOpenMainAgentDm: (initialPrompt?: string) => void;
  onOpenRoute: (route: string) => void;
  onSelectAgent: (agentId: string) => void;
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
  currentAgentId: string | null;
  decorativeTokens: SpotlightToken[];
  mentionTargets: MentionTargetItem[];
  onEnterHome: () => void;
  onOpenMainAgentDm: (initialPrompt?: string) => void;
  onQueryChange: (value: string) => void;
  onSelectAgent: (agentId: string) => void;
  onOpenRecentEntry: (entry: RecentLauncherEntry) => void;
  onSubmit: (submittedQuery: string) => boolean;
  query: string;
  recentEntries: RecentLauncherEntry[];
  isQueryLoading: boolean;
}
