// INPUT: Home 共享目录、Room 活动快照、定时任务目录、主智能体身份与路由。
// OUTPUT: 首页启动台的草稿、提交、继续区、接下来任务与示例状态。
// POS: Home 启动台控制器；主智能体投递复用 Direct Room 的 initial 草稿协议，不自行发送消息。

import { useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";

import { getDefaultAgentId } from "@/config/runtime-options";
import { resolveDirectRoomNavigationTarget } from "@/features/navigation/direct-room/direct-room-navigation";
import { listScheduledTasksApi } from "@/lib/api/capability/scheduled-task-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import { AppRouteBuilders } from "@/shared/navigation/route-paths";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import { useHomeDirectory } from "../home-directory-resource";
import { useRoomActivity } from "../room-activity-resource";
import {
  buildConversationItems,
  type SidebarConversationItem,
} from "../sidebar/sidebar-conversation-model";
import {
  resolveHomeDayPeriod,
  selectContinueItems,
  selectUpcomingTasks,
  shouldShowSamples,
  type HomeDayPeriod,
} from "./home-launch-model";

const PERIOD_REFRESH_MS = 60_000;

export interface HomeLaunchController {
  continueItems: SidebarConversationItem[];
  draft: string;
  hasLoaded: boolean;
  isSubmitting: boolean;
  openConversation: (item: SidebarConversationItem) => void;
  openRoute: (route: string) => void;
  period: HomeDayPeriod;
  setDraft: (value: string) => void;
  showSamples: boolean;
  submit: () => void;
  submitError: boolean;
  upcomingTasks: ScheduledTaskItem[];
}

export function useHomeLaunchController(): HomeLaunchController {
  const navigate = useNavigate();
  const { locale, t } = useI18n();
  const directory = useHomeDirectory();
  const roomActivity = useRoomActivity();
  const period = useHomeDayPeriod();
  const [draft, setDraft] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState(false);
  const upcomingTasks = useUpcomingTasks();

  const conversationItems = useMemo(() => buildConversationItems({
    agents: directory.agents,
    conversations: directory.conversations,
    locale,
    roomActivity,
    rooms: directory.rooms,
    untitledRoomLabel: t("home.untitled_room"),
  }), [directory.agents, directory.conversations, directory.rooms, locale, roomActivity, t]);

  const submit = useCallback(() => {
    const message = draft.trim();
    if (!message || isSubmitting) {
      return;
    }
    setIsSubmitting(true);
    setSubmitError(false);
    // initial 草稿由主智能体 DM 的 Composer 消费并进入 inbox，首页不重复实现投递。
    resolveDirectRoomNavigationTarget(getDefaultAgentId(), message)
      .then(({ route }) => {
        setDraft("");
        navigate(route);
      })
      .catch((error: unknown) => {
        console.error("[HomeLaunch] 打开主智能体对话失败:", error);
        setSubmitError(true);
      })
      .finally(() => setIsSubmitting(false));
  }, [draft, isSubmitting, navigate]);

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
    continueItems: selectContinueItems(conversationItems),
    draft,
    hasLoaded: directory.hasLoaded,
    isSubmitting,
    openConversation,
    openRoute: navigate,
    period,
    setDraft: (value: string) => {
      setDraft(value);
      setSubmitError(false);
    },
    showSamples: directory.hasLoaded && shouldShowSamples(conversationItems),
    submit,
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
