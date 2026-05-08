"use client";

import {
  ChevronRight,
  ExternalLink,
  Gamepad2,
  MessageCircle,
  Plus,
  Power,
  RefreshCw,
  Send,
  Sparkles,
  X,
} from "lucide-react";
import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { createPortal } from "react-dom";

import { get_agents } from "@/lib/api/agent-manage-api";
import {
  ChannelConfigView,
  ChannelCredentialField,
  ImChannelType,
  list_channels_api,
  upsert_channel_config_api,
} from "@/lib/api/channel-api";
import { cn } from "@/lib/utils";
import { FeedbackBannerStack, type FeedbackBannerItem } from "@/shared/ui/feedback/feedback-banner-stack";
import {
  WorkspaceSurfaceHeader,
  WorkspaceSurfaceToolbarAction,
} from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import type { Agent } from "@/types/agent/agent";

const CHANNEL_ORDER: ImChannelType[] = ["dingtalk", "wechat", "feishu", "telegram", "discord"];

const CHANNEL_STYLES: Record<ImChannelType, { color: string; icon: typeof Send; cnName: string }> = {
  dingtalk: { color: "#1677ff", icon: Send, cnName: "bg-[#1677ff] text-white" },
  wechat: { color: "#15c45d", icon: MessageCircle, cnName: "bg-[#15c45d] text-white" },
  feishu: { color: "#356bff", icon: Send, cnName: "bg-[#356bff] text-white" },
  telegram: { color: "#28a8ea", icon: Send, cnName: "bg-[#28a8ea] text-white" },
  discord: { color: "#5865f2", icon: Gamepad2, cnName: "bg-[#5865f2] text-white" },
};

function is_channel_planned(item: ChannelConfigView) {
  return item.runtime_status === "planned";
}

function channel_status_text(item: ChannelConfigView) {
  if (is_channel_planned(item)) return "未上线";
  if (!item.configured) return "未关联";
  if (item.connection_state === "connected") return "已连接";
  if (item.connection_state === "error") return "异常";
  return "已配置";
}

function channel_hint(item: ChannelConfigView) {
  if (is_channel_planned(item)) return "未上线";
  return "设置机器人";
}

function guide_steps(channel_type: ImChannelType) {
  switch (channel_type) {
  case "dingtalk":
    return [
      <>前往 <a href="https://open.dingtalk.com/" target="_blank" rel="noreferrer">钉钉开放平台</a> 创建企业内部应用，并添加 <b>机器人能力</b></>,
      <>进入 <b>应用配置</b>，左侧菜单 <b>机器人 → 机器人配置</b>，消息接收模式必须选择 <b>Stream</b> 模式，不要选 Webhook</>,
      <>在 <b>凭证与基础信息</b> 页面复制 <b>Client ID</b> 和 <b>Client Secret</b></>,
      <>先在钉钉侧 <b>发布应用版本</b>，确认应用可见范围包含你的账号</>,
      <>在钉钉群中添加该机器人并 <b>@机器人</b>，或私聊机器人完成配对授权</>,
    ];
  case "wechat":
    return [];
  case "feishu":
    return [
      <>登录 <a href="https://open.feishu.cn/" target="_blank" rel="noreferrer">飞书开放平台</a> 创建企业自建应用，在 <b>应用能力</b> 中添加机器人能力</>,
      <>在 <b>凭证与基础信息</b> 页面获取 <b>App ID</b> 和 <b>App Secret</b></>,
      <>进入 <b>权限管理</b>，批量导入权限，至少包含 <b>im.message.receive_v1</b></>,
      <>进入 <b>事件与回调 → 事件配置 → 订阅方式</b>，使用 <b>长连接</b> 接收事件</>,
      <>创建应用版本并发布，确认可用范围包含目标用户或群</>,
      <>在飞书群中添加该机器人并 <b>@机器人</b>，或私聊机器人完成配对授权</>,
    ];
  case "telegram":
    return [
      <>在 Telegram 中搜索 <a href="https://t.me/BotFather" target="_blank" rel="noreferrer">@BotFather</a>，发送 <b>/newbot</b> 创建机器人</>,
      <>按提示设置机器人名称和用户名，成功后 BotFather 会返回 <b>Bot Token</b></>,
      <>将 <b>Bot Token</b> 填入下方表单，完成连接</>,
      <>在 Telegram 群中添加该机器人并 <b>@机器人</b>，或私聊机器人完成配对授权</>,
    ];
  case "discord":
    return [
      <>打开 <a href="https://discord.com/developers/applications" target="_blank" rel="noreferrer">Discord 开发者平台</a>，点击 <b>New Application</b> 创建应用</>,
      <>进入应用左侧 <b>机器人</b> 页面，点击 <b>Reset Token</b> 获取 Token，并开启 <b>消息内容意图</b></>,
      <>在下方填写凭证，生成 <b>授权链接</b>，打开链接并添加到 <b>服务器</b></>,
    ];
  }
}

