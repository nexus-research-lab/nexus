import { useMemo, useState } from "react";
import {
  ArrowRight,
  Brain,
  CalendarClock,
  CheckCircle2,
  CircleDot,
  Clock3,
  Compass,
  FilePlus,
  FolderOpen,
  FolderPlus,
  FolderTree,
  History,
  Info,
  Link2,
  ListTodo,
  LogOut,
  type LucideIcon,
  MessageCircle,
  Paperclip,
  PanelLeftClose,
  Plus,
  Puzzle,
  Send,
  Settings,
  Upload,
  Users2,
  X,
} from "lucide-react";
import { Link } from "react-router-dom";

import { APP_ROUTE_PATHS } from "@/app/router/route-paths";
import {
  COMPOSER_ACTION_BUTTON_CLASS_NAME,
  COMPOSER_PRIMARY_ACTION_BUTTON_CLASS_NAME,
  get_composer_shell_class_name,
} from "@/features/conversation/shared/composer-styles";
import { ConversationResizeHandle } from "@/features/conversation/shared/editor/conversation-resize-handle";
import { MessageItem } from "@/features/conversation/shared/message";
import { WorkspaceFileTree } from "@/features/conversation/room/workspace/workspace-file-tree";
import { cn } from "@/lib/utils";
import { UiAgentAvatar, UiRoomAvatar } from "@/shared/ui/avatar";
import { UiBadge, UiCounterBadge } from "@/shared/ui/badge";
import { UiSearchInput } from "@/shared/ui/form-control";
import { UiListRow } from "@/shared/ui/list-row";
import { WorkspaceConversationTabs } from "@/shared/ui/workspace/controls/workspace-conversation-tabs";
import {
  WorkspaceSurfaceHeader,
  WorkspaceSurfaceToolbarAction,
  WorkspaceTaskStrip,
} from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import { WorkspaceSurfaceView } from "@/shared/ui/workspace/surface/workspace-surface-view";
import type { WorkspaceFileEntry } from "@/types/agent/agent";
import type { Message } from "@/types/conversation/message";
import type { RoomConversationView } from "@/types/conversation/conversation";
import type { RoomSurfaceTabKey } from "@/types/conversation/room-surface";
import type { TodoItem } from "@/types/conversation/todo";

import "./landing-page.css";

type LandingSidebarTab = "chat" | "contacts" | "capabilities";

const capability_items = [
  {
    title: "Rooms",
    copy: "Shared context for multi-agent work.",
    meta: "@mentions · directed replies",
    Icon: MessageCircle,
  },
  {
    title: "Agents",
    copy: "Independent identity, workspace, skills, and memory.",
    meta: "DM · profiles · status",
    Icon: Users2,
  },
  {
    title: "Skills",
    copy: "Reusable methods and workflows agents can call.",
    meta: "library · import · update",
    Icon: Puzzle,
  },
  {
    title: "Connectors",
    copy: "Secure access to documents, code, tasks, and IM.",
    meta: "GitHub · Gmail · Slack",
    Icon: Link2,
  },
  {
    title: "Scheduled",
    copy: "Recurring work with sessions, delivery, and history.",
    meta: "one-off · recurring · manual",
    Icon: CalendarClock,
  },
  {
    title: "Memory",
    copy: "Long-term context that can be reviewed and curated.",
    meta: "candidate · promoted · ignored",
    Icon: Brain,
  },
];

const flow_items = [
  ["01", "Launch", "Start from the launcher. Mention an agent, target a room, or hand the task to Nexus."],
  ["02", "Route", "Nexus opens the right DM or room and keeps the session boundary explicit."],
  ["03", "Run", "Agents use skills, connectors, workspace files, and scheduled runs without leaving the task."],
  ["04", "Review", "Files, history, permissions, and task status stay visible for human review."],
] as const;

const ledger_items = [
  ["09:42", "Launcher", "Route landing page request"],
  ["09:44", "Workspace", "Read product routes"],
  ["09:51", "Agent", "Update landing-page.tsx"],
  ["10:03", "Control", "Run typecheck, lint, visual QA"],
] as const;

const hero_route_nodes = [
  ["Launch", "Task enters Nexus"],
  ["Route", "DM or room context"],
  ["Run", "Skills and connectors"],
  ["Review", "Files, history, approval"],
] as const;

const hero_activity_items = [
  ["10:03", "Nexus", "Split public landing from authenticated app."],
  ["10:05", "Amy", "Reads workspace files and prepares the UI pass."],
  ["10:07", "Control", "Keeps route, history, and validation visible."],
] as const;

