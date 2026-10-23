// INPUT: 在线 Room 消息快照、投递状态、本机执行任务与成员/Agent 目录。
// OUTPUT: 复用 Room 共享消息卡片（MessageUserSection / RoomAgentResultCard）的在线消息轨道。
// POS: Team Room 消息展示与本机/远端投递状态投影层；不持有发送或同步事务。

import { Fragment, useCallback, useEffect, useRef, useState, type ReactNode } from "react";

import { RoomAgentResultCard } from "@/features/conversation/room/group/thread/round-card/group-agent-execution-shell";
import { ThreadActionButton } from "@/features/conversation/room/group/thread/round-card/thread-action-button";
import { CONVERSATION_CONTENT_LANE_CLASS_NAME } from "@/features/conversation/shared/conversation-panel-styles";
import type { AgentMentionDirectory } from "@/features/conversation/shared/message/agent-mention-chip";
import {
  MessageActivityStatus,
  ROOM_RESULT_ACTIVITY_ALIGNMENT_CLASS_NAME,
} from "@/features/conversation/shared/message/item/view/message-activity-status";
import { MessageUserAttachments } from "@/features/conversation/shared/message/item/view/user/message-user-attachments";
import { MessageUserSection } from "@/features/conversation/shared/message/item/view/user/message-user-section";
import { formatMessageTime } from "@/features/conversation/shared/message/message-time";
import type {
  ControlAgentDirectoryEntry,
  ControlMemberDirectoryEntry,
} from "@/lib/api/account/control-api";
import {
  cancelTeamDelivery,
  type TeamDeliveryStatus,
  type TeamMessage,
} from "@/lib/api/conversation/team-api";
import { saveTeamFile } from "@/lib/api/conversation/team-files-api";
import type { TeamNodeJob } from "@/lib/api/conversation/team-node-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { MessageAttachment } from "@/types/conversation/message/attachment";
import type { AgentMention } from "@/types/conversation/message/entity";

import { TeamInvitationList } from "./team-invitation-list";
import { useTeamInvitations } from "./use-team-invitations";

interface TeamMessageFeedProps {
  agentDirectory: ControlAgentDirectoryEntry[];
  currentUserAvatar?: string | null;
  currentUserId: string | null;
  deliveries: TeamDeliveryStatus[];
  directUserId?: string;
  isCompact: boolean;
  isLoading: boolean;
  jobs: TeamNodeJob[];
  loadFailed: boolean;
  memberDirectory: ControlMemberDirectoryEntry[];
  messages: TeamMessage[];
  onOpenThread: (job: TeamNodeJob) => void;
  roomId: string;
  selectedThreadID?: string;
  stopAction: (job: TeamNodeJob) => ReactNode;
}

