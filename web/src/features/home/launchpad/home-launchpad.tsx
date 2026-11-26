// INPUT: Home 启动台控制器。
// OUTPUT: 字标下方以输入壳为左基线的窄列：问候、复用共享 Composer 的主智能体输入框、接下来的定时任务、新手示例与进行中的会话卡片。
// POS: Home 启动台视图；数据与导航全部来自 useHomeLaunchController。
"use client";

import {
  CalendarClock,
  FileText,
  FolderGit2,
  Sunrise,
  Users,
  type LucideIcon,
} from "lucide-react";
import { useRef } from "react";

import { ComposerPanel } from "@/features/conversation/shared/composer/composer-panel";
import { focusComposerInputAtEnd } from "@/features/conversation/shared/composer/controller/composer-textarea";
import { getDefaultAgentRuntimeKind } from "@/config/runtime-options";

import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { cn } from "@/shared/ui/class-name";
import { UiAgentAvatar, UiRoomAvatar } from "@/shared/ui/display/avatar";
import { UiBadge } from "@/shared/ui/display/badge";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { InputQueueItem } from "@/types/agent/agent-conversation";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";
import type { CommandCatalogData } from "@/types/generated/protocol";

import type { SidebarConversationItem } from "../sidebar/sidebar-conversation-model";
import {
  HOME_LAUNCH_SAMPLES,
  type HomeDayPeriod,
  type HomeLaunchSample,
} from "./home-launch-model";
import {
  HOME_LAUNCH_DRAFT_SCOPE,
  useHomeLaunchController,
  type HomeLaunchController,
} from "./use-home-launch-controller";

// 首页把正文与附件带进主智能体对话；Slash、队列与 Goal 依赖会话实时通道，留给进入对话后的完整 Composer。
const EMPTY_COMMAND_CATALOG: CommandCatalogData = { commands: [], status: "unavailable" };
const EMPTY_INPUT_QUEUE: InputQueueItem[] = [];

/** 与共享 Composer 外层的响应式内边距一致：列表行的悬停底色与输入壳左右边缘对齐。 */
const CONTENT_INSET_CLASS = "px-3 sm:px-5 xl:px-6";

const GREETING_KEYS: Record<HomeDayPeriod, TranslationKey> = {
  morning: "home.launch.greeting_morning",
  afternoon: "home.launch.greeting_afternoon",
  evening: "home.launch.greeting_evening",
  night: "home.launch.greeting_night",
};

const SAMPLE_ICONS: Record<HomeLaunchSample["id"], LucideIcon> = {
  room: Users,
  brief: Sunrise,
  repo: FolderGit2,
  doc: FileText,
};

export function HomeLaunchpad() {
  const controller = useHomeLaunchController();
  const composerRef = useRef<HTMLDivElement | null>(null);

  const fillSample = (prompt: string) => {
    controller.fillDraft(prompt);
    // 示例只填入草稿，让用户改完再发；光标放到末尾方便继续输入。
    window.requestAnimationFrame(() => {
      const textarea = composerRef.current?.querySelector("textarea");
      if (textarea) {
        focusComposerInputAtEnd(textarea);
      }
    });
  };

  // 字标居中，下方内容统一以输入壳左边缘为基线左对齐；不重复侧栏的会话列表与导航入口。
  return (
    <div className="mx-auto flex w-full max-w-[720px] flex-col gap-8 px-4 pb-12">
      <HomeLaunchComposer composerRef={composerRef} controller={controller} />
      {controller.showSamples || controller.activeItems.length > 0 ? (
        <div className={cn("flex flex-col gap-8", CONTENT_INSET_CLASS)}>
          {controller.showSamples ? <HomeSampleSection onPick={fillSample} /> : null}
          {controller.activeItems.length > 0 ? <HomeActiveSection controller={controller} /> : null}
        </div>
      ) : null}
    </div>
  );
}