const hero_context_items = ["Rooms", "DMs", "Skills", "Connectors", "Memory", "Schedules", "Files"] as const;

const demo_conversations: RoomConversationView[] = [
  {
    session_key: "landing-session",
    agent_id: "agent-amy",
    session_id: "sess_landing_demo",
    room_session_id: "room_sess_landing",
    room_id: "room-landing",
    conversation_id: "conv-landing",
    conversation_type: "dm",
    title: "Landing review",
    options: {},
    created_at: 1_746_720_000_000,
    last_activity_at: 1_746_722_400_000,
    is_active: true,
    message_count: 8,
  },
  {
    session_key: "runtime-session",
    agent_id: "agent-amy",
    session_id: "sess_runtime_demo",
    room_session_id: "room_sess_runtime",
    room_id: "room-landing",
    conversation_id: "conv-runtime",
    conversation_type: "dm",
    title: "Runtime QA",
    options: {},
    created_at: 1_746_633_600_000,
    last_activity_at: 1_746_637_200_000,
    is_active: false,
    message_count: 5,
  },
];

const demo_todos: TodoItem[] = [
  { content: "Read real workspace source", status: "completed" },
  { content: "Remove decorative SVG motion", status: "completed" },
  { content: "Verify route and visual layout", status: "in_progress" },
];

const demo_workspace_entries: WorkspaceFileEntry[] = [
  { path: "web", name: "web", is_dir: true, modified_at: "2026-06-01T09:42:00Z", depth: 0 },
  { path: "web/src", name: "src", is_dir: true, modified_at: "2026-06-01T09:44:00Z", depth: 1 },
  { path: "web/src/pages", name: "pages", is_dir: true, modified_at: "2026-06-01T09:45:00Z", depth: 2 },
  { path: "web/src/pages/landing", name: "landing", is_dir: true, modified_at: "2026-06-01T09:46:00Z", depth: 3 },
  {
    path: "web/src/pages/landing/landing-page.tsx",
    name: "landing-page.tsx",
    is_dir: false,
    size: 18_420,
    modified_at: "2026-06-01T09:51:00Z",
    depth: 4,
  },
  {
    path: "web/src/pages/landing/landing-page.css",
    name: "landing-page.css",
    is_dir: false,
    size: 13_800,
    modified_at: "2026-06-01T09:52:00Z",
    depth: 4,
  },
  { path: "web/src/app", name: "app", is_dir: true, modified_at: "2026-06-01T09:48:00Z", depth: 2 },
  {
    path: "web/src/app/router/app-router.tsx",
    name: "app-router.tsx",
    is_dir: false,
    size: 4_180,
    modified_at: "2026-06-01T09:49:00Z",
    depth: 3,
  },
  {
    path: "CHANGELOG.md",
    name: "CHANGELOG.md",
    is_dir: false,
    size: 9_280,
    modified_at: "2026-06-01T10:03:00Z",
    depth: 0,
  },
];

