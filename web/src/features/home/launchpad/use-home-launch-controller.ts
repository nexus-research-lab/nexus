// INPUT: Home 共享目录、Room 活动快照、定时任务目录、主智能体身份/配置与路由。
// OUTPUT: 首页启动台的主智能体投递、示例填充、进行中会话、接下来任务与示例状态。
// POS: Home 启动台控制器；输入框复用共享 Composer，投递复用 Direct Room 的 initial 草稿协议，不自行发送消息。

import { useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";

import { getDefaultAgentId, getDefaultAgentRuntimeKind } from "@/config/runtime-options";
import { prepareWorkspaceAttachments } from "@/features/conversation/shared/composer/attachments/composer-attachments";
import { useComposerDraftStore } from "@/features/conversation/shared/composer/composer-draft-store";
import { stashInitialDraftAttachments } from "@/features/conversation/shared/composer/composer-initial-attachments";
import type { ComposerSessionSettingsScope } from "@/features/conversation/shared/composer/composer-model";
import { resolveDirectRoomNavigationTarget } from "@/features/navigation/direct-room/direct-room-navigation";
import { listScheduledTasksApi } from "@/lib/api/capability/scheduled-task-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import { AppRouteBuilders } from "@/shared/navigation/route-paths";
import { useAgentStore } from "@/store/agent";
import type { AgentConversationDeliveryPolicy } from "@/types/agent/agent-conversation";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";
import type { MessageAttachment } from "@/types/conversation/message/attachment";

import { useHomeDirectory } from "../home-directory-resource";
import { useRoomActivity } from "../room-activity-resource";
import {
  buildConversationItems,
  type SidebarConversationItem,
} from "../sidebar/sidebar-conversation-model";
import {
  resolveHomeDayPeriod,
  selectActiveItems,
  selectUpcomingTasks,
  shouldShowSamples,
  type HomeDayPeriod,
} from "./home-launch-model";

const PERIOD_REFRESH_MS = 60_000;
/** 首页 Composer 的草稿与发送历史作用域；与主智能体 DM 的 Session 草稿互不覆盖。 */
export const HOME_LAUNCH_DRAFT_SCOPE = "home:main-agent";

export interface HomeLaunchController {
  activeItems: SidebarConversationItem[];
  fillDraft: (prompt: string) => void;
  hasLoaded: boolean;
  isSubmitting: boolean;
  openConversation: (item: SidebarConversationItem) => void;
  openScheduledTasks: () => void;
  period: HomeDayPeriod;
  prepareAttachments: (files: File[]) => Promise<MessageAttachment[]>;
  sendToMainAgent: (
    content: string,
    deliveryPolicy: AgentConversationDeliveryPolicy,
    attachments?: MessageAttachment[],
  ) => Promise<void>;
  /** 主智能体会话已存在时，Footer 的权限、模型与连接器直接写入该 Session 覆盖。 */
  sessionSettings?: ComposerSessionSettingsScope;
  showSamples: boolean;
  submitError: boolean;
  upcomingTasks: ScheduledTaskItem[];
}

export function useHomeLaunchController(): HomeLaunchController {
  const navigate = useNavigate();
  const { locale, t } = useI18n();
  const directory = useHomeDirectory();
  const roomActivity = useRoomActivity();
  const period = useHomeDayPeriod();
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState(false);
  const upcomingTasks = useUpcomingTasks();
  const mainAgentId = getDefaultAgentId();
  const mainAgent = useAgentStore((state) => state.agents.find((agent) => agent.agent_id === mainAgentId));
  const loadAgentsFromServer = useAgentStore((state) => state.load_agents_from_server);
  const hasAgentCatalog = useAgentStore((state) => state.agents.length > 0);
  // 冷启动直接落在首页时 Agent 目录尚未加载；没有主智能体就投影不出 Footer 的权限与模型控件。
  useEffect(() => {
    if (!hasAgentCatalog) {
      void loadAgentsFromServer();
    }
  }, [hasAgentCatalog, loadAgentsFromServer]);

  const conversationItems = useMemo(() => buildConversationItems({
    agents: directory.agents,
    conversations: directory.conversations,
    locale,
    roomActivity,
    rooms: directory.rooms,
    untitledRoomLabel: t("home.untitled_room"),
  }), [directory.agents, directory.conversations, directory.rooms, locale, roomActivity, t]);

  // Composer 已认领草稿后才调用；失败时抛出，让 Composer 按共享协议恢复草稿。
  const sendToMainAgent = useCallback(async (
    content: string,
    _deliveryPolicy: AgentConversationDeliveryPolicy,
    attachments: MessageAttachment[] = [],
  ) => {
    // 只带附件时补一句说明，initial 草稿协议要求正文非空。
    const message = content.trim() || (attachments.length > 0 ? t("home.launch.attachments_only") : "");
    if (!message) {
      return;
    }
    setIsSubmitting(true);
    setSubmitError(false);
    try {
      // initial 草稿由主智能体 DM 的 Composer 消费并进入 inbox；附件已上传到主智能体工作区，随草稿一起交接。
      const { route } = await resolveDirectRoomNavigationTarget(mainAgentId, message);
      stashInitialDraftAttachments(message, attachments);
      navigate(route);
    } catch (error) {
      console.error("[HomeLaunch] 打开主智能体对话失败:", error);
      setSubmitError(true);
      throw error;
    } finally {
      setIsSubmitting(false);
    }
  }, [mainAgentId, navigate, t]);

  const prepareAttachments = useCallback(
    (files: File[]) => prepareWorkspaceAttachments(mainAgentId, files),
    [mainAgentId],
  );

  const mainSessionKey = conversationItems.find((item) => item.isPinned)?.sessionKey;
  const sessionSettings = useMemo<ComposerSessionSettingsScope | undefined>(() => {
    if (!mainAgent || !mainSessionKey) {
      return undefined;
    }
    const runtimeKind = getDefaultAgentRuntimeKind();
    return {
      initialTargetId: mainAgent.agent_id,
      runtimeKind,
      targets: [{
        agentId: mainAgent.agent_id,
        avatar: mainAgent.avatar,
        defaultConnectorIds: mainAgent.options.connector_ids,
        defaultModel: mainAgent.options.model,
        defaultPermissionMode: mainAgent.options.permission_mode,
        defaultProvider: mainAgent.options.provider,
        name: mainAgent.name,
        sessionKey: mainSessionKey,
      }],
    };
  }, [mainAgent, mainSessionKey]);

  const fillDraft = useCallback((prompt: string) => {
    useComposerDraftStore.getState().update_composer_draft(
      HOME_LAUNCH_DRAFT_SCOPE,
      (current) => ({ ...current, input: prompt }),
    );
    setSubmitError(false);
  }, []);

  const openConversation = useCallback((item: SidebarConversationItem) => {
    const routeRoomId = item.routeRoomId ?? item.roomId;
    if (!routeRoomId) {
      return;
    }
    navigate(item.conversationId
      ? AppRouteBuilders.roomConversation(routeRoomId, item.conversationId)
      : AppRouteBuilders.room(routeRoomId));
  }, [navigate]);

  return {
    activeItems: selectActiveItems(conversationItems),
    fillDraft,
    hasLoaded: directory.hasLoaded,
    isSubmitting,
    openConversation,
    openScheduledTasks: () => navigate(AppRouteBuilders.scheduledTasks()),
    period,
    prepareAttachments,
    sendToMainAgent,
    sessionSettings,
    showSamples: directory.hasLoaded && shouldShowSamples(conversationItems),
    submitError,
    upcomingTasks,
  };
}

function useHomeDayPeriod(): HomeDayPeriod {
  const [period, setPeriod] = useState(() => resolveHomeDayPeriod(new Date()));
  useEffect(() => {
    const timer = window.setInterval(() => {
      setPeriod(resolveHomeDayPeriod(new Date()));
    }, PERIOD_REFRESH_MS);
    return () => window.clearInterval(timer);
  }, []);
  return period;
}

/** 首页只读一次任务目录；失败时静默隐藏「接下来」，不影响输入与继续区。 */
function useUpcomingTasks(): ScheduledTaskItem[] {
  const [tasks, setTasks] = useState<ScheduledTaskItem[]>([]);
  useEffect(() => {
    let cancelled = false;
    listScheduledTasksApi()
      .then((items) => {
        if (!cancelled) {
          setTasks(selectUpcomingTasks(items));
        }
      })
      .catch(() => {
        if (!cancelled) {
          setTasks([]);
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);
  return tasks;
}