function HomeLaunchComposer({
  composerRef,
  controller,
}: {
  composerRef: React.RefObject<HTMLDivElement | null>;
  controller: HomeLaunchController;
}) {
  const { t } = useI18n();

  return (
    <section className="flex flex-col gap-1">
      <p className={cn(CONTENT_INSET_CLASS, getUiTypographyClassName({ role: "pageTitle", tone: "muted", weight: "regular" }))}>
        {t(GREETING_KEYS[controller.period])}
      </p>
      <div className="home-launch-composer" ref={composerRef}>
        <ComposerPanel
          commandCatalog={EMPTY_COMMAND_CATALOG}
          compact={false}
          contextUsage={null}
          defaultDeliveryPolicy="queue"
          defaultPlaceholder={t("home.launch.placeholder")}
          draftScopeKey={HOME_LAUNCH_DRAFT_SCOPE}
          goalScopeLabel="Nexus"
          historyScopeKey={HOME_LAUNCH_DRAFT_SCOPE}
          inputQueueItems={EMPTY_INPUT_QUEUE}
          isLoading={controller.isSubmitting}
          onDeleteQueuedMessage={ignoreQueuedMessage}
          onEnqueueMessage={controller.sendToMainAgent}
          onGuideQueuedMessage={ignoreQueuedMessage}
          onPrepareAttachments={controller.prepareAttachments}
          onReorderQueueMessages={ignoreQueueOrder}
          onSendMessage={controller.sendToMainAgent}
          queueWhenSessionBusy={false}
          roomMembers={[]}
          runtimeKind={getDefaultAgentRuntimeKind()}
          runtimePhase={controller.isSubmitting ? "sending" : null}
          sessionSettings={controller.sessionSettings}
          tourAnchor=""
        />
      </div>
      {controller.submitError || controller.upcomingTasks.length > 0 ? (
        <div className={cn("flex flex-col gap-2", CONTENT_INSET_CLASS)}>
          {controller.submitError ? (
            <p className={getUiTypographyClassName({ role: "metadata", tone: "danger" })} role="alert">
              {t("home.launch.submit_failed")}
            </p>
          ) : null}
          {/* 定时任务紧贴输入框：它是今天的时间提示，排在需要处理的会话之前。 */}
          {controller.upcomingTasks.length > 0 ? (
            <HomeUpcomingStrip onOpen={controller.openScheduledTasks} tasks={controller.upcomingTasks} />
          ) : null}
        </div>
      ) : null}
    </section>
  );
}

function HomeSampleSection({ onPick }: { onPick: (prompt: string) => void }) {
  const { t } = useI18n();
  return (
    <section className="flex flex-col gap-3">
      <HomeSectionHeader title={t("home.launch.samples_title")} />
      <div className="grid grid-cols-1 gap-2.5 sm:grid-cols-2">
        {HOME_LAUNCH_SAMPLES.map((sample) => {
          const Icon = SAMPLE_ICONS[sample.id];
          return (
            <button
              className="home-launch-card radius-control-lg flex items-start gap-3 p-3.5 text-left"
              key={sample.id}
              onClick={() => onPick(t(sample.promptKey))}
              type="button"
            >
              <span className="home-launch-glyph radius-control-md flex h-9 w-9 shrink-0 items-center justify-center">
                <Icon aria-hidden className="h-4 w-4" />
              </span>
              <span className="flex min-w-0 flex-col gap-0.5">
                <span className={getUiTypographyClassName({ role: "control", tone: "strong", weight: "medium" })}>
                  {t(sample.titleKey)}
                </span>
                <span className={getUiTypographyClassName({ role: "metadata", tone: "muted" })}>
                  {t(sample.descriptionKey)}
                </span>
              </span>
            </button>
          );
        })}
      </div>
    </section>
  );
}

function HomeActiveSection({ controller }: { controller: HomeLaunchController }) {
  const { t } = useI18n();
  return (
    <section className="flex flex-col gap-3">
      <HomeSectionHeader title={t("home.launch.activity_title")} />
      <div className="grid grid-cols-1 gap-2.5 sm:grid-cols-2">
        {controller.activeItems.map((item) => (
          <HomeActiveCard item={item} key={item.id} onOpen={controller.openConversation} />
        ))}
      </div>
    </section>
  );
}