const demo_rounds: Array<{ round_id: string; messages: Message[] }> = [
  {
    round_id: "landing-round-1",
    messages: [
      {
        message_id: "landing-user-1",
        session_key: "landing-session",
        room_id: "room-landing",
        conversation_id: "conv-landing",
        agent_id: "agent-amy",
        round_id: "landing-round-1",
        role: "user",
        timestamp: 1_746_722_180_000,
        content: "Ship the public landing route and keep the review files next to the conversation.",
      },
      {
        message_id: "landing-assistant-1",
        session_key: "landing-session",
        room_id: "room-landing",
        conversation_id: "conv-landing",
        agent_id: "agent-amy",
        round_id: "landing-round-1",
        role: "assistant",
        timestamp: 1_746_722_186_000,
        model: "kimi-for-coding",
        is_complete: true,
        stop_reason: "end_turn",
        content: [
          {
            type: "text",
            text: "I found the route split: public `/`, authenticated `/launcher`, and guarded app surfaces under `/app` and `/rooms`.",
          },
          {
            type: "tool_use",
            id: "tool-read-router",
            name: "Read",
            input: { file_path: "web/src/app/router/app-router.tsx" },
          },
          {
            type: "tool_result",
            tool_use_id: "tool-read-router",
            content: "LandingPage is mounted before AuthGuard. Launcher remains guarded.",
          },
        ],
        result_summary: {
          subtype: "success",
          duration_ms: 3400,
          duration_api_ms: 2900,
          num_turns: 1,
          is_error: false,
          usage: {
            input_tokens: 3620,
            output_tokens: 840,
            cache_read_input_tokens: 15_200,
          },
        },
      },
    ],
  },
  {
    round_id: "landing-round-2",
    messages: [
      {
        message_id: "landing-user-2",
        session_key: "landing-session",
        room_id: "room-landing",
        conversation_id: "conv-landing",
        agent_id: "agent-amy",
        round_id: "landing-round-2",
        role: "user",
        timestamp: 1_746_722_520_000,
        content: "Review the workspace changes and prepare the validation pass.",
      },
      {
        message_id: "landing-assistant-2",
        session_key: "landing-session",
        room_id: "room-landing",
        conversation_id: "conv-landing",
        agent_id: "agent-amy",
        round_id: "landing-round-2",
        role: "assistant",
        timestamp: 1_746_722_529_000,
        model: "kimi-for-coding",
        is_complete: true,
        stop_reason: "end_turn",
        content: [
          {
            type: "text",
            text: "The route, workbench surface, and validation notes are ready for review.",
          },
          {
            type: "workspace_file_artifact",
            path: "web/src/pages/landing/landing-page.tsx",
            display_path: "web/src/pages/landing/landing-page.tsx",
            artifact_kind: "code",
            operation: "updated",
            scope: "agent_workspace",
            workspace_agent_id: "agent-amy",
          },
        ],
        result_summary: {
          subtype: "success",
          duration_ms: 2700,
          duration_api_ms: 2400,
          num_turns: 1,
          is_error: false,
          usage: {
            input_tokens: 4180,
            output_tokens: 690,
            cache_read_input_tokens: 18_900,
          },
        },
      },
    ],
  },
];

function LandingHeader() {
  return (
    <header className="landing-header">
      <div className="landing-header-inner">
        <Link aria-label="Nexus home" className="landing-brand" to={APP_ROUTE_PATHS.landing}>
          <img alt="" className="landing-brand-logo" src="/logo.webp" />
          <span>NEXUS</span>
        </Link>

        <nav aria-label="Landing navigation" className="landing-nav">
          <a href="#workbench">Workbench</a>
          <a href="#flow">Flow</a>
          <a href="#capabilities">Capabilities</a>
          <a href="#control">Control</a>
        </nav>

        <div className="landing-actions">
          <Link className="landing-primary-button" to={APP_ROUTE_PATHS.launcher}>
            Enter app
            <ArrowRight size={15} />
          </Link>
        </div>
      </div>
    </header>
  );
}

function HeroSignal() {
  return (
    <aside className="landing-hero-signal" aria-label="Nexus routing overview">
      <div className="landing-hero-route" aria-label="Nexus execution route">
        {hero_route_nodes.map(([title, copy]) => (
          <div className="landing-hero-route-node" key={title}>
            <span aria-hidden="true" />
            <strong>{title}</strong>
            <p>{copy}</p>
          </div>
        ))}
      </div>

      <div className="landing-hero-activity" aria-label="Live Nexus task activity">
        <div className="landing-hero-activity-head">
          <span>Active task</span>
          <strong>Landing review</strong>
        </div>
        {hero_activity_items.map(([time, actor, copy]) => (
          <div className="landing-hero-activity-row" key={`${time}-${actor}`}>
            <span>{time}</span>
            <strong>{actor}</strong>
            <p>{copy}</p>
          </div>
        ))}
      </div>

      <div className="landing-hero-context" aria-label="Nexus shared context">
        {hero_context_items.map((item) => (
          <span key={item}>{item}</span>
        ))}
      </div>
    </aside>
  );
}

function HeroSection() {
  return (
    <section className="landing-hero">
      <div className="landing-section landing-hero-inner">
        <div className="landing-hero-copy-block">
          <div className="landing-hero-title-wrap">
            <h1>Nexus</h1>
            <img alt="" aria-hidden="true" className="landing-hero-title-persona" src="/nexus/stickers/card-top.png" />
          </div>
          <p className="landing-hero-line">Agent work, in one calm workspace.</p>
          <p className="landing-hero-copy">
            Rooms, DMs, skills, connectors, memory, schedules, and workspace files share one operating surface.
          </p>
          <div className="landing-hero-actions">
            <Link className="landing-primary-button" to={APP_ROUTE_PATHS.launcher}>
              Enter Nexus
              <ArrowRight size={15} />
            </Link>
          </div>
        </div>
        <HeroSignal />
      </div>
    </section>
  );
}