export function TeamMessageFeed({
  agentDirectory,
  currentUserAvatar,
  currentUserId,
  deliveries,
  directUserId,
  isCompact,
  isLoading,
  jobs,
  loadFailed,
  memberDirectory,
  messages,
  onOpenThread,
  roomId,
  selectedThreadID,
  stopAction,
}: TeamMessageFeedProps) {
  const { t } = useI18n();
  const onAccepted = useCallback(() => {
    window.dispatchEvent(new Event("focus"));
  }, []);
  const invitations = useTeamInvitations(onAccepted, Boolean(directUserId));
  const pending = invitations.invitations.filter(
    (item) => item.invited_by_user_id === directUserId,
  );
  if (loadFailed && !isLoading && messages.length === 0) return null;
  if (messages.length === 0 && pending.length === 0) {
    if (!isLoading) return null;
    return (
      <div
        role="status"
        className={`${CONVERSATION_CONTENT_LANE_CLASS_NAME} flex h-full min-h-64 items-center justify-center ${getUiTypographyClassName({ role: "supporting", tone: "muted" })}`}
      >
        {t("team.loading")}
      </div>
    );
  }
  const agentsByID = new Map(
    agentDirectory.map((agent) => [agent.agent_id, agent]),
  );
  const messagesByID = new Map(
    messages.map((message) => [message.id, message]),
  );
  const sourcesByDelivery = new Map(
    deliveries.map((delivery) => [
      delivery.id,
      messagesByID.get(delivery.message_id),
    ]),
  );
  for (const [id, source] of jobs
    .filter((job) => job.delivery_id && job.source_message_id)
    .map(
      (job) =>
        [job.delivery_id, messagesByID.get(job.source_message_id!)] as const,
    )) {
    if (id) sourcesByDelivery.set(id, source);
  }
  const replyTarget = (source?: TeamMessage) => {
    if (!source) return undefined;
    const agent =
      source.author_type === "agent"
        ? agentsByID.get(source.author_agent_id ?? "")
        : undefined;
    const person =
      source.author_type === "user"
        ? memberDirectory.find(
            (member) => member.user_id === source.author_user_id,
          )
        : undefined;
    return {
      name:
        agent?.name ||
        person?.display_name ||
        person?.username ||
        source.author_display_name ||
        source.author_username ||
        "?",
      avatar:
        agent?.avatar ||
        person?.avatar ||
        (source.author_type === "user" &&
        source.author_user_id === currentUserId
          ? currentUserAvatar
          : undefined),
      message: source.content.blocks.map((block) => block.text).join("\n"),
    };
  };
  const mentionDirectory: AgentMentionDirectory = {
    names: Object.fromEntries(
      agentDirectory.map((agent) => [agent.agent_id, agent.name]),
    ),
    avatars: Object.fromEntries(
      agentDirectory.map((agent) => [agent.agent_id, agent.avatar ?? null]),
    ),
  };

  return (
    <ol
      aria-busy={isLoading || undefined}
      className={`nexus-chat-feed ${CONVERSATION_CONTENT_LANE_CLASS_NAME} flex flex-col`}
    >
      {messages.map((message) =>
        message.content.blocks.some((block) => block.type === "room_invitation") ? (
          <li key={message.id} className="px-3 py-2 text-sm text-(--text-muted)">
            {message.content.blocks
              .filter((block) => block.type === "room_invitation")
              .map((block, index) => {
                const pendingInvitation = pending.find(
                  (item) =>
                    item.room.id === block.room_id &&
                    new Date(item.created_at).getTime() ===
                      new Date(block.invited_at ?? "").getTime(),
                );
                return pendingInvitation ? (
                  <TeamInvitationList
                    key={index}
                    {...invitations}
                    invitations={[pendingInvitation]}
                    recoveryRooms={[]}
                    onRefresh={invitations.refresh}
                    onResolve={invitations.resolve}
                    onRecover={invitations.recover}
                  />
                ) : (
                  <div
                    key={index}
                    className="surface-radius-md border divider-subtle p-4"
                  >
                    <span>
                      {t("team.pending_invitations")} · {block.text}
                    </span>
                    <time className="ml-3" dateTime={message.created_at}>
                      {formatMessageTime(new Date(message.created_at).getTime())}
                    </time>
                    <p className="mt-2">
                      {t(
                        block.invitee_user_id !== currentUserId
                          ? "team.invitation_sent"
                          : invitations.loading
                          ? "team.loading"
                          : invitations.failed
                          ? "team.invitation_failed"
                          : "team.invitation_closed",
                      )}
                    </p>
                  </div>
                );
              })}
          </li>
        ) : (
          <Fragment key={message.id}>
            <TeamMessageItem
              roomId={roomId}
              agent={
                message.author_type === "agent"
                  ? agentsByID.get(message.author_agent_id ?? "")
                  : undefined
              }
              person={memberDirectory.find(
                (member) => member.user_id === message.author_user_id,
              )}
              currentUserId={currentUserId}
              isCompact={isCompact}
              key={message.id}
              message={message}
              replyTarget={replyTarget(
                message.author_type === "agent" && message.delivery_id
                  ? sourcesByDelivery.get(message.delivery_id)
                  : undefined,
              )}
              mentionDirectory={mentionDirectory}
              threadAction={jobs
                .filter(
                  (job) =>
                    message.author_type === "agent" &&
                    message.delivery_id &&
                    job.delivery_id === message.delivery_id,
                )
                .map((job) => (
                  <Fragment key={job.id}>
                    {stopAction(job)}
                    <ThreadActionButton
                      active={selectedThreadID === job.id}
                      agentName={
                        agentsByID.get(job.agent_id)?.name ?? job.agent_id
                      }
                      onClick={() => onOpenThread(job)}
                    />
                  </Fragment>
                ))}
            />
            {jobs
              .filter(
                (job) =>
                  job.source_message_id === message.id &&
                  !messages.some(
                    (reply) =>
                      reply.author_type === "agent" &&
                      reply.delivery_id === job.delivery_id,
                  ),
              )
              .map((job) => (
                <li key={job.id}>
                  <RoomAgentResultCard
                    activityState={
                      job.state === "ready"
                        ? "sending"
                        : job.state === "running"
                        ? "thinking"
                        : undefined
                    }
                    agentAvatar={agentsByID.get(job.agent_id)?.avatar}
                    agentName={
                      agentsByID.get(job.agent_id)?.name ?? job.agent_id
                    }
                    canRespondToPermissions={false}
                    compact={isCompact}
                    emptyState={
                      <span
                        className={getUiTypographyClassName({
                          role: "supporting",
                          tone: "muted",
                        })}
                      >
                        {t(
                          job.failure_code === "artifact_delivery_failed"
                            ? "team.artifact_delivery_failed"
                            : job.failure_code === "execution_failed"
                            ? "team.delivery_failed"
                            : `team.node_job_${job.state}`,
                        )}
                      </span>
                    }
                    headerActions={
                      <>
                        {stopAction(job)}
                        <ThreadActionButton
                          active={selectedThreadID === job.id}
                          agentName={
                            agentsByID.get(job.agent_id)?.name ?? job.agent_id
                          }
                          onClick={() => onOpenThread(job)}
                        />
                      </>
                    }
                    isLastRound
                    isLoading={
                      job.state === "running" || job.state === "ready"
                    }
                    messages={[]}
                    replyTarget={replyTarget(message)}
                    roundId={job.round_id!}
                  />
                </li>
              ))}
            {deliveries
              .filter(
                (delivery) =>
                  delivery.message_id === message.id &&
                  !jobs.some((job) => job.delivery_id === delivery.id) &&
                  delivery.state !== "completed" &&
                  !messages.some(
                    (reply) =>
                      reply.delivery_id === delivery.id &&
                      reply.output_kind === "final",
                  ),
              )
              .map((delivery) => (
                <li key={delivery.id}>
                  <RoomAgentResultCard
                    agentAvatar={agentsByID.get(delivery.agent_id)?.avatar}
                    agentName={
                      agentsByID.get(delivery.agent_id)?.name ??
                      delivery.agent_id
                    }
                    canRespondToPermissions={false}
                    compact={isCompact}
                    emptyState={
                      <div role="status">
                        {delivery.state === "pending" ||
                        delivery.state === "leased" ? (
                          <MessageActivityStatus
                            className={ROOM_RESULT_ACTIVITY_ALIGNMENT_CLASS_NAME}
                            stableSlot
                            state={
                              delivery.state === "pending"
                                ? "sending"
                                : delivery.execution_state === "waiting_input"
                                ? "waiting_input"
                                : "replying"
                            }
                            label={t(
                              delivery.cancel_requested
                                ? "team.delivery_stopping"
                                : delivery.state === "leased" &&
                                  delivery.execution_state
                                ? `team.delivery_${delivery.execution_state}`
                                : `team.delivery_${delivery.state}`,
                            )}
                          />
                        ) : (
                          <span
                            className={getUiTypographyClassName({
                              role: "supporting",
                              tone: "muted",
                            })}
                          >
                            {t(
                              DELIVERY_FAILURE_KEYS[
                                delivery.failure_code ?? ""
                              ] ??
                                (delivery.state === "completed"
                                  ? "team.node_job_completed"
                                  : `team.delivery_${delivery.state}`),
                            )}
                          </span>
                        )}
                      </div>
                    }
                    headerAction={
                      (delivery.state === "pending" ||
                        (delivery.state === "leased" &&
                          !delivery.cancel_requested)) &&
                      message.author_user_id === currentUserId ? (
                        <DeliveryCancelButton
                          roomId={roomId}
                          deliveryId={delivery.id}
                          running={delivery.state === "leased"}
                        />
                      ) : undefined
                    }
                    isLastRound
                    isLoading={false}
                    messages={[]}
                    replyTarget={replyTarget(message)}
                    roundId={delivery.id}
                  />
                </li>
              ))}
          </Fragment>
        ),
      )}
      {directUserId ? (
        <li>
          <TeamInvitationList
            {...invitations}
            invitations={pending.filter(
              (item) =>
                !messages.some((message) =>
                  message.content.blocks.some(
                    (block) =>
                      block.room_id === item.room.id &&
                      new Date(block.invited_at ?? "").getTime() ===
                        new Date(item.created_at).getTime(),
                  ),
                ),
            )}
            recoveryRooms={[]}
            onRefresh={invitations.refresh}
            onResolve={invitations.resolve}
            onRecover={invitations.recover}
          />
        </li>
      ) : null}
    </ol>
  );
}

