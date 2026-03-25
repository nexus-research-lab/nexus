"use client";

import { Send } from "lucide-react";

import { cn, formatRelativeTime } from "@/lib/utils";
import { ProtocolActionRequestRecord, ProtocolChannelAggregate, ProtocolSnapshotRecord } from "@/types";

import {
  PROTOCOL_EVENT_STYLE_MAP,
  renderActionLabel,
  renderChannelName,
  renderChannelTopic,
  renderChannelVisibilityLabel,
  renderEventIcon,
  renderFeedBody,
  renderFeedHeadline,
  renderPhaseLabel,
  renderEventTypeLabel,
  toTimestamp,
} from "./protocol-room-helpers";

interface ProtocolRoomFeedProps {
  selected_channel: ProtocolChannelAggregate | null;
  selected_channel_events: ProtocolSnapshotRecord[];
  selected_channel_requests: ProtocolActionRequestRecord[];
  request_payloads: Record<string, Record<string, string>>;
  request_actors: Record<string, string>;
  busy_request_id: string | null;
  inject_channel_id: string;
  inject_message: string;
  composer_actor_id: string;
  visible_channel_member_agent_ids: string[];
  is_busy_control: boolean;
  on_change_request_actor: (request_id: string, actor_id: string) => void;
  on_change_request_payload: (request_id: string, field_name: string, value: string) => void;
  on_submit_request: (request: ProtocolActionRequestRecord, as_override: boolean) => Promise<void>;
  on_change_inject_message: (value: string) => void;
  on_change_composer_actor: (value: string) => void;
  on_send_room_message: () => Promise<void>;
}