function LandingWorkbenchSidebar() {
  const [active_tab, set_active_tab] = useState<LandingSidebarTab>("chat");
  const [query, set_query] = useState("");
  const sidebar_items = useMemo(() => [
    {
      id: "amy",
      title: "Amy",
      description: "Landing review",
      time: "10:03",
      active: true,
      leading: <UiAgentAvatar avatar="/icon/agent/8.png" is_working name="Amy" />,
      status: <UiBadge size="xs" tone="primary">Working</UiBadge>,
    },
    {
      id: "design-room",
      title: "Design Room",
      description: "Visual pass",
      time: "5/18",
      active: false,
      leading: (
        <UiRoomAvatar
          members={[
            { id: "amy", name: "Amy", avatar: "/icon/agent/8.png" },
            { id: "nexus", name: "Nexus", avatar: "/logo.webp" },
            { id: "lee", name: "Lee", avatar: "/icon/agent/17.png" },
          ]}
          room_id="design-room"
          title="Design Room"
        />
      ),
      status: null,
    },
    {
      id: "runtime",
      title: "Runtime QA",
      description: "Typecheck, lint",
      time: "5/12",
      active: false,
      leading: <UiRoomAvatar members={[]} room_id="runtime" title="Runtime QA" />,
      status: <UiCounterBadge count={2} />,
    },
  ], []);
  const visible_items = sidebar_items.filter((item) => (
    item.title.toLowerCase().includes(query.trim().toLowerCase()) ||
    item.description.toLowerCase().includes(query.trim().toLowerCase())
  ));
  const tabs: Array<{ key: LandingSidebarTab; label: string; Icon: LucideIcon }> = [
    { key: "chat", label: "聊天", Icon: MessageCircle },
    { key: "contacts", label: "联系人", Icon: Users2 },
    { key: "capabilities", label: "能力", Icon: Puzzle },
  ];

  return (
    <aside className="landing-real-sidebar">
      <div className="landing-real-sidebar-header">
        <button className="landing-real-nexus-button" type="button">
          <img alt="Nexus" src="/logo.webp" />
        </button>
        <span>NEXUS</span>
      </div>

      <div className="landing-real-sidebar-tabs" role="tablist" aria-label="Landing workbench sidebar tabs">
        {tabs.map(({ key, label, Icon }) => {
          const is_active = active_tab === key;
          return (
            <button
              aria-selected={is_active}
              className={cn("landing-real-sidebar-tab", is_active && "is-active")}
              key={key}
              onClick={() => set_active_tab(key)}
              role="tab"
              type="button"
            >
              <Icon className="h-3.5 w-3.5" />
              <span>{label}</span>
            </button>
          );
        })}
      </div>

      <div className="landing-real-sidebar-search">
        <UiSearchInput
          class_name="flex-1"
          input_class_name="text-[13px]"
          on_change={set_query}
          placeholder="搜索聊天"
          value={query}
        />
        <button className="landing-real-sidebar-create" title="创建群聊" type="button">
          <Plus className="h-4 w-4" />
        </button>
      </div>

      <div className="landing-real-sidebar-list">
        {visible_items.map((item) => (
          <UiListRow
            active={item.active}
            description={item.description}
            key={item.id}
            leading={item.leading}
            meta={<span className="text-[11px] tabular-nums text-(--text-soft)">{item.time}</span>}
            subtitle_trailing={item.status}
            title={item.title}
          />
        ))}
      </div>

      <div className="landing-real-sidebar-footer">
        <div className="landing-real-sidebar-footer-group">
          <button aria-label="Settings" type="button">
            <Settings className="h-4 w-4" />
          </button>
          <button aria-label="Guide center" type="button">
            <Compass className="h-4 w-4" />
          </button>
        </div>
        <span />
        <div className="landing-real-sidebar-footer-group">
          <button aria-label="Logout" type="button">
            <LogOut className="h-4 w-4" />
          </button>
          <button aria-label="Collapse sidebar" type="button">
            <PanelLeftClose className="h-4 w-4" />
          </button>
        </div>
      </div>
    </aside>
  );
}

