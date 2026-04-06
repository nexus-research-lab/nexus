/**
 * Contacts 面板内容
 *
 * Agent 列表面板：
 * - 头部显示"成员" + 成员数 + [+] 新建按钮
 * - Agent 条目：头像 + 名称 + 在线状态指示器
 * - 点击导航到 /contacts/:agent_id
 *
 * 数据源复用 useAgentStore。
 */

import { Plus, Users } from "lucide-react";
import { memo, useCallback, useEffect } from "react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { cn } from "@/lib/utils";
import { CollapsibleSection } from "@/shared/ui/sidebar/collapsible-section";
import { useAgentStore } from "@/store/agent";
import { useSidebarStore } from "@/store/sidebar";

export const ContactsPanelContent = memo(function ContactsPanelContent() {
  const navigate = useNavigate();
  const agents = useAgentStore((s) => s.agents);
  const load_agents = useAgentStore((s) => s.load_agents_from_server);
  const active_item_id = useSidebarStore((s) => s.active_panel_item_id);
  const set_active_item = useSidebarStore((s) => s.set_active_panel_item);

  // 初始化加载
  useEffect(() => {
    void load_agents();
  }, [load_agents]);

  // 点击 Agent 条目
  const handle_click = useCallback(
    (agent_id: string) => {
      set_active_item(agent_id);
      navigate(AppRouteBuilders.contact_profile(agent_id));
    },
    [navigate, set_active_item],
  );

  // 新建 Agent（导航到 contacts 页面）
  const handle_create = useCallback(() => {
    navigate(AppRouteBuilders.contacts());
  }, [navigate]);

  return (
    <div className="flex flex-col gap-2">
      <CollapsibleSection
        action_icon={<Plus className="h-3.5 w-3.5" />}
        action_title="新建成员"
        count={agents.length}
        icon={<Users className="h-3.5 w-3.5" />}
        on_action={handle_create}
        section_id="contacts-agents"
        title="联系人"
      >
        {agents.length > 0 ? (
          agents.map((agent) => {
            const is_active = active_item_id === agent.agent_id;
            const is_running = agent.status === "running";
            const avatar_letter = agent.name.charAt(0).toUpperCase();

            return (
              <button
                key={agent.agent_id}
                className={cn(
                  "box-border flex w-full items-center gap-2.5 rounded-lg border border-transparent px-2 py-1.5 text-left transition-all duration-150",
                  is_active
                    ? ""
                    : "hover:bg-[var(--surface-interactive-hover-background)]",
                )}
                style={is_active ? {
                  background: "var(--surface-interactive-active-background)",
                  borderColor: "var(--surface-interactive-active-border)",
                } : undefined}
                onClick={() => handle_click(agent.agent_id)}
                type="button"
              >
                {/* 头像 + 状态指示器 */}
                <div className="relative">
                  <div
                    className="flex h-7 w-7 items-center justify-center rounded-full text-[10px] font-bold"
                    style={{
                      background: "var(--surface-avatar-background)",
                      color: "var(--surface-avatar-foreground)",
                    }}
                  >
                    {avatar_letter}
                  </div>
                  {/* 在线状态圆点 */}
                  <span
                    className={cn(
                      "absolute -bottom-0.5 -right-0.5 h-2.5 w-2.5 rounded-full border-2 border-white",
                      is_running ? "bg-emerald-400" : "bg-slate-300",
                    )}
                  />
                </div>

                {/* 名称 */}
                <span className="min-w-0 flex-1 truncate text-[12px] font-medium text-slate-700">
                  {agent.name}
                </span>

                {/* 状态文字 */}
                <span className="shrink-0 text-[10px] text-slate-400">
                  {is_running ? "运行中" : "空闲"}
                </span>
              </button>
            );
          })
        ) : (
          <div className="flex flex-col items-center gap-2 py-8 text-center">
            <Users className="h-5 w-5 text-slate-300" />
            <p className="text-[11px] text-slate-400">暂无成员</p>
          </div>
        )}
      </CollapsibleSection>
    </div>
  );
});