// 未领取的投递直接取消；执行中的投递只请求停止，由执行节点中断后收口。
function DeliveryCancelButton({
  roomId,
  deliveryId,
  running,
}: {
  roomId: string;
  deliveryId: string;
  running: boolean;
}) {
  const { t } = useI18n();
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);
  return (
    <>
      <UiButton
        variant="ghost"
        disabled={busy}
        onClick={async () => {
          setBusy(true);
          setFailed(false);
          try {
            await cancelTeamDelivery(roomId, deliveryId);
          } catch {
            setFailed(true);
          } finally {
            setBusy(false);
            window.dispatchEvent(new Event("focus"));
          }
        }}
      >
        {t(running ? "team.stop_delivery" : "team.cancel_waiting")}
      </UiButton>
      {failed ? <span role="alert">{t("team.cancel_failed")}</span> : null}
    </>
  );
}

function TeamMessageItem({
  roomId,
  replyTarget,
  threadAction,
  mentionDirectory,
  person,
  agent,
  currentUserId,
  isCompact,
  message,
}: {
  roomId: string;
  replyTarget?: { name: string; avatar?: string | null; message?: string };
  threadAction?: ReactNode[];
  mentionDirectory: AgentMentionDirectory;
  agent?: ControlAgentDirectoryEntry;
  person?: ControlMemberDirectoryEntry;
  currentUserId: string | null;
  isCompact: boolean;
  message: TeamMessage;
}) {
  const { t } = useI18n();
  const [downloadFailed, setDownloadFailed] = useState(false);
  const transfer = useRef<AbortController | null>(null);
  useEffect(() => () => transfer.current?.abort(), []);
  const attachments: MessageAttachment[] = (
    message.content.attachments ?? []
  ).map((file) => ({
    file_name: file.name,
    workspace_path: "",
    scope: "relayRoom",
    kind: "file",
    size: file.size,
    room_id: roomId,
    relay_file: { id: file.id, sha256: file.sha256 },
  }));
  const download = (attachment: MessageAttachment) => {
    const file = message.content.attachments?.find(
      (item) => item.id === attachment.relay_file?.id,
    );
    if (!file) return;
    transfer.current?.abort();
    const controller = new AbortController();
    transfer.current = controller;
    setDownloadFailed(false);
    void saveTeamFile(roomId, file, controller.signal).catch(() => {
      if (!controller.signal.aborted) setDownloadFailed(true);
    });
  };
  const content = message.content.blocks
    .map((block) => block.text)
    .join("\n\n");
  const agentMentions = projectTeamMentions(
    content,
    message.mentions ?? [],
    mentionDirectory,
  );
  // 在线回复只适配消息事实，身份头、正文、复制与 Thread 排布沿用 Room 展示面。
  if (message.author_type === "agent") {
    const files = attachments.length ? (
      <div className="flex">
        <MessageUserAttachments
          attachments={attachments}
          onOpenAttachment={download}
        />
      </div>
    ) : null;
    return (
      <li>
        <RoomAgentResultCard
          agentAvatar={agent?.avatar}
          agentMentionDirectory={mentionDirectory}
          agentName={
            agent?.name ||
            message.author_display_name ||
            message.author_username ||
            "?"
          }
          canRespondToPermissions={false}
          compact={isCompact}
          emptyState={files}
          headerActions={threadAction?.length ? threadAction : undefined}
          isLoading={false}
          messages={[
            {
              message_id: message.id,
              session_key: `team:${message.conversation_id}`,
              conversation_id: message.conversation_id,
              agent_id: message.author_agent_id ?? "",
              round_id: message.id,
              role: "assistant",
              timestamp: new Date(message.created_at).getTime(),
              content: [{ type: "text", text: content }],
              is_complete: true,
              agent_mentions: agentMentions,
              model: message.content.execution?.model,
              result_summary: message.content.execution?.result_summary
                ? {
                    ...message.content.execution.result_summary,
                    subtype: "success",
                    is_error: false,
                  }
                : undefined,
            },
          ]}
          replyTarget={replyTarget}
          roundId={message.id}
        />
        {content.trim() ? files : null}
        {downloadFailed ? <p role="alert">{t("team.files_error")}</p> : null}
      </li>
    );
  }
  const author =
    person?.display_name ||
    person?.username ||
    message.author_display_name ||
    message.author_username ||
    "?";
  return (
    <li>
      <MessageUserSection
        alignment={
          currentUserId && message.author_user_id === currentUserId
            ? "right"
            : "left"
        }
        onOpenAttachment={download}
        compact={isCompact}
        author={
          message.author_user_id === currentUserId
            ? undefined
            : { name: author, avatar: person?.avatar }
        }
        agentMentionDirectory={mentionDirectory}
        message={{
          attachments,
          agent_id: "",
          client_message_id: message.client_message_id,
          content,
          agent_mentions: agentMentions,
          conversation_id: message.conversation_id,
          message_id: message.id,
          role: "user",
          room_id: null,
          round_id: message.id,
          session_key: `team:${message.conversation_id}`,
          timestamp: new Date(message.created_at).getTime(),
        }}
      />
      {downloadFailed ? <p role="alert">{t("team.files_error")}</p> : null}
    </li>
  );
}

