import { useMemo, useState, type MouseEvent } from "react";
import {
  Compass,
  FilePlus,
  FolderOpen,
  FolderPlus,
  FolderTree,
  History,
  Info,
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

import {
  COMPOSER_ACTION_BUTTON_CLASS_NAME,
  COMPOSER_PRIMARY_ACTION_BUTTON_CLASS_NAME,
  COMPOSER_SHELL_CLASS_NAME,
} from "@/features/conversation/shared/composer/composer-styles";
import { ConversationResizeHandle } from "@/features/conversation/shared/editor/conversation-resize-handle";
import { MessageItem } from "@/features/conversation/shared/message";
import { cn } from "@/shared/ui/class-name";
import { UiAgentAvatar, UiRoomAvatar } from "@/shared/ui/display/avatar";
import { UiBadge, UiCounterBadge } from "@/shared/ui/display/badge";
import { UiSearchInput } from "@/shared/ui/form/form-control";
import { UiListRow } from "@/shared/ui/list/list-row";
import { WorkspaceConversationTabs } from "@/shared/ui/workspace/controls/workspace-conversation-tabs";
import { WorkspaceFileTree } from "@/shared/ui/workspace/tree/workspace-file-tree";
import {
  WorkspaceSurfaceHeader,
  WorkspaceSurfaceToolbarAction,
} from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import { WorkspaceTaskPanel } from "@/shared/ui/workspace/surface/workspace-task-strip";
import { WorkspaceSurfaceView } from "@/shared/ui/workspace/surface/workspace-surface-view";
import type { RoomSurfaceTabKey } from "@/features/conversation/room/surface/header/room-header-tabs";

import {
  demoConversations,
  demoRounds,
  demoTodos,
  demoWorkspaceEntries,
} from "./landing-demo-data";

type LandingSidebarTab = "chat" | "contacts" | "capabilities";

function LandingWorkbenchSidebar() {
  const [activeTab, setActiveTab] = useState<LandingSidebarTab>("chat");
  const [query, setQuery] = useState("");
  const sidebarItems = useMemo(() => [
    {
      id: "amy",
      title: "Amy",
      description: "Landing review",
      time: "10:03",
      active: true,
      leading: <UiAgentAvatar avatar="/icon/agent/8.png" isWorking name="Amy" />,
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
          roomId="design-room"
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
      leading: <UiRoomAvatar members={[]} roomId="runtime" title="Runtime QA" />,
      status: <UiCounterBadge count={2} />,
    },
  ], []);
  const visibleItems = sidebarItems.filter((item) => (
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
          const isActive = activeTab === key;
          return (
            <button
              aria-selected={isActive}
              className={cn("landing-real-sidebar-tab", isActive && "is-active")}
              key={key}
              onClick={() => setActiveTab(key)}
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
          className="flex-1"
          inputClassName="text-[13px]"
          onChange={setQuery}
          placeholder="搜索聊天"
          value={query}
        />
        <button className="landing-real-sidebar-create" title="创建群聊" type="button">
          <Plus className="h-4 w-4" />
        </button>
      </div>

      <div className="landing-real-sidebar-list">
        {visibleItems.map((item) => (
          <UiListRow
            active={item.active}
            description={item.description}
            key={item.id}
            leading={item.leading}
            meta={<span className="text-[11px] tabular-nums text-(--text-soft)">{item.time}</span>}
            subtitleTrailing={item.status}
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
        {demoRounds.map((round, index) => (
          <MessageItem
            assistantContentMode="dm_archived"
            currentAgentAvatar="/icon/agent/8.png"
            currentAgentName="Amy"
            currentUserAvatar="/icon/agent/17.png"
            isLastRound={index === demoRounds.length - 1}
            key={round.round_id}
            messages={round.messages}
            roundId={round.round_id}
            workspaceAgentId="agent-amy"
          />
        ))}
      </div>

      <div className="landing-real-composer">
        <div className={cn(COMPOSER_SHELL_CLASS_NAME, "landing-real-composer-shell")}>
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
      <WorkspaceTaskPanel todos={demoTodos} />
    </section>
  );
}

function LandingWorkbenchMain() {
  const [activeTab, setActiveTab] = useState<RoomSurfaceTabKey>("workspace");

  return (
    <section className="landing-real-main">
      <WorkspaceSurfaceScaffold
        bodyClassName="relative"
        header={(
          <WorkspaceSurfaceHeader
            activeTab={activeTab}
            density="compact"
            leading={<UiAgentAvatar avatar="/icon/agent/8.png" className="h-full w-full border-0 shadow-none" name="Amy" size="sm" />}
            onChangeTab={setActiveTab}
            tabs={[
              { key: "history", label: "历史", icon: History },
              { key: "workspace", label: "工作区", icon: FolderTree },
              { key: "about", label: "关于", icon: Info },
            ]}
            tabsLeading={(
              <WorkspaceConversationTabs
                conversationId="conv-landing"
                conversations={demoConversations}
                onCreateConversation={async () => "conv-new"}
                onSelectConversation={() => undefined}
              />
            )}
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
              ariaLabel="调整右侧面板宽度"
              onMouseDown={() => undefined}
            />
            <LandingWorkbenchFiles />
          </section>
        </div>
      </WorkspaceSurfaceScaffold>
    </section>
  );
}

function LandingWorkbenchFiles() {
  const noopFile = () => undefined;
  const noopMenu = (event: MouseEvent) => {
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
        bodyClassName="px-3 py-2 sm:px-3 xl:px-4"
        bodyScrollable={false}
        contentClassName="landing-real-workspace-content"
        eyebrow="Workspace"
        maxWidthClassName="max-w-none"
        showEyebrow={false}
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
                activePath="web/src/pages/landing/landing-page.tsx"
                entries={demoWorkspaceEntries}
                focusedDirectoryPath="web/src/pages/landing"
                onClickDirectory={noopFile}
                onClickFile={noopFile}
                onContextMenu={noopMenu}
                onDeleteEntry={noopFile}
                onRenameEntry={noopFile}
              />
            </div>
          </div>
        </div>
      </WorkspaceSurfaceView>
    </aside>
  );
}

export function WorkbenchSection() {
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