function build_discord_oauth_url(config: Record<string, string>) {
  const app_id = config.application_id?.trim();
  if (!app_id) return "";
  const params = new URLSearchParams({
    client_id: app_id,
    permissions: "274877975552",
    scope: "bot applications.commands",
  });
  return `https://discord.com/oauth2/authorize?${params.toString()}`;
}

function ChannelIcon({ type, size = "card" }: { type: ImChannelType; size?: "card" | "dialog" }) {
  const style = CHANNEL_STYLES[type];
  const Icon = style.icon;
  return (
    <span
      className={cn(
        "flex shrink-0 items-center justify-center rounded-[18px] shadow-[0_12px_28px_rgba(15,23,42,0.12)]",
        size === "dialog" ? "h-[52px] w-[52px]" : "h-12 w-12",
        style.cnName,
      )}
    >
      <Icon className={size === "dialog" ? "h-[26px] w-[26px]" : "h-6 w-6"} />
    </span>
  );
}

interface ChannelDialogProps {
  item: ChannelConfigView;
  agents: Agent[];
  on_close: () => void;
  on_saved: (item: ChannelConfigView, announce?: boolean) => void;
  on_error: (message: string) => void;
}

function ChannelConnectDialog({ item, agents, on_close, on_saved, on_error }: ChannelDialogProps) {
  const [current_item, set_current_item] = useState(item);
  const [agent_id, set_agent_id] = useState(item.agent_id || agents[0]?.agent_id || "");
  const [config, set_config] = useState<Record<string, string>>(item.public_config || {});
  const [credentials, set_credentials] = useState<Record<string, string>>({});
  const [saving, set_saving] = useState(false);
  const is_planned = is_channel_planned(current_item);
  const discord_oauth_url = current_item.channel_type === "discord" ? build_discord_oauth_url(config) : "";

  useEffect(() => {
    set_current_item(item);
    set_agent_id(item.agent_id || agents[0]?.agent_id || "");
    set_config(item.public_config || {});
    set_credentials({});
  }, [agents, item]);

  const handle_field_change = (field: ChannelCredentialField, value: string) => {
    if (field.secret) {
      set_credentials((current) => ({ ...current, [field.key]: value }));
      return;
    }
    set_config((current) => ({ ...current, [field.key]: value }));
  };

  const save_channel = useCallback(async (close_on_success: boolean) => {
    if (!agent_id) return;
    if (is_planned) return;
    set_saving(true);
    try {
      const saved = await upsert_channel_config_api(current_item.channel_type, {
        agent_id,
        config,
        credentials,
      });
      set_current_item(saved);
      on_saved(saved);
      if (close_on_success) on_close();
    } catch (error) {
      on_error(error instanceof Error ? error.message : "连接失败");
    } finally {
      set_saving(false);
    }
  }, [agent_id, config, credentials, current_item.channel_type, is_planned, on_close, on_error, on_saved]);

  const handle_submit = async (event: FormEvent) => {
    event.preventDefault();
    await save_channel(true);
  };

  const dialog = (
    <div className="dialog-backdrop z-[9999]" role="dialog" aria-modal="true">
      <form
        className="flex max-h-[86vh] w-full max-w-[680px] flex-col overflow-hidden rounded-[22px] bg-white shadow-[0_30px_90px_rgba(15,23,42,0.26)]"
        onSubmit={handle_submit}
      >
        <div className="flex items-center gap-4 px-7 pb-5 pt-7">
          <ChannelIcon type={current_item.channel_type} size="dialog" />
          <h2 className="flex-1 text-[22px] font-black tracking-normal text-[#111827]">连接 {current_item.title}</h2>
          <button
            aria-label="关闭"
            className="flex h-9 w-9 items-center justify-center rounded-full text-[#8b8f9a] transition hover:bg-[#f3f4f6] hover:text-[#111827]"
            onClick={on_close}
            type="button"
          >
            <X className="h-[22px] w-[22px]" />
          </button>
        </div>

        <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-7 pb-6">
          {is_planned ? (
            <div className="rounded-[16px] border border-dashed border-[#d8dce6] bg-[#fafbfc] px-5 py-6 text-center">
              <div className="text-[15px] font-black text-[#303542]">该消息渠道未上线</div>
              <div className="mx-auto mt-2 max-w-[460px] text-[13px] leading-6 text-[#7c8390]">
                消息渠道接入将在后续版本补充，当前版本暂不支持配置机器人或配对授权。
              </div>
            </div>
          ) : (
            <>
              <div className="rounded-[16px] border border-[#f5d45f] bg-[#fff9e8] px-5 py-4 text-[14px] leading-6 text-[#41444d] shadow-[0_10px_24px_rgba(245,190,36,0.14)]">
                <div className="mb-2 text-[14px] font-black text-[#a83a07]">如何连接：</div>
                <ol className="list-decimal space-y-1 pl-5">
                  {guide_steps(current_item.channel_type).map((step, index) => (
                    <li key={index} className="[&_a]:text-[#1d73ff] [&_b]:font-black">{step}</li>
                  ))}
                </ol>
                {current_item.channel_type === "dingtalk" ? (
                  <div className="mt-4 border-t border-[#f5d45f] pt-3 text-[13px] font-semibold leading-6 text-[#c44707]">
                    钉钉群中，通常需要 @机器人 发送消息；本通道使用官方 Stream 模式长连接。
                  </div>
                ) : null}
                {current_item.channel_type === "feishu" ? (
                  <div className="mt-4 border-t border-[#f5d45f] pt-3 text-[13px] font-semibold leading-6 text-[#c44707]">
                    本通道使用官方飞书长连接 SDK；请确认应用已启用长连接事件订阅。
                  </div>
                ) : null}
              </div>

              {current_item.runtime_note ? (
                <div className="mt-4 rounded-[12px] border border-[#e7eefc] bg-[#f7fbff] px-4 py-3 text-[13px] font-semibold leading-5 text-[#4a5568]">
                  {current_item.runtime_note}
                </div>
              ) : null}

              <label className="mt-6 block text-[15px] font-black text-[#3f424b]">
                处理智能体 <span className="text-[#ff4d4f]">*</span>
                <select
                  className="mt-2 h-12 w-full rounded-[14px] border border-[#e2e5ea] bg-white px-4 text-[14px] font-semibold text-[#22252d] outline-none transition focus:border-[#9bdab8] focus:ring-4 focus:ring-[#9bdab8]/35"
                  onChange={(event) => set_agent_id(event.target.value)}
                  required
                  value={agent_id}
                >
                  {agents.map((agent) => (
                    <option key={agent.agent_id} value={agent.agent_id}>{agent.name}</option>
                  ))}
                </select>
              </label>

              <div className="mt-6 space-y-4">
                {current_item.credential_fields.map((field) => (
                  <label key={field.key} className="block text-[15px] font-black text-[#3f424b]">
                    {field.label} {field.required ? <span className="text-[#ff4d4f]">*</span> : null}
                    <input
                      className="mt-2 h-12 w-full rounded-[14px] border border-[#e2e5ea] bg-white px-4 text-[14px] font-semibold text-[#22252d] outline-none transition placeholder:text-[#a5a9b5] focus:border-[#9bdab8] focus:ring-4 focus:ring-[#9bdab8]/35"
                      onChange={(event) => handle_field_change(field, event.target.value)}
                      placeholder={field.placeholder || ""}
                      required={field.required && !(field.secret && current_item.has_credentials)}
                      type={field.kind === "password" ? "password" : "text"}
                      value={field.secret ? credentials[field.key] || "" : config[field.key] || ""}
                    />
                  </label>
                ))}
              </div>

              {current_item.channel_type === "discord" ? (
                <div className="mt-6">
                  <div className="mb-2 text-[15px] font-black text-[#3f424b]">授权机器人到服务器</div>
                  <button
                    className="flex h-12 w-full items-center justify-center gap-2 rounded-[14px] bg-[#2f7df6] text-[15px] font-black text-white transition hover:bg-[#216fe8] disabled:cursor-not-allowed disabled:opacity-60"
                    disabled={!discord_oauth_url}
                    onClick={() => discord_oauth_url && window.open(discord_oauth_url, "_blank", "noopener,noreferrer")}
                    type="button"
                  >
                    <ExternalLink className="h-5 w-5" />
                    授权机器人
                  </button>
                </div>
              ) : null}
            </>
          )}

        </div>

        <div className="grid grid-cols-2 gap-3 px-7 pb-7">
          <button
            className="h-12 rounded-[14px] border border-[#e1e4e9] text-[15px] font-black text-[#4a4d56] transition hover:bg-[#f7f8fa]"
            onClick={on_close}
            type="button"
          >
            取消
          </button>
          <button
            className="flex h-12 items-center justify-center gap-2 rounded-[14px] bg-[#2f7df6] text-[15px] font-black text-white transition hover:bg-[#216fe8] disabled:cursor-not-allowed disabled:opacity-60"
            disabled={saving || !agent_id || is_planned}
            type="submit"
          >
            <Power className="h-5 w-5" />
            {is_planned ? "未上线" : saving ? "连接中..." : "连接"}
          </button>
        </div>
      </form>
    </div>
  );

  return createPortal(dialog, document.body);
}