function HomeActiveCard({
  item,
  onOpen,
}: {
  item: SidebarConversationItem;
  onOpen: (item: SidebarConversationItem) => void;
}) {
  const { t } = useI18n();
  const isWorking = item.activityStatus === "working";
  return (
    <button
      className="home-launch-card radius-control-lg flex items-center gap-3 p-3 text-left"
      onClick={() => onOpen(item)}
      type="button"
    >
      {item.kind === "room" ? (
        <UiRoomAvatar
          avatar={item.avatar}
          isWorking={isWorking}
          members={item.members}
          roomId={item.roomId}
          size="md"
          title={item.title}
        />
      ) : (
        <UiAgentAvatar
          avatar={(item.members[0]?.avatar ?? item.avatar) ?? undefined}
          isWorking={isWorking}
          name={item.members[0]?.name ?? item.title}
          size="md"
        />
      )}
      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="flex min-w-0 items-center gap-2">
          <span className={cn("min-w-0 truncate", getUiTypographyClassName({ role: "control", tone: "strong", weight: "medium" }))}>
            {item.title}
          </span>
          {item.activityStatus === "waiting" ? (
            <UiBadge shape="pill" size="xs" tone="primary">{t("home.launch.status_waiting")}</UiBadge>
          ) : (
            <UiBadge shape="pill" size="xs" tone="running">{t("home.launch.status_working")}</UiBadge>
          )}
          <span className={cn("ml-auto shrink-0", getUiTypographyClassName({ role: "caption", tone: "soft" }))}>
            {item.timeLabel}
          </span>
        </span>
        <span className={cn("truncate", getUiTypographyClassName({ role: "metadata", tone: "muted" }))}>
          {item.summary}
        </span>
      </span>
    </button>
  );
}

/** 接下来的定时任务收成一行横排：任务只是时间提示，不和需要处理的会话抢行。 */
function HomeUpcomingStrip({
  onOpen,
  tasks,
}: {
  onOpen: () => void;
  tasks: ScheduledTaskItem[];
}) {
  const { locale, t } = useI18n();
  return (
    <button
      className="home-launch-card radius-control-lg flex items-center gap-3 px-3.5 py-2.5 text-left"
      onClick={onOpen}
      type="button"
    >
      <span className={cn("flex shrink-0 items-center gap-1.5", getUiTypographyClassName({ role: "metadata", tone: "muted", weight: "medium" }))}>
        <CalendarClock aria-hidden className="h-3.5 w-3.5" />
        {t("home.launch.upcoming")}
      </span>
      <span className="flex min-w-0 flex-1 flex-wrap items-center gap-x-4 gap-y-1">
        {tasks.map((task) => (
          <span className="flex min-w-0 items-center gap-1.5" key={task.job_id}>
            <span className={cn("truncate", getUiTypographyClassName({ role: "metadata", tone: "strong" }))}>
              {task.name}
            </span>
            <span className={cn("shrink-0", getUiTypographyClassName({ role: "caption", tone: task.running ? "brand" : "soft" }))}>
              {task.running
                ? t("home.launch.upcoming_running")
                : t("home.launch.upcoming_at", { time: formatUpcomingTime(task.next_run_at, locale) })}
            </span>
          </span>
        ))}
      </span>
    </button>
  );
}

function HomeSectionHeader({ title }: { title: string }) {
  return (
    <h3 className={getUiTypographyClassName({ role: "control", tone: "default", weight: "semibold" })}>{title}</h3>
  );
}

function formatUpcomingTime(timestamp: number | null, locale: string): string {
  if (timestamp === null) {
    return "";
  }
  const date = new Date(timestamp);
  const now = new Date();
  const time = date.toLocaleTimeString(locale, { hour: "2-digit", hour12: false, minute: "2-digit" });
  if (date.toDateString() === now.toDateString()) {
    return time;
  }
  const day = date.toLocaleDateString(locale, { day: "numeric", month: "numeric" });
  return `${day} ${time}`;
}

async function ignoreQueuedMessage(): Promise<void> {}

async function ignoreQueueOrder(_orderedIds: string[]): Promise<void> {}
