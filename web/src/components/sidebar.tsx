"use client";

/**
 * 侧边栏组件 — Agent + Session 两层结构
 *
 * [INPUT]: 依赖 @/types 的 Agent/Session 类型
 * [OUTPUT]: 对外提供 Sidebar 组件
 * [POS]: components 模块的导航层，展示 Agent 列表和其下属对话
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { useMemo, useState } from "react";
import {
  Bot,
  ChevronDown,
  ChevronRight,
  Edit3,
  MessageSquare,
  Plus,
  Search,
  Settings,
  Terminal,
  Trash2
} from "lucide-react";
import { cn } from "@/lib/utils";
import { format } from "date-fns";
import { Session } from "@/types/session";
import { Agent } from "@/types/agent";

// ==================== 类型定义 ====================

interface SidebarProps {
  /** Agent 列表 */
  agents: Agent[];
  /** 当前 Agent 下的 Session 列表 */
  sessions: Session[];
  /** 当前选中的 Agent ID */
  current_agent_id: string | null;
  /** 当前选中的 Session（agentId 即 session 路由键） */
  current_session_key: string | null;
  /** 创建新 Agent */
  on_new_agent: () => void;
  /** 选中 Agent */
  on_agent_select: (agent_id: string) => void;
  /** 删除 Agent */
  on_delete_agent: (agent_id: string) => void;
  /** 在当前 Agent 下新建对话 */
  on_new_session: () => void;
  /** 选中对话 */
  on_session_select: (session_id: string) => void;
  /** 删除对话 */
  on_delete_session: (session_id: string) => void;
  /** 编辑对话标题 */
  on_edit_title?: (session_id: string, title: string) => void;
  /** 编辑 Agent 设置 */
  on_edit_agent?: (agent_id: string) => void;
  /** 侧边栏收起 */
  is_collapsed?: boolean;
  on_toggle_collapse?: () => void;
}

// ==================== 组件实现 ====================