function ChannelCard({
  item,
  on_configure,
}: {
  item: ChannelConfigView;
  on_configure: (item: ChannelConfigView) => void;
}) {
  const planned = is_channel_planned(item);

  return (
    <article className="rounded-[16px] border border-[#e7e8ec] bg-white px-6 py-6 shadow-[0_1px_0_rgba(15,23,42,0.02)]">
      <div className="flex items-start gap-4">
        <ChannelIcon type={item.channel_type} />
        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 items-center gap-2">
            <h3 className="truncate text-[20px] font-black tracking-normal text-[#20222a]">{item.title}</h3>
            {!planned && item.docs_url ? (
              <a className="text-[13px] font-semibold text-[#2b74ff] underline-offset-2 hover:underline" href={item.docs_url} target="_blank" rel="noreferrer">
                如何接入？
              </a>
            ) : null}
          </div>
          <p className="mt-1 truncate text-[14px] text-[#777d8a]">{item.bot_label}</p>
        </div>
        <span className={cn(
          "rounded-full px-3 py-1 text-[12px] font-semibold",
          planned
            ? "bg-[#f2f4f7] text-[#98a2b3]"
            : item.connection_state === "connected"
            ? "bg-[#ecfdf3] text-[#067647]"
            : item.runtime_status === "external_adapter"
              ? "bg-[#fff7ed] text-[#c2410c]"
              : "bg-[#f6f7f9] text-[#9aa0ad]",
        )}>
          {channel_status_text(item)}
        </span>
      </div>

      <div className="mt-7 grid grid-cols-3 gap-2">
        <div className="rounded-[12px] border border-[#e7e8ec] px-4 py-3">
          <div className="text-[13px] text-[#777d8a]">已配对用户</div>
          <div className="mt-1 text-right text-[20px] font-black text-[#20222a]">{item.stats.paired_user_count}</div>
        </div>
        <div className="rounded-[12px] border border-[#e7e8ec] px-4 py-3">
          <div className="text-[13px] text-[#777d8a]">{item.supports_group ? "已配对群聊" : "不支持群聊配对"}</div>
          <div className="mt-1 text-right text-[20px] font-black text-[#20222a]">{item.supports_group ? item.stats.paired_group_count : "-"}</div>
        </div>
        <div className="rounded-[12px] border border-[#e7e8ec] px-4 py-3">
          <div className="text-[13px] text-[#777d8a]">待处理请求</div>
          <div className="mt-1 text-right text-[20px] font-black text-[#20222a]">{item.stats.pending_count}</div>
        </div>
      </div>

      <button
        className={cn(
          "mt-6 flex h-20 w-full items-center gap-4 rounded-[14px] border border-dashed border-[#d8dbe3] px-4 text-left transition",
          planned
            ? "cursor-not-allowed bg-[#fafafa] text-[#9aa1ad]"
            : "hover:border-[#b8c4d9] hover:bg-[#fafcff]",
        )}
        disabled={planned}
        onClick={() => {
          if (!planned) on_configure(item);
        }}
        type="button"
      >
        <Plus className="h-5 w-5 shrink-0 text-[#98a0ad]" />
        <span className="min-w-0 flex-1">
          <span className={cn("block text-[15px] font-black", planned ? "text-[#7d8490]" : "text-[#2f333d]")}>
            {planned ? "未上线" : item.configured ? item.agent_name || "已配置智能体" : "请配置智能体"}
          </span>
          <span className="mt-1 block truncate text-[12px] text-[#777d8a]">
            {planned ? "该消息渠道将在后续版本补充" : "选择一个智能体来处理此渠道的消息"}
          </span>
        </span>
        {!planned ? <ChevronRight className="h-5 w-5 shrink-0 text-[#a7adba]" /> : null}
      </button>

      <button
        className={cn(
          "mt-6 flex h-12 w-full items-center justify-center gap-2 rounded-[14px] border border-[#e0e3e8] bg-white text-[15px] font-black transition",
          planned
            ? "cursor-not-allowed text-[#9aa1ad]"
            : "text-[#282c35] hover:bg-[#f7f8fa]",
        )}
        disabled={planned}
        onClick={() => {
          if (!planned) on_configure(item);
        }}
        type="button"
      >
        <Sparkles className="h-4 w-4" />
        {channel_hint(item)}
      </button>
    </article>
  );
}