function LandingWorkbenchChat() {
  return (
    <section className="landing-real-chat">
      <div className="landing-real-feed nexus-chat-feed soft-scrollbar">
        {demo_rounds.map((round, index) => (
          <MessageItem
            assistant_content_mode="dm_archived"
            current_agent_avatar="/icon/agent/8.png"
            current_agent_name="Amy"
            current_user_avatar="/icon/agent/17.png"
            is_last_round={index === demo_rounds.length - 1}
            key={round.round_id}
            messages={round.messages}
            round_id={round.round_id}
            workspace_agent_id="agent-amy"
          />
        ))}
      </div>

      <div className="landing-real-composer">
        <div className={cn(get_composer_shell_class_name(false), "landing-real-composer-shell")}>
          <button className={COMPOSER_ACTION_BUTTON_CLASS_NAME} type="button">
            <Paperclip className="h-4 w-4" />
          </button>
          <div className="landing-real-composer-copy">
            Message Amy, paste images, or attach workspace files...
          </div>
          <button className={COMPOSER_PRIMARY_ACTION_BUTTON_CLASS_NAME} type="button">
            <Send className="h-4 w-4" />
          </button>
        </div>
      </div>
    </section>
  );
}

function LandingWorkbenchMain() {
  const [active_tab, set_active_tab] = useState<RoomSurfaceTabKey>("workspace");

  return (
    <section className="landing-real-main">
      <WorkspaceSurfaceScaffold
        body_class_name="relative"
        header={(
          <WorkspaceSurfaceHeader
            active_tab={active_tab}
            badge="DM"
            density="compact"
            leading={<UiAgentAvatar avatar="/icon/agent/8.png" class_name="h-full w-full border-0 shadow-none" name="Amy" size="sm" />}
            on_change_tab={set_active_tab}
            tabs={[
              { key: "history", label: "历史", icon: History },
              { key: "workspace", label: "工作区", icon: FolderTree },
              { key: "about", label: "关于", icon: Info },
            ]}
            tabs_leading={(
              <WorkspaceConversationTabs
                conversation_id="conv-landing"
                conversations={demo_conversations}
                on_create_conversation={async () => "conv-new"}
                on_select_conversation={() => undefined}
              />
            )}
            tabs_trailing={<WorkspaceTaskStrip todos={demo_todos} />}
            title="Amy"
            trailing={(
              <WorkspaceSurfaceToolbarAction>
                <Compass className="h-3.5 w-3.5" />
                查看指南
              </WorkspaceSurfaceToolbarAction>
            )}
          />
        )}
      >
        <div className="landing-real-main-body">
          <LandingWorkbenchChat />
          <section className="landing-real-aux-panel">
            <ConversationResizeHandle
              aria_label="调整右侧面板宽度"
              on_mouse_down={() => undefined}
            />
            <LandingWorkbenchFiles />
          </section>
        </div>
      </WorkspaceSurfaceScaffold>
    </section>
  );
}

function LandingWorkbenchFiles() {
  const noop_file = () => undefined;
  const noop_menu = (event: React.MouseEvent) => {
    event.preventDefault();
  };

  return (
    <aside className="landing-real-workspace">
      <WorkspaceSurfaceView
        action={(
          <WorkspaceSurfaceToolbarAction>
            <X className="h-3.5 w-3.5" />
            关闭
          </WorkspaceSurfaceToolbarAction>
        )}
        body_class_name="px-3 py-2 sm:px-3 xl:px-4"
        body_scrollable={false}
        content_class_name="landing-real-workspace-content"
        eyebrow="Workspace"
        max_width_class_name="max-w-none"
        show_eyebrow={false}
        title="Workspace"
      >
        <div className="landing-real-workspace-split">
          <div className="landing-real-editor-preview">
            <div className="landing-real-editor-head">
              <strong>landing-page.tsx</strong>
              <span>TypeScript React</span>
            </div>
            <pre aria-label="Workspace file preview">{`function WorkbenchSection() {
  return (
    <section id="workbench">
      <LandingWorkbenchSidebar />
      <LandingWorkbenchMain />
    </section>
  );
}`}</pre>
          </div>

          <div className="landing-real-file-list">
            <div className="landing-real-directory-chip">
              <FolderOpen className="h-3 w-3" />
              <span>landing</span>
            </div>

            <div className="landing-real-file-actions">
              <button
                aria-label="上传"
                className="landing-real-file-action-icon is-primary"
                title="上传"
                type="button"
              >
                <Upload className="h-3 w-3" />
              </button>
              <button
                aria-label="新建文件夹"
                className="landing-real-file-action-icon"
                title="新建文件夹"
                type="button"
              >
                <FolderPlus className="h-3 w-3" />
              </button>
              <button
                aria-label="新建文件"
                className="landing-real-file-action-icon"
                title="新建文件"
                type="button"
              >
                <FilePlus className="h-3 w-3" />
              </button>
            </div>

            <div className="landing-real-file-tree soft-scrollbar">
              <WorkspaceFileTree
                active_path="web/src/pages/landing/landing-page.tsx"
                entries={demo_workspace_entries}
                focused_directory_path="web/src/pages/landing"
                on_click_directory={noop_file}
                on_click_file={noop_file}
                on_context_menu={noop_menu}
                on_delete_entry={noop_file}
                on_rename_entry={noop_file}
              />
            </div>
          </div>
        </div>
      </WorkspaceSurfaceView>
    </aside>
  );
}