export function Sidebar({
  agents,
  sessions,
  current_agent_id,
  current_session_key,
  on_new_agent,
  on_agent_select,
  on_delete_agent,
  on_new_session,
  on_session_select,
  on_delete_session,
  on_edit_title,
  on_edit_agent,
  is_collapsed = false,
  on_toggle_collapse,
}: SidebarProps) {
  const [search_query, set_search_query] = useState("");
  const [expanded_agents, set_expanded_agents] = useState<Record<string, boolean>>({});
  const [editing_session_id, set_editing_session_id] = useState<string | null>(null);
  const [edit_title, set_edit_title] = useState("");

  // 过滤 Agent（按名称搜索）
  const filtered_agents = useMemo(() => {
    if (!search_query) return agents;
    return agents.filter(a =>
      a.name.toLowerCase().includes(search_query.toLowerCase())
    );
  }, [agents, search_query]);

  // 当前 Agent 展开/收起
  const toggle_agent = (agent_id: string) => {
    set_expanded_agents(prev => ({
      ...prev,
      [agent_id]: !prev[agent_id]
    }));
    // 选择 Agent 时自动展开
    on_agent_select(agent_id);
  };

  // 编辑标题
  const handle_start_edit = (e: React.MouseEvent, session_id: string, title: string) => {
    e.stopPropagation();
    set_editing_session_id(session_id);
    set_edit_title(title);
  };

  const handle_save_edit = (e: React.FormEvent) => {
    e.preventDefault();
    if (editing_session_id && on_edit_title) {
      on_edit_title(editing_session_id, edit_title);
      set_editing_session_id(null);
    }
  };

  // 按 owner_agent_id 分组 sessions
  const sessions_by_agent = useMemo(() => {
    const map = new Map<string, Session[]>();
    sessions.forEach(session => {
      const owner = session.agent_id || 'main';
      const list = map.get(owner) || [];
      list.push(session);
      map.set(owner, list);
    });
    return map;
  }, [sessions]);

  return (
    <div
      className={cn(
        "h-full flex flex-col bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60 border-r border-border relative transition-all duration-300 ease-in-out",
        is_collapsed ? "w-20" : "w-61.8"
      )}>
      {/* Header */}
      <div className="p-4 border-b border-border/40">
        <div className={cn(
          "flex items-center mb-4",
          is_collapsed ? "justify-center" : "gap-3"
        )}>
          <div className="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center text-primary">
            <Terminal size={18} />
          </div>
          {!is_collapsed && <h1 className="font-bold text-lg tracking-tight">Nexus Core</h1>}
        </div>

        {/* 新建 Agent 按钮 */}
        <button
          onClick={on_new_agent}
          className={cn(
            "w-full flex items-center gap-2 bg-primary text-primary-foreground hover:bg-primary/90 rounded-md transition-all shadow-sm font-medium text-sm",
            is_collapsed ? "justify-center px-2 py-2.5" : "justify-center px-4 py-2.5"
          )}
          title={is_collapsed ? "New Agent" : undefined}
        >
          <Plus size={16} />
          {!is_collapsed && "New Agent"}
        </button>
      </div>

      {/* Search */}
      {!is_collapsed && (
        <div className="px-4 py-3">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground w-4 h-4" />
            <input
              type="text"
              placeholder="Search agents..."
              value={search_query}
              onChange={(e) => set_search_query(e.target.value)}
              className="w-full pl-9 pr-4 py-2 bg-muted/50 border border-transparent focus:border-primary/20 focus:bg-background rounded-md text-sm outline-none transition-all placeholder:text-muted-foreground/70"
            />
          </div>
        </div>
      )}

      {/* Toggle Button */}
      {on_toggle_collapse && (
        <button
          onClick={on_toggle_collapse}
          className="absolute bottom-12 -right-3 z-10 w-6 h-6 rounded-full bg-background border border-border shadow-md hover:shadow-lg transition-all flex items-center justify-center text-muted-foreground hover:text-foreground"
          title={is_collapsed ? "Expand sidebar" : "Collapse sidebar"}
        >
          <ChevronRight
            size={30}
            className={cn(
              "transition-transform duration-300",
              !is_collapsed && "rotate-180"
            )}
          />
        </button>
      )}

      {/* Agent + Session 两层列表 */}
      <div className="flex-1 overflow-y-auto px-2 py-2 space-y-1 custom-scrollbar">
        {filtered_agents.map(agent => {
          const is_current = current_agent_id === agent.agent_id;
          const is_expanded = expanded_agents[agent.agent_id] ?? is_current;

          return (
            <div key={agent.agent_id} className="space-y-0.5">
              {/* Agent 行 */}
              <div
                className={cn(
                  "group relative flex items-center rounded-md cursor-pointer transition-all border border-transparent",
                  is_collapsed ? "justify-center px-2 py-2.5" : "gap-2 px-3 py-2",
                  is_current
                    ? "bg-primary/10 border-primary/10 text-primary"
                    : "hover:bg-muted/60 text-foreground/80 hover:text-foreground"
                )}
                onClick={() => toggle_agent(agent.agent_id)}
                title={is_collapsed ? agent.name : undefined}
              >
                <Bot size={16} className={cn(
                  "shrink-0",
                  is_current ? "text-primary" : "text-muted-foreground"
                )} />

                {!is_collapsed && (
                  <>
                    <div className="flex-1 min-w-0 flex items-center gap-1">
                      {is_expanded
                        ? <ChevronDown size={12} className="shrink-0 text-muted-foreground" />
                        : <ChevronRight size={12} className="shrink-0 text-muted-foreground" />
                      }
                      <span className="truncate text-sm font-medium">{agent.name}</span>
                    </div>

                    {/* Agent 操作按钮 */}
                    <div className={cn(
                      "absolute right-2 flex items-center gap-0.5 opacity-0 transition-opacity bg-background/80 backdrop-blur-sm rounded-md p-0.5 shadow-sm",
                      "group-hover:opacity-100"
                    )}>
                      {on_edit_agent && (
                        <button
                          onClick={(e) => { e.stopPropagation(); on_edit_agent(agent.agent_id); }}
                          className="p-1 hover:bg-muted rounded text-muted-foreground hover:text-foreground transition-colors"
                          title="Agent Settings"
                        >
                          <Settings size={12} />
                        </button>
                      )}
                      <button
                        onClick={(e) => { e.stopPropagation(); on_delete_agent(agent.agent_id); }}
                        className="p-1 hover:bg-destructive/10 rounded text-muted-foreground hover:text-destructive transition-colors"
                        title="Delete Agent"
                      >
                        <Trash2 size={12} />
                      </button>
                    </div>
                  </>
                )}
              </div>

              {/* Agent 下属 Session 列表 */}
              {is_expanded && is_current && !is_collapsed && (() => {
                const agent_sessions = sessions_by_agent.get(agent.agent_id) || [];
                return (
                  <div className="ml-4 pl-2 border-l border-border/30 space-y-0.5">
                    {/* 新建对话按钮 */}
                    <button
                      onClick={(e) => { e.stopPropagation(); on_new_session(); }}
                      className="w-full flex items-center gap-2 px-2 py-1.5 text-xs text-muted-foreground hover:text-foreground hover:bg-muted/40 rounded transition-colors"
                    >
                      <Plus size={12} />
                      <span>New Chat</span>
                    </button>

                    {/* Session 列表 */}
                    {agent_sessions.map((session, idx) => (
                      <div
                        key={session.session_key || idx}
                        className={cn(
                          "group/session relative flex items-center gap-2 px-2 py-1.5 rounded cursor-pointer transition-all",
                          current_session_key === session.session_key
                            ? "bg-primary/8 text-primary"
                            : "hover:bg-muted/40 text-foreground/70 hover:text-foreground"
                        )}
                        onClick={() => on_session_select(session.session_key)}
                      >
                        <MessageSquare size={12} className="shrink-0 text-muted-foreground/70" />

                        {editing_session_id === session.session_key ? (
                          <form onSubmit={handle_save_edit} className="flex-1 min-w-0" onClick={e => e.stopPropagation()}>
                            <input
                              autoFocus
                              type="text"
                              value={edit_title}
                              onChange={e => set_edit_title(e.target.value)}
                              onBlur={() => set_editing_session_id(null)}
                              className="w-full bg-background border border-primary/30 rounded px-1 py-0.5 text-xs outline-none"
                            />
                          </form>
                        ) : (
                          <div className="flex-1 min-w-0 flex flex-col">
                            <span className="truncate text-xs font-medium leading-tight">
                              {session.title || "Untitled Chat"}
                            </span>
                            <span className="text-[9px] text-muted-foreground/60">
                              {format(session.last_activity_at || session.created_at || Date.now(), "HH:mm")}
                            </span>
                          </div>
                        )}

                        {/* Session 操作 */}
                        <div className={cn(
                          "absolute right-1 flex items-center gap-0.5 opacity-0 transition-opacity bg-background/80 backdrop-blur-sm rounded p-0.5",
                          "group-hover/session:opacity-100",
                          editing_session_id === session.session_key && "hidden"
                        )}>
                          {on_edit_title && (
                            <button
                              onClick={(e) => handle_start_edit(e, session.session_key, session.title)}
                              className="p-0.5 hover:bg-muted rounded text-muted-foreground hover:text-foreground transition-colors"
                              title="Rename"
                            >
                              <Edit3 size={10} />
                            </button>
                          )}
                          <button
                            onClick={(e) => { e.stopPropagation(); on_delete_session(session.session_key); }}
                            className="p-0.5 hover:bg-destructive/10 rounded text-muted-foreground hover:text-destructive transition-colors"
                            title="Delete"
                          >
                            <Trash2 size={10} />
                          </button>
                        </div>
                      </div>
                    ))}

                    {agent_sessions.length === 0 && (
                      <div className="px-2 py-3 text-center">
                        <p className="text-[10px] text-muted-foreground/50">No chats yet</p>
                      </div>
                    )}
                  </div>
                );
              })()}
            </div>
          );
        })}

        {/* 空状态 */}
        {agents.length === 0 && (
          <div className="flex flex-col items-center justify-center py-12 text-center px-4">
            <div className="w-12 h-12 rounded-full bg-muted/50 flex items-center justify-center mb-3">
              <Bot size={20} className="text-muted-foreground/50" />
            </div>
            <p className="text-sm text-muted-foreground">No agents yet</p>
            <p className="text-xs text-muted-foreground/50 mt-1">Create an agent to start</p>
          </div>
        )}
      </div>

      {/* Footer */}
      <div className="p-3 border-t border-border/40 bg-muted/20">
        <button
          className={cn(
            "w-full flex items-center rounded-md hover:bg-muted/50 text-muted-foreground hover:text-foreground transition-colors text-sm",
            is_collapsed ? "justify-center px-2 py-2" : "gap-3 px-3 py-2"
          )}
          title={is_collapsed ? "Settings" : undefined}
        >
          <Settings size={16} />
          {!is_collapsed && <span>Settings</span>}
        </button>
      </div>
    </div>
  );
}
