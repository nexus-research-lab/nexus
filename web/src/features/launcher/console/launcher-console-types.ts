/**
 * INPUT: Launcher 目录投影、导航回调与可选页面级可靠性反馈。
 * OUTPUT: Console、Hero 和最近会话入口之间共享的窄类型合同。
 * POS: Launcher 展示层类型边界；不持有请求、导航或恢复状态。
 */
import type { FeedbackBannerProps } from "@/shared/ui/feedback/feedback-banner-contract";
import type { MentionTargetItem } from "@/shared/ui/mention/mention-target-model";
import type {
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
  feedback?: FeedbackBannerProps | null;
  initialQuery: string;
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

export interface LauncherMentionTarget extends MentionTargetItem {
  kind: "agent" | "room";
}

export interface HeroStageProps {
  currentAgentId: string | null;
  decorativeTokens: SpotlightToken[];
  mentionTargets: LauncherMentionTarget[];
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
