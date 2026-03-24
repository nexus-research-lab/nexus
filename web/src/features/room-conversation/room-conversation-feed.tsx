import { RefObject } from "react";

import { MessageItem } from "@/components/message";
import { PendingPermission, PermissionDecisionPayload } from "@/types/permission";
import { Message } from "@/types/message";

interface RoomConversationFeedProps {
  bottomAnchorRef: RefObject<HTMLDivElement | null>;
  currentAgentName: string | null;
  isLastRoundPendingPermission: PendingPermission | null;
  isLoading: boolean;
  isMobileLayout: boolean;
  messageGroups: Map<string, Message[]>;
  onDeleteRound: (roundId: string) => Promise<void>;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse: (payload: PermissionDecisionPayload) => void;
  onRegenerateRound?: (roundId: string) => Promise<void>;
  roundIds: string[];
}

export function RoomConversationFeed({
  bottomAnchorRef,
  currentAgentName,
  isLastRoundPendingPermission,
  isLoading,
  isMobileLayout,
  messageGroups,
  onDeleteRound,
  onOpenWorkspaceFile,
  onPermissionResponse,
  onRegenerateRound,
  roundIds,
}: RoomConversationFeedProps) {
  return (
    <div className={isMobileLayout ? "space-y-4" : "mx-auto flex w-full max-w-[980px] flex-col gap-6 xl:gap-8"}>
      {roundIds.map((roundId, idx) => {
        const roundMessages = messageGroups.get(roundId) || [];
        const isLastRound = idx === roundIds.length - 1;

        return (
          <MessageItem
            key={roundId}
            currentAgentName={currentAgentName}
            roundId={roundId}
            messages={roundMessages}
            isLastRound={isLastRound}
            isLoading={isLoading}
            pendingPermission={isLastRound ? isLastRoundPendingPermission : null}
            onPermissionResponse={onPermissionResponse}
            onOpenWorkspaceFile={onOpenWorkspaceFile}
            onDelete={onDeleteRound}
            onRegenerate={isLastRound ? onRegenerateRound : undefined}
          />
        );
      })}
      <div ref={bottomAnchorRef} className="h-px w-full" />
    </div>
  );
}