export function ChannelsDirectory() {
  const [channels, set_channels] = useState<ChannelConfigView[]>([]);
  const [agents, set_agents] = useState<Agent[]>([]);
  const [selected, set_selected] = useState<ChannelConfigView | null>(null);
  const [loading, set_loading] = useState(true);
  const [feedback, set_feedback] = useState<{ tone: "success" | "error"; title: string; message: string } | null>(null);

  const sorted_channels = useMemo(() => {
    return [...channels].sort((left, right) => CHANNEL_ORDER.indexOf(left.channel_type) - CHANNEL_ORDER.indexOf(right.channel_type));
  }, [channels]);

  const refresh = async () => {
    set_loading(true);
    try {
      const [next_channels, next_agents] = await Promise.all([list_channels_api(), get_agents()]);
      set_channels(next_channels);
      set_agents(next_agents);
    } catch (error) {
      set_feedback({ tone: "error", title: "加载失败", message: error instanceof Error ? error.message : "消息渠道加载失败" });
    } finally {
      set_loading(false);
    }
  };

  useEffect(() => {
    void refresh();
  }, []);

  const handle_channel_saved = useCallback((item: ChannelConfigView, announce = true) => {
    set_channels((current) => current.map((value) => value.channel_type === item.channel_type ? item : value));
    if (announce) {
      set_feedback({ tone: "success", title: "连接成功", message: `${item.title} 已完成配置` });
    }
  }, []);

  const feedback_items: FeedbackBannerItem[] = feedback
    ? [{
        key: "channels-feedback",
        tone: feedback.tone,
        title: feedback.title,
        message: feedback.message,
        on_dismiss: () => set_feedback(null),
      }]
    : [];

  return (
    <>
      <WorkspaceSurfaceScaffold
        body_class_name="px-6 py-5"
        body_scrollable
        header={(
          <WorkspaceSurfaceHeader
            badge={`${channels.length || 5} 个渠道`}
            density="compact"
            leading={<MessageCircle className="h-4 w-4" />}
            subtitle="IM 渠道接入将在后续版本开放，当前仅保留入口骨架与本地配对数据结构。"
            title="消息渠道"
            trailing={(
              <WorkspaceSurfaceToolbarAction onClick={() => void refresh()}>
                <RefreshCw className="h-3.5 w-3.5" />
                刷新
              </WorkspaceSurfaceToolbarAction>
            )}
          />
        )}
        stable_gutter
      >
        {loading ? (
          <div className="flex h-40 items-center justify-center text-[13px] text-[#8d95a3]">加载消息渠道...</div>
        ) : (
          <div className="grid grid-cols-1 gap-5 xl:grid-cols-2 2xl:grid-cols-3">
            {sorted_channels.map((item) => (
              <ChannelCard item={item} key={item.channel_type} on_configure={set_selected} />
            ))}
          </div>
        )}
      </WorkspaceSurfaceScaffold>

      {selected ? (
        <ChannelConnectDialog
          agents={agents}
          item={selected}
          on_close={() => set_selected(null)}
          on_error={(message) => set_feedback({ tone: "error", title: "连接失败", message })}
          on_saved={handle_channel_saved}
        />
      ) : null}

      <FeedbackBannerStack items={feedback_items} />
    </>
  );
}
