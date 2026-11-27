// INPUT: Home 启动台控制器。
// OUTPUT: 字标下方以输入壳为左基线的窄列：问候、复用共享 Composer 的主智能体输入框、接下来的定时任务、新手提示词分类与进行中的会话行。
// POS: Home 启动台视图；数据与导航全部来自 useHomeLaunchController。
"use client";

import {
  CalendarClock,
  ChevronRight,
  Code2,
  Lightbulb,
  PenLine,
  Users,
  X,
  type LucideIcon,
} from "lucide-react";
import { useEffect, useRef, useState } from "react";

import { ComposerPanel } from "@/features/conversation/shared/composer/composer-panel";
import { focusComposerInputAtEnd } from "@/features/conversation/shared/composer/controller/composer-textarea";
import { getDefaultAgentRuntimeKind } from "@/config/runtime-options";

import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { cn } from "@/shared/ui/class-name";
import { UiAgentAvatar, UiRoomAvatar } from "@/shared/ui/display/avatar";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { InputQueueItem } from "@/types/agent/agent-conversation";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";
import type { CommandCatalogData } from "@/types/generated/protocol";

import { ConversationStatusDot } from "../sidebar/conversation-status-dot";
import type { SidebarConversationItem } from "../sidebar/sidebar-conversation-model";
import {
  HOME_LAUNCH_CATEGORIES,
  pickMixedPrompts,
  type HomeDayPeriod,
  type HomeLaunchPickerId,
  type HomeLaunchPrompt,
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

const PICKER_ICONS: Record<HomeLaunchPickerId, LucideIcon> = {
  collab: Users,
  automate: CalendarClock,
  code: Code2,
  write: PenLine,
  mix: Lightbulb,
};

const PICKER_IDS: readonly HomeLaunchPickerId[] = [...HOME_LAUNCH_CATEGORIES.map((category) => category.id), "mix"];

export function HomeLaunchpad() {
  const controller = useHomeLaunchController();
  const composerRef = useRef<HTMLDivElement | null>(null);
  // 悬停提示词时把完整内容预览在输入框占位处；只在草稿为空时可见，不改动用户已输入的内容。
  const [previewPrompt, setPreviewPrompt] = useState<string | null>(null);

  const fillSample = (prompt: string) => {
    setPreviewPrompt(null);
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
      <HomeLaunchComposer
        composerRef={composerRef}
        controller={controller}
        previewPrompt={previewPrompt}
        prompts={controller.showSamples ? <HomePromptPicker onPick={fillSample} onPreview={setPreviewPrompt} /> : null}
      />
      {controller.activeItems.length > 0 ? (
        <div className={CONTENT_INSET_CLASS}>
          <HomeActiveSection controller={controller} />
        </div>
      ) : null}
    </div>
  );
}

function HomeLaunchComposer({
  composerRef,
  controller,
  previewPrompt,
  prompts,
}: {
  composerRef: React.RefObject<HTMLDivElement | null>;
  controller: HomeLaunchController;
  previewPrompt: string | null;
  /** 提示词选择器紧跟输入框，排在「接下来」定时任务条之前。 */
  prompts: React.ReactNode;
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
          defaultPlaceholder={previewPrompt ?? t("home.launch.placeholder")}
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
      {prompts ? <div className={cn("pt-5 pb-2", CONTENT_INSET_CLASS)}>{prompts}</div> : null}
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

/**
 * 新手提示词：收起时是一排分类胶囊；点开一个分类后整排换成该分类的提示词列表。
 * 悬停或键盘聚焦某条提示词时在输入框预览全文，点击才填入草稿。
 */
function HomePromptPicker({
  onPick,
  onPreview,
}: {
  onPick: (prompt: string) => void;
  onPreview: (prompt: string | null) => void;
}) {
  const { t } = useI18n();
  const [openId, setOpenId] = useState<HomeLaunchPickerId | null>(null);
  const [prompts, setPrompts] = useState<HomeLaunchPrompt[]>([]);

  const open = (id: HomeLaunchPickerId) => {
    setOpenId(id);
    // 「随便看看」每次打开重新抽一组，其余分类按固定顺序。
    setPrompts(id === "mix" ? pickMixedPrompts() : [...(HOME_LAUNCH_CATEGORIES.find((category) => category.id === id)?.prompts ?? [])]);
  };

  const close = () => {
    setOpenId(null);
    onPreview(null);
  };

  // Esc 收起列表；预览随之清空。
  useEffect(() => {
    if (!openId) {
      return;
    }
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setOpenId(null);
        onPreview(null);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [onPreview, openId]);

  if (!openId) {
    return (
      <div aria-label={t("home.launch.samples_title")} className="flex flex-wrap justify-center gap-2" role="group">
        {PICKER_IDS.map((id) => {
          const Icon = PICKER_ICONS[id];
          return (
            <button
              className={cn(
                "home-launch-chip flex h-8 items-center gap-1.5 rounded-full px-3",
                getUiTypographyClassName({ role: "metadata", tone: "default", weight: "medium" }),
              )}
              key={id}
              onClick={() => open(id)}
              type="button"
            >
              <Icon aria-hidden className="h-3.5 w-3.5 text-(--icon-muted)" />
              {t(`home.launch.category_${id}`)}
            </button>
          );
        })}
      </div>
    );
  }

  const Icon = PICKER_ICONS[openId];
  return (
    <section className="home-launch-panel radius-control-lg flex flex-col p-1.5" onMouseLeave={() => onPreview(null)}>
      <header className="flex items-center gap-1.5 py-1 pl-2.5 pr-1">
        <Icon aria-hidden className="h-3.5 w-3.5 text-(--icon-muted)" />
        <h3 className={cn("flex-1", getUiTypographyClassName({ role: "metadata", tone: "muted", weight: "medium" }))}>
          {t(`home.launch.category_${openId}`)}
        </h3>
        <button
          aria-label={t("home.launch.prompts_close")}
          className="home-launch-row radius-control-md flex h-7 w-7 items-center justify-center text-(--icon-muted)"
          onClick={close}
          type="button"
        >
          <X aria-hidden className="h-4 w-4" />
        </button>
      </header>
      <ul className="flex flex-col">
        {prompts.map((item) => {
          const fullPrompt = t(item.promptKey);
          return (
            <li className="home-launch-prompt-item" key={item.id}>
              <button
                className={cn(
                  "home-launch-row radius-control-md group flex w-full items-center gap-2 px-2.5 py-2 text-left",
                  getUiTypographyClassName({ role: "supporting", tone: "default" }),
                )}
                onBlur={() => onPreview(null)}
                onClick={() => {
                  setOpenId(null);
                  onPick(fullPrompt);
                }}
                onFocus={() => onPreview(fullPrompt)}
                onMouseEnter={() => onPreview(fullPrompt)}
                type="button"
              >
                <span className="min-w-0 flex-1 truncate">{t(item.titleKey)}</span>
                <ChevronRight
                  aria-hidden
                  className="h-4 w-4 shrink-0 text-(--icon-muted) opacity-0 transition-opacity group-hover:opacity-100 group-focus-visible:opacity-100"
                />
              </button>
            </li>
          );
        })}
      </ul>
    </section>
  );
}

function HomeActiveSection({ controller }: { controller: HomeLaunchController }) {
  const { t } = useI18n();
  return (
    <section className="flex flex-col gap-3">
      <HomeSectionHeader title={t("home.launch.activity_title")} />
      {/* 会话用无边框单列行：行内容贴左基线，悬停底色向两侧外扩，避免一排卡片显得杂乱。 */}
      <div className="-mx-3 flex flex-col">
        {controller.activeItems.map((item) => (
          <HomeActiveRow item={item} key={item.id} onOpen={controller.openConversation} />
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
  const statusLabel = t(isWorking ? "home.launch.status_working" : "home.launch.status_waiting");
  return (
    <button
      aria-label={`${item.title} · ${statusLabel}`}
      className="home-launch-row radius-control-md flex items-center gap-3 px-3 py-2 text-left"
      onClick={() => onOpen(item)}
      title={item.title}
      type="button"
    >
      {/* Room 头像 32px、Agent 头像 28px：放进同一宽度的槽里，让各行文字起点对齐。 */}
      <span className="flex h-8 w-8 shrink-0 items-center justify-center">
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
      </span>
      {/* 行内只留头像和最新一句：名称长短不一会把摘要挤得参差不齐，名称交给头像、读屏标签与悬停提示。 */}
      <span className={cn("min-w-0 flex-1 truncate", getUiTypographyClassName({ role: "supporting", tone: "default" }))}>
        {item.summary || item.title}
      </span>
      {/* 首页行宽充裕，状态色点排在行尾，和左侧头像分开读。 */}
      <ConversationStatusDot placement="inline" status={item.activityStatus} />
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
