import type { PendingPermission } from "@/types/conversation/interaction/permission";
import type { Message } from "@/types/conversation/message/entity";

export interface ConversationThreadRound {
  messages: Message[];
  roundId: string;
}

export type ConversationThreadLayout = "desktop" | "mobile";
export type ConversationThreadNavigation = "auto" | "back" | "close";
export type ConversationThreadNavigationAction = "back" | "close" | null;

export interface ConversationThreadRoundModel extends ConversationThreadRound {
  isLast: boolean;
  isLoading: boolean;
  pendingPermissions: PendingPermission[];
  showDivider: boolean;
}

export interface ConversationThreadModel {
  allMessages: Message[];
  isMobile: boolean;
  leadingAction: ConversationThreadNavigationAction;
  rounds: ConversationThreadRoundModel[];
  sessionKey: string;
  trailingAction: ConversationThreadNavigationAction;
  workspaceAgentId: string;
}

interface ConversationThreadModelInput {
  agentId: string;
  isLoading: boolean;
  layout: ConversationThreadLayout;
  messages: Message[];
  navigation: ConversationThreadNavigation;
  pendingPermissions: PendingPermission[];
  roundId: string;
  rounds?: ConversationThreadRound[];
  sessionKey?: string;
  workspaceAgentId?: string | null;
}

interface ConversationThreadNavigationModel {
  leadingAction: ConversationThreadNavigationAction;
  trailingAction: ConversationThreadNavigationAction;
}

const THREAD_NAVIGATION_MODEL: Record<
  ConversationThreadLayout,
  Record<ConversationThreadNavigation, ConversationThreadNavigationModel>
> = {
  desktop: {
    auto: { leadingAction: null, trailingAction: "close" },
    back: { leadingAction: "back", trailingAction: null },
    close: { leadingAction: null, trailingAction: "close" },
  },
  mobile: {
    auto: { leadingAction: "back", trailingAction: null },
    back: { leadingAction: "back", trailingAction: null },
    close: { leadingAction: null, trailingAction: "close" },
  },
};

export function buildConversationThreadModel(
  input: ConversationThreadModelInput,
): ConversationThreadModel {
  const sourceRounds = input.rounds ?? [
    { messages: input.messages, roundId: input.roundId },
  ];
  const lastRoundIndex = sourceRounds.length - 1;
  const navigation = THREAD_NAVIGATION_MODEL[input.layout][input.navigation];

  return {
    allMessages: sourceRounds.flatMap((round) => round.messages),
    isMobile: input.layout === "mobile",
    leadingAction: navigation.leadingAction,
    rounds: sourceRounds.map((round, index) => {
      const isLast = index === lastRoundIndex;
      return {
        ...round,
        isLast,
        isLoading: isLast && input.isLoading,
        pendingPermissions: isLast ? input.pendingPermissions : [],
        showDivider: !isLast,
      };
    }),
    sessionKey: input.sessionKey ?? `${input.roundId}:${input.agentId}`,
    trailingAction: navigation.trailingAction,
    workspaceAgentId: input.workspaceAgentId ?? input.agentId,
  };
}
