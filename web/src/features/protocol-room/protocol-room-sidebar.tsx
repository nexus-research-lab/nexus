"use client";

import { ArrowRight, Lock } from "lucide-react";

import { cn } from "@/lib/utils";
import {
  ProtocolChannelAggregate,
  ProtocolRunDetail,
  ProtocolRunListItem,
  RoomMemberRecord,
} from "@/types";

import {
  renderChannelIcon,
  renderChannelName,
  renderChannelTopic,
  renderRoleLabel,
  renderSeatStatusLabel,
  renderStatusLabel,
} from "./protocol-room-helpers";

interface ProtocolRoomSidebarProps {
  detail: ProtocolRunDetail;
  room_agent_members: RoomMemberRecord[];
  viewer_agent_id: string | null;
  visible_channels: ProtocolChannelAggregate[];
  selected_channel_id: string | null;
  runs: ProtocolRunListItem[];
  member_request_status: Map<string, "pending" | "submitted" | "idle">;
  alive_agent_ids: Set<string>;
  eliminated_agent_ids: Set<string>;
  roles_by_agent_id: Record<string, string>;
  on_set_viewer: (agent_id: string | null) => void;
  on_select_channel: (channel_id: string) => void;
  on_select_run: (run_id: string) => void;
}

export function ProtocolRoomSidebar({
  detail,
  room_agent_members,
  viewer_agent_id,
  visible_channels,
  selected_channel_id,
  runs,
  member_request_status,
  alive_agent_ids,
  eliminated_agent_ids,
  roles_by_agent_id,
  on_set_viewer,
  on_select_channel,
  on_select_run,
}: ProtocolRoomSidebarProps) {
  return (
    <aside className="neo-card flex min-h-0 flex-col rounded-[28px] p-4">
      <div className="flex items-center justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-700/48">
            当前视角
          </p>
          <p className="mt-1 text-sm font-semibold text-slate-950/88">
            {viewer_agent_id || "观察者"}
          </p>
        </div>
        <select
          className="neo-inset rounded-full px-3 py-2 text-sm text-slate-900/86 outline-none"
          onChange={(event) => on_set_viewer(event.target.value || null)}
          value={viewer_agent_id ?? ""}
        >
          <option value="">观察者</option>
          {room_agent_members.map((member) => (
            <option key={member.id} value={member.member_agent_id ?? ""}>
              {member.member_agent_id}
            </option>
          ))}
        </select>
      </div>

      <div className="mt-5">
        <div className="flex items-center justify-between gap-3">
          <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-700/48">
            成员
          </p>
          <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-700/40">
            {room_agent_members.length} 人
          </span>
        </div>
        <div className="mt-3 space-y-3">
          {room_agent_members.map((member) => {
            const agent_id = member.member_agent_id ?? "";
            const is_alive = alive_agent_ids.has(agent_id);
            const is_eliminated = eliminated_agent_ids.has(agent_id);
            const role = roles_by_agent_id?.[agent_id] || "member";
            return (
              <div
                key={member.id}
                className={cn(
                  "rounded-[22px] border px-4 py-3 transition",
                  is_alive
                    ? "border-emerald-400/24 bg-emerald-500/8"
                    : "border-slate-400/18 bg-slate-500/6",
                )}
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <p className="truncate text-sm font-semibold text-slate-950/90">
                      {agent_id}
                    </p>
                    <div className="mt-2 flex flex-wrap gap-2">
                      <span className="rounded-full bg-white/78 px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-700/60">
                        {renderRoleLabel(role)}
                      </span>
                      <span className="rounded-full bg-white/70 px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-700/60">
                        {renderSeatStatusLabel(
                          member_request_status.get(agent_id) || "idle",
                          is_alive,
                          is_eliminated,
                        )}
                      </span>
                    </div>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      </div>

      <div className="mt-5 min-h-0 flex-1">
        <div className="flex items-center justify-between gap-3">
          <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-700/48">
            频道
          </p>
          <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-700/40">
            {visible_channels.length} 个
          </span>
        </div>
        <div className="mt-3 space-y-2 overflow-y-auto pr-1 scrollbar-hide">
          {visible_channels.map((channel) => {
            const ChannelIcon = renderChannelIcon(channel.channel.channel_type);
            const is_visible = Boolean(channel.channel.metadata?.is_visible);
            const is_selected = channel.channel.id === selected_channel_id;
            return (
              <button
                key={channel.channel.id}
                className={cn(
                  "flex w-full items-start gap-3 rounded-[22px] border px-3 py-3 text-left transition",
                  is_selected
                    ? "border-slate-950/20 bg-slate-950/8 shadow-[0_12px_26px_rgba(15,23,42,0.08)]"
                    : "border-white/60 bg-white/36 hover:bg-white/48",
                )}
                onClick={() => on_select_channel(channel.channel.id)}
                type="button"
              >
                <div className="mt-0.5 rounded-full bg-white/80 p-2">
                  <ChannelIcon className="h-4 w-4 text-slate-900/76" />
                </div>
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <p className="truncate text-sm font-semibold text-slate-950/90">
                      {renderChannelName(channel.channel.slug, channel.channel.name)}
                    </p>
                    {!is_visible ? <Lock className="h-3.5 w-3.5 text-slate-500" /> : null}
                  </div>
                  <p className="mt-1 text-xs leading-5 text-slate-700/56">
                    {renderChannelTopic(channel.channel.slug, channel.channel.topic)}
                  </p>
                </div>
              </button>
            );
          })}
        </div>
      </div>

      <div className="mt-5">
        <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-700/48">
          协作轮次
        </p>
        <div className="mt-3 space-y-2">
          {runs.map((item) => (
            <button
              key={item.run.id}
              className={cn(
                "flex w-full items-center justify-between rounded-[18px] border px-3 py-3 text-left transition",
                item.run.id === detail.run.id
                  ? "border-slate-950/18 bg-slate-950/8"
                  : "border-white/60 bg-white/40 hover:bg-white/52",
              )}
              onClick={() => on_select_run(item.run.id)}
              type="button"
            >
              <div className="min-w-0">
                <p className="truncate text-sm font-semibold text-slate-950/90">
                  {item.run.title || item.definition.name}
                </p>
                <p className="mt-1 text-xs text-slate-700/56">
                  {renderStatusLabel(item.run.status)}
                </p>
              </div>
              <ArrowRight className="h-4 w-4 shrink-0 text-slate-700/42" />
            </button>
          ))}
        </div>
      </div>
    </aside>
  );
}
