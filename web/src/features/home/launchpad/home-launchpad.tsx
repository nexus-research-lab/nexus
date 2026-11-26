// INPUT: Home 启动台控制器。
// OUTPUT: 字标下方的居中单列：问候、复用共享 Composer 的主智能体输入框、新手示例、进行中的会话与接下来的定时任务。
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

/** 首页没有消息流争夺纵向空间，输入区空态留出约三行落笔高度。 */
const HOME_COMPOSER_INPUT_MIN_HEIGHT_PX = 72;

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

  const hasActivity = controller.activeItems.length > 0 || controller.upcomingTasks.length > 0;

  // 与字标同一条居中轴的窄列；分组只靠留白，除输入框外不再画边框，也不重复侧栏的会话列表。
  return (
    <div className="mx-auto flex w-full max-w-[720px] flex-col gap-8 px-4 pb-12">
      <HomeLaunchComposer composerRef={composerRef} controller={controller} />
      {controller.showSamples ? <HomeSampleSection onPick={fillSample} /> : null}
      {hasActivity ? <HomeActivitySection controller={controller} /> : null}
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
    <section className="flex flex-col gap-3">
      <p className={cn("text-center", getUiTypographyClassName({ role: "pageTitle", tone: "default", weight: "regular" }))}>
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
          inputMinHeight={HOME_COMPOSER_INPUT_MIN_HEIGHT_PX}
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
      {controller.submitError ? (
        <p className={cn(CONTENT_INSET_CLASS, getUiTypographyClassName({ role: "metadata", tone: "danger" }))} role="alert">
          {t("home.launch.submit_failed")}
        </p>
      ) : null}
    </section>
  );
}

function HomeSampleSection({ onPick }: { onPick: (prompt: string) => void }) {
  const { t } = useI18n();
  return (
    <section className={cn("flex flex-col gap-1.5", CONTENT_INSET_CLASS)}>
      <HomeSectionHeader title={t("home.launch.samples_title")} />
      <div className="grid grid-cols-1 gap-x-2 sm:grid-cols-2">
        {HOME_LAUNCH_SAMPLES.map((sample) => {
          const Icon = SAMPLE_ICONS[sample.id];
          return (
            <button
              className="home-launch-row radius-control-md flex items-start gap-3 px-3 py-2.5 text-left"
              key={sample.id}
              onClick={() => onPick(t(sample.promptKey))}
              type="button"
            >
              <Icon aria-hidden className="home-launch-glyph mt-0.5 h-4 w-4 shrink-0" />
              <span className="flex min-w-0 flex-col gap-0.5">
                <span className={getUiTypographyClassName({ role: "supporting", tone: "strong", weight: "medium" })}>
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

function HomeActivitySection({ controller }: { controller: HomeLaunchController }) {
  const { t } = useI18n();
  return (
    <section className={cn("flex flex-col gap-1.5", CONTENT_INSET_CLASS)}>
      <HomeSectionHeader title={t("home.launch.activity_title")} />
      <div className="flex flex-col">
        {controller.activeItems.map((item) => (
          <HomeActiveRow item={item} key={item.id} onOpen={controller.openConversation} />
        ))}
        {controller.upcomingTasks.map((task) => (
          <HomeUpcomingRow key={task.job_id} onOpen={controller.openScheduledTasks} task={task} />
        ))}
      </div>
    </section>
  );
}

function HomeActiveRow({
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
      className="home-launch-row radius-control-md flex items-center gap-3 px-3 py-2 text-left"
      onClick={() => onOpen(item)}
      type="button"
    >
      {item.kind === "room" ? (
        <UiRoomAvatar
          avatar={item.avatar}
          isWorking={isWorking}
          members={item.members}
          roomId={item.roomId}
          size="sm"
          title={item.title}
        />
      ) : (
        <UiAgentAvatar
          avatar={(item.members[0]?.avatar ?? item.avatar) ?? undefined}
          isWorking={isWorking}
          name={item.members[0]?.name ?? item.title}
          size="sm"
        />
      )}
      <span className={cn("max-w-[45%] shrink-0 truncate", getUiTypographyClassName({ role: "supporting", tone: "strong", weight: "medium" }))}>
        {item.title}
      </span>
      <span className={cn("min-w-0 flex-1 truncate", getUiTypographyClassName({ role: "metadata", tone: "muted" }))}>
        {item.summary}
      </span>
      {item.activityStatus === "waiting" ? (
        <UiBadge shape="pill" size="xs" tone="primary">{t("home.launch.status_waiting")}</UiBadge>
      ) : (
        <UiBadge shape="pill" size="xs" tone="running">{t("home.launch.status_working")}</UiBadge>
      )}
    </button>
  );
}

function HomeUpcomingRow({
  onOpen,
  task,
}: {
  onOpen: () => void;
  task: ScheduledTaskItem;
}) {
  const { locale, t } = useI18n();
  return (
    <button
      className="home-launch-row radius-control-md flex items-center gap-3 px-3 py-2 text-left"
      onClick={onOpen}
      type="button"
    >
      {/* 与会话头像同宽的图标槽，让任务名和会话名落在同一条文字基线上。 */}
      <span className="flex h-7 w-7 shrink-0 items-center justify-center">
        <CalendarClock aria-hidden className="home-launch-glyph h-4 w-4" />
      </span>
      <span className={cn("min-w-0 flex-1 truncate", getUiTypographyClassName({ role: "supporting", tone: "strong", weight: "medium" }))}>
        {task.name}
      </span>
      <span className={cn("shrink-0", getUiTypographyClassName({ role: "caption", tone: task.running ? "brand" : "soft" }))}>
        {task.running
          ? t("home.launch.upcoming_running")
          : t("home.launch.upcoming_at", { time: formatUpcomingTime(task.next_run_at, locale) })}
      </span>
    </button>
  );
}

function HomeSectionHeader({ title }: { title: string }) {
  return (
    <h3 className={cn("px-3", getUiTypographyClassName({ role: "caption", tone: "soft", weight: "medium" }))}>
      {title}
    </h3>
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