export function ProtocolRoomFeed({
  selected_channel,
  selected_channel_events,
  selected_channel_requests,
  request_payloads,
  request_actors,
  busy_request_id,
  inject_channel_id,
  inject_message,
  composer_actor_id,
  visible_channel_member_agent_ids,
  is_busy_control,
  on_change_request_actor,
  on_change_request_payload,
  on_submit_request,
  on_change_inject_message,
  on_change_composer_actor,
  on_send_room_message,
}: ProtocolRoomFeedProps) {
  return (
    <main className="neo-card flex min-h-0 flex-col rounded-[28px] p-4">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-white/55 pb-3">
        <div className="min-w-0">
          <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-700/48">
            房间流
          </p>
          <h2 className="mt-1 text-[24px] font-black tracking-[-0.05em] text-slate-950/92">
            {selected_channel ? renderChannelName(selected_channel.channel.slug, selected_channel.channel.name) : "未选择频道"}
          </h2>
          <p className="mt-2 text-sm leading-6 text-slate-700/62">
            {selected_channel
              ? renderChannelTopic(selected_channel.channel.slug, selected_channel.channel.topic)
              : "请选择一个频道，查看该频道下的房间流和互动。"}
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          {selected_channel ? (
            <div className="neo-card-flat rounded-full px-3 py-2 text-xs font-semibold uppercase tracking-[0.14em] text-slate-700/56">
              {renderChannelVisibilityLabel(selected_channel.channel.visibility)}
            </div>
          ) : null}
        </div>
      </div>

      <div className="mt-4 min-h-0 flex-1 overflow-y-auto pr-1 scrollbar-hide">
        <div className="space-y-3">
          {selected_channel_events.map((snapshot) => {
            const EventIcon = renderEventIcon(snapshot.event_type);
            const relative_time = formatRelativeTime(toTimestamp(snapshot.created_at));
            return (
              <article
                key={snapshot.id}
                className={cn(
                  "rounded-[24px] border px-4 py-4",
                  PROTOCOL_EVENT_STYLE_MAP[snapshot.event_type] ?? "border-white/60 bg-white/40",
                )}
              >
                <div className="flex items-start gap-3">
                  <div className="rounded-full bg-white/78 p-2">
                    <EventIcon className="h-4 w-4 text-slate-900/78" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.15em] text-slate-700/48">
                      <span>{renderEventTypeLabel(snapshot.event_type)}</span>
                      <span>{renderPhaseLabel(snapshot.phase_name)}</span>
                      <span>{relative_time}</span>
                      {snapshot.metadata?.redacted ? <span>受限</span> : null}
                    </div>
                    <h3 className="mt-2 text-sm font-semibold text-slate-950/90">
                      {renderFeedHeadline(snapshot)}
                    </h3>
                    <p className="mt-2 text-sm leading-6 text-slate-700/66">
                      {renderFeedBody(snapshot)}
                    </p>
                  </div>
                </div>
              </article>
            );
          })}

          {selected_channel_requests.map((request) => {
            const actor_options = request.allowed_actor_agent_ids;
            const fields = Array.isArray(request.input_schema?.fields)
              ? request.input_schema.fields
              : [];

            return (
              <div key={request.id} className="rounded-[24px] border border-amber-400/28 bg-amber-500/10 px-4 py-4">
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <p className="text-sm font-semibold text-slate-950/90">
                      {renderActionLabel(request.action_type)}
                    </p>
                    <p className="mt-1 text-xs uppercase tracking-[0.14em] text-slate-700/46">
                      {renderPhaseLabel(request.phase_name)}
                    </p>
                  </div>
                  <div className="rounded-full bg-white/74 px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-700/56">
                    待处理
                  </div>
                </div>

                {request.prompt_text ? (
                  <p className="mt-3 text-sm leading-6 text-slate-700/64">
                    {request.prompt_text}
                  </p>
                ) : null}

                {actor_options.length ? (
                  <div className="mt-3">
                    <label className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-700/48">
                      执行成员
                    </label>
                    <select
                      className="neo-inset mt-1.5 w-full rounded-[16px] px-3 py-2 text-sm text-slate-900/86 outline-none"
                      onChange={(event) => on_change_request_actor(request.id, event.target.value)}
                      value={request_actors[request.id] ?? actor_options[0] ?? ""}
                    >
                      {actor_options.map((agent_id) => (
                        <option key={agent_id} value={agent_id}>
                          {agent_id}
                        </option>
                      ))}
                    </select>
                  </div>
                ) : null}

                {fields.map((field: Record<string, any>) => {
                  const field_name = String(field.name || "");
                  const current_value = request_payloads[request.id]?.[field_name] ?? "";
                  const options = Array.isArray(field.options)
                    ? field.options
                    : request.target_scope?.candidate_agent_ids ?? [];

                  if (field.type === "agent_id") {
                    return (
                      <div key={field_name} className="mt-3">
                        <label className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-700/48">
                          {field.label || field_name}
                        </label>
                        <select
                          className="neo-inset mt-1.5 w-full rounded-[16px] px-3 py-2 text-sm text-slate-900/86 outline-none"
                          onChange={(event) => on_change_request_payload(request.id, field_name, event.target.value)}
                          value={current_value}
                        >
                          <option value="">请选择目标</option>
                          {options.map((option: string) => (
                            <option key={option} value={option}>
                              {option}
                            </option>
                          ))}
                        </select>
                      </div>
                    );
                  }

                  return (
                    <div key={field_name} className="mt-3">
                      <label className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-700/48">
                        {field.label || field_name}
                      </label>
                      <textarea
                        className="neo-inset mt-1.5 min-h-[96px] w-full resize-y rounded-[16px] px-3 py-2.5 text-sm leading-6 text-slate-900/86 outline-none"
                        onChange={(event) => on_change_request_payload(request.id, field_name, event.target.value)}
                        placeholder={field.placeholder || "输入内容"}
                        value={current_value}
                      />
                    </div>
                  );
                })}

                <div className="mt-4 flex flex-wrap gap-2">
                  <button
                    className="inline-flex items-center gap-2 rounded-full bg-slate-950 px-4 py-2 text-sm font-semibold text-white transition hover:translate-y-[-1px]"
                    disabled={busy_request_id === request.id}
                    onClick={() => void on_submit_request(request, false)}
                    type="button"
                  >
                    <Send className="h-4 w-4" />
                    提交动作
                  </button>
                  <button
                    className="neo-pill inline-flex items-center gap-2 rounded-full px-4 py-2 text-sm font-semibold text-slate-900 transition hover:translate-y-[-1px]"
                    disabled={busy_request_id === request.id}
                    onClick={() => void on_submit_request(request, true)}
                    type="button"
                  >
                    <Zap className="h-4 w-4" />
                    强制覆盖
                  </button>
                </div>
              </div>
            );
          })}

          {!selected_channel_events.length && !selected_channel_requests.length ? (
            <div className="flex min-h-[180px] items-center justify-center rounded-[24px] border border-dashed border-white/60 bg-white/24 px-6 text-center text-sm leading-6 text-slate-700/56">
              当前频道还没有任何消息或动作，请等待流程推进，或者直接在下方参与发言。
            </div>
          ) : null}
        </div>
      </div>

      <div className="mt-4 border-t border-white/55 pt-4">
        <div className="rounded-[26px] border border-white/60 bg-white/46 p-4 shadow-[inset_0_0_0_1px_rgba(255,255,255,0.12)]">
          <div className="flex flex-wrap items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-700/48">
            <span>房间发言</span>
            <span>{selected_channel ? renderChannelName(selected_channel.channel.slug, selected_channel.channel.name) : "未选择频道"}</span>
            <span>{composer_actor_id ? `发言身份：${composer_actor_id}` : "发言身份：观察者"}</span>
          </div>

          <div className="mt-3 flex flex-wrap items-center gap-2">
            <select
              className="neo-inset rounded-full px-3 py-2 text-sm text-slate-900/86 outline-none"
              onChange={(event) => on_change_composer_actor(event.target.value)}
              value={composer_actor_id}
            >
              <option value="">观察者 / 主持人</option>
              {visible_channel_member_agent_ids.map((agent_id) => (
                <option key={agent_id} value={agent_id}>
                  {agent_id}
                </option>
              ))}
            </select>
          </div>

          <textarea
            className="neo-inset mt-3 min-h-[112px] w-full resize-y rounded-[18px] px-4 py-3 text-sm leading-6 text-slate-900/86 outline-none"
            onChange={(event) => on_change_inject_message(event.target.value)}
            placeholder="在这个房间频道里发言、补充线索、回应其他成员，或者直接给他们一个新的推动信息……"
            value={inject_message}
          />

          <div className="mt-3 flex items-center justify-between gap-3">
            <p className="text-xs leading-5 text-slate-700/56">
              这里的发言会直接进入当前频道的房间流，更像多人协作对话，而不是一条额外的系统命令。
            </p>
            <button
              className="inline-flex items-center gap-2 rounded-full bg-slate-950 px-4 py-2 text-sm font-semibold text-white transition hover:translate-y-[-1px] disabled:cursor-not-allowed disabled:opacity-50"
              disabled={!inject_channel_id || !inject_message.trim() || is_busy_control}
              onClick={() => void on_send_room_message()}
              type="button"
            >
              <Send className="h-4 w-4" />
              发送到房间
            </button>
          </div>
        </div>
      </div>
    </main>
  );
}