// Relay 的目标身份只用于展示映射；正文出现同名文本不会新增唤醒目标。
function projectTeamMentions(
  content: string,
  mentions: NonNullable<TeamMessage["mentions"]>,
  directory: AgentMentionDirectory,
): AgentMention[] {
  return mentions.flatMap(({ member_id }) => {
    const name = directory.names?.[member_id];
    if (!name) return [];
    const escaped = name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    return Array.from(
      content.matchAll(
        new RegExp(
          `(?:^|\\s)(@${escaped})(?=$|\\s|[，。！？、,.!?;:：；])`,
          "giu",
        ),
      ),
      (match) => {
        const label = match[1];
        const start = match.index + match[0].length - label.length;
        const startRune = Array.from(content.slice(0, start)).length;
        return {
          agent_id: member_id,
          label,
          content_block_index: 0,
          start_rune: startRune,
          end_rune: startRune + Array.from(label).length,
        };
      },
    );
  });
}

const DELIVERY_FAILURE_KEYS: Record<
  string,
  | "team.handoff_limit_exceeded"
  | "team.artifact_delivery_failed"
  | "team.queue_expired"
  | "team.request_cancelled"
  | "team.delivery_expired"
  | "team.delivery_failed"
> = {
  handoff_limit_exceeded: "team.handoff_limit_exceeded",
  artifact_delivery_failed: "team.artifact_delivery_failed",
  queue_expired: "team.queue_expired",
  request_cancelled: "team.request_cancelled",
  lease_expired: "team.delivery_expired",
  execution_failed: "team.delivery_failed",
};