function WorkbenchSection() {
  return (
    <section className="landing-section landing-workbench-section" id="workbench">
      <div className="landing-section-heading-row">
        <h2>The Nexus workbench.</h2>
        <p>
          Conversation, workspace files, tasks, history, and review stay together in the same operating surface.
        </p>
      </div>

      <div className="landing-real-shell" aria-label="Nexus real workbench preview">
        <LandingWorkbenchSidebar />
        <LandingWorkbenchMain />
      </div>
    </section>
  );
}

function FlowSection() {
  return (
    <section className="landing-section landing-flow-section" id="flow">
      <div className="landing-section-heading-row">
        <h2>From prompt to persistent work.</h2>
        <p>A short route from the launcher into durable agent execution.</p>
      </div>

      <div className="landing-flow-grid">
        {flow_items.map(([step, title, copy]) => (
          <article className="landing-flow-item" key={step}>
            <span>{step}</span>
            <h3>{title}</h3>
            <p>{copy}</p>
          </article>
        ))}
      </div>
    </section>
  );
}

function CapabilitiesSection() {
  return (
    <section className="landing-section landing-capabilities" id="capabilities">
      <div className="landing-section-heading-row">
        <h2>The actual Nexus objects.</h2>
        <p>These are product modules, not generic feature names.</p>
      </div>

      <div className="landing-capability-list">
        {capability_items.map(({ title, copy, meta, Icon }) => (
          <article className="landing-capability-row" key={title}>
            <Icon size={18} />
            <strong>{title}</strong>
            <p>{copy}</p>
            <span>{meta}</span>
          </article>
        ))}
      </div>
    </section>
  );
}

function ControlSection() {
  return (
    <section className="landing-section landing-control" id="control">
      <div className="landing-control-copy">
        <h2>Fast agents. Visible boundaries.</h2>
        <p>
          Runtime state, permissions, memory, and scheduled tasks stay close to the conversation so
          automation remains reviewable.
        </p>
        <div className="landing-control-checks">
          {[
            "Default ask mode for sensitive actions",
            "Plan-first execution for higher-risk work",
            "Workspace output next to the conversation",
            "Scheduled runs with history and delivery targets",
          ].map((item) => (
            <span key={item}>
              <CheckCircle2 size={15} />
              {item}
            </span>
          ))}
        </div>
      </div>

      <div className="landing-ledger">
        <div className="landing-ledger-head">
          <ListTodo size={17} />
          <strong>Run ledger</strong>
          <span>active</span>
        </div>
        {ledger_items.map(([time, actor, action], index) => (
          <div className="landing-ledger-row" key={`${time}-${actor}`}>
            <span>{time}</span>
            {index === ledger_items.length - 1 ? <CircleDot size={14} /> : <Clock3 size={14} />}
            <strong>{actor}</strong>
            <p>{action}</p>
          </div>
        ))}
      </div>
    </section>
  );
}

function FinalCta() {
  return (
    <section className="landing-final">
      <div className="landing-section landing-final-inner">
        <img alt="" src="/logo.webp" />
        <h2>Start from the launcher.</h2>
        <p>Route the task, keep the files, review the run.</p>
        <Link className="landing-primary-button" to={APP_ROUTE_PATHS.launcher}>
          Enter Nexus
          <ArrowRight size={15} />
        </Link>
      </div>
    </section>
  );
}

export function LandingPage() {
  return (
    <main className="landing-page">
      <LandingHeader />
      <HeroSection />
      <WorkbenchSection />
      <FlowSection />
      <CapabilitiesSection />
      <ControlSection />
      <FinalCta />
    </main>
  );
}
