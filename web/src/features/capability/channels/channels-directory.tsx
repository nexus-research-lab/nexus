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
} from "lucide-react";
import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";

import { get_agents } from "@/lib/api/agent-manage-api";
import {
  ChannelConfigView,
  ChannelCredentialField,
  ImChannelType,
  list_channels_api,
  upsert_channel_config_api,
} from "@/lib/api/channel-api";
import { cn } from "@/lib/utils";
import { UiBadge } from "@/shared/ui/badge";
import { UiButton } from "@/shared/ui/button";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogFormShell,
  UiDialogHeader,
  UiDialogPortal,
} from "@/shared/ui/dialog/dialog";
import {
  get_dialog_note_class_name,
  get_dialog_note_style,
} from "@/shared/ui/dialog/dialog-styles";
import { FeedbackBannerStack, type FeedbackBannerItem } from "@/shared/ui/feedback/feedback-banner-stack";
import { UiField, UiInput } from "@/shared/ui/form-control";
import { UiPanel } from "@/shared/ui/panel";
import { UiSelectMenu } from "@/shared/ui/select-menu";
import { UiSkeleton } from "@/shared/ui/skeleton";
import { UiStateBlock } from "@/shared/ui/state-block";
import {
  WorkspaceSurfaceHeader,
  WorkspaceSurfaceToolbarAction,
} from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import {
  WorkspaceCatalogAction,
  WorkspaceCatalogBody,
  WorkspaceCatalogCard,
  WorkspaceCatalogDescription,
  WorkspaceCatalogFooter,
  WorkspaceCatalogHeader,
  WorkspaceCatalogTag,
  WorkspaceCatalogTextAction,
  WorkspaceCatalogTitle,
} from "@/shared/ui/workspace/catalog/workspace-catalog-card";
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
        "flex shrink-0 items-center justify-center border border-white/35 shadow-(--surface-avatar-shadow)",
        size === "dialog" ? "h-[52px] w-[52px] rounded-[18px]" : "h-11 w-11 rounded-[16px]",
        style.cnName,
      )}
    >
      <Icon className={size === "dialog" ? "h-[26px] w-[26px]" : "h-5 w-5"} />
    </span>
  );
}

function ChannelStatePill({
  children,
  tone = "neutral",
}: {
  children: string;
  tone?: "neutral" | "success" | "warning" | "danger" | "info";
}) {
  return (
    <UiBadge tone={tone === "neutral" ? "default" : tone}>
      {children}
    </UiBadge>
  );
}

function ChannelMetric({
  label,
  value,
}: {
  label: string;
  value: string | number;
}) {
  return (
    <div className="min-w-0">
      <div className="truncate text-[10.5px] font-medium text-(--text-soft)">
        {label}
      </div>
      <div className="mt-1 truncate text-[16px] font-semibold tracking-[-0.02em] text-(--text-strong)">
        {value}
      </div>
    </div>
  );
}

interface ChannelDialogProps {
  item: ChannelConfigView;
  agents: Agent[];
  on_close: () => void;
  on_saved: (item: ChannelConfigView, announce?: boolean) => void;
  on_error: (message: string) => void;
}

function ChannelGuide({
  item,
}: {
  item: ChannelConfigView;
}) {
  const steps = guide_steps(item.channel_type);

  if (steps.length === 0) {
    return null;
  }

  return (
    <div className={get_dialog_note_class_name("default")} style={get_dialog_note_style("default")}>
      <div className="mb-2 text-[13px] font-semibold text-(--text-strong)">如何连接</div>
      <ol className="list-decimal space-y-1 pl-5 text-[13px] leading-6 text-(--text-default)">
        {steps.map((step, index) => (
          <li key={index} className="[&_a]:font-semibold [&_a]:text-(--primary) [&_b]:font-semibold">
            {step}
          </li>
        ))}
      </ol>
      {item.channel_type === "dingtalk" ? (
        <div className="mt-4 border-t border-(--divider-subtle-color) pt-3 text-[12px] font-medium leading-5 text-(--text-muted)">
          钉钉群中，通常需要 @机器人 发送消息；本通道使用官方 Stream 模式长连接。
        </div>
      ) : null}
      {item.channel_type === "feishu" ? (
        <div className="mt-4 border-t border-(--divider-subtle-color) pt-3 text-[12px] font-medium leading-5 text-(--text-muted)">
          本通道使用官方飞书长连接 SDK；请确认应用已启用长连接事件订阅。
        </div>
      ) : null}
    </div>
  );
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

  return (
    <UiDialogPortal>
      <UiDialogBackdrop class_name="z-[9999]" labelled_by="channel-connect-dialog-title" on_close={on_close}>
        <UiDialogFormShell
          class_name="max-h-[86vh]"
          onSubmit={handle_submit}
          size="lg"
        >
          <UiDialogHeader
            icon={<ChannelIcon type={current_item.channel_type} size="dialog" />}
            icon_class_name="h-[52px] w-[52px] overflow-visible border-0 bg-transparent p-0 shadow-none"
            on_close={on_close}
            title={`连接 ${current_item.title}`}
            title_id="channel-connect-dialog-title"
          />

          <UiDialogBody class_name="space-y-5" scrollable>
            {is_planned ? (
              <UiStateBlock
                description="消息渠道接入将在后续版本补充，当前版本暂不支持配置机器人或配对授权。"
                size="sm"
                title="该消息渠道未上线"
                variant="inset"
              />
            ) : (
              <>
                <ChannelGuide item={current_item} />

                {current_item.runtime_note ? (
                  <div className="rounded-[14px] border border-(--divider-subtle-color) bg-(--surface-inset-background) px-4 py-3 text-[13px] font-medium leading-5 text-(--text-default)">
                    {current_item.runtime_note}
                  </div>
                ) : null}

                <UiField label={<>处理智能体 <span className="text-(--destructive)">*</span></>}>
                  <UiSelectMenu
                    aria_label="选择消息渠道处理智能体"
                    on_change={set_agent_id}
                    options={agents.map((agent) => ({
                      value: agent.agent_id,
                      label: agent.name,
                    }))}
                    size="sm"
                    value={agent_id}
                  />
                </UiField>

                <div className="space-y-4">
                  {current_item.credential_fields.map((field) => (
                    <UiField
                      key={field.key}
                      label={(
                        <>
                          {field.label} {field.required ? <span className="text-(--destructive)">*</span> : null}
                        </>
                      )}
                    >
                      <UiInput
                        onChange={(event) => handle_field_change(field, event.target.value)}
                        placeholder={field.placeholder || ""}
                        required={field.required && !(field.secret && current_item.has_credentials)}
                        type={field.kind === "password" ? "password" : "text"}
                        value={field.secret ? credentials[field.key] || "" : config[field.key] || ""}
                        variant="dialog"
                      />
                    </UiField>
                  ))}
                </div>

                {current_item.channel_type === "discord" ? (
                  <UiField label="授权机器人到服务器">
                    <UiButton
                      class_name="w-full"
                      disabled={!discord_oauth_url}
                      onClick={() => discord_oauth_url && window.open(discord_oauth_url, "_blank", "noopener,noreferrer")}
                      size="lg"
                      tone="primary"
                      type="button"
                      variant="solid"
                    >
                      <ExternalLink className="h-5 w-5" />
                      授权机器人
                    </UiButton>
                  </UiField>
                ) : null}
              </>
            )}

          </UiDialogBody>

          <UiDialogFooter>
            <UiButton
              class_name="min-w-[104px]"
              onClick={on_close}
              size="lg"
              type="button"
            >
              取消
            </UiButton>
            <UiButton
              class_name="min-w-[124px]"
              disabled={saving || !agent_id || is_planned}
              size="lg"
              tone="primary"
              type="submit"
              variant="solid"
            >
              <Power className="h-5 w-5" />
              {is_planned ? "未上线" : saving ? "连接中..." : "连接"}
            </UiButton>
          </UiDialogFooter>
        </UiDialogFormShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}

function ChannelCard({
  item,
  on_configure,
}: {
  item: ChannelConfigView;
  on_configure: (item: ChannelConfigView) => void;
}) {
  const planned = is_channel_planned(item);
  const connected = item.connection_state === "connected";
  const state_tone = planned
    ? "neutral"
    : connected
      ? "success"
      : item.connection_state === "error"
        ? "danger"
        : item.runtime_status === "external_adapter"
          ? "warning"
          : item.configured
            ? "info"
            : "neutral";
  const description = planned
    ? "该消息渠道将在后续版本补充，目前仅保留入口和信息结构。"
    : item.configured
      ? `由 ${item.agent_name || "已配置智能体"} 处理该渠道消息。`
      : "选择一个智能体并填写机器人凭证后，即可开始处理来自该渠道的消息。";

  return (
    <WorkspaceCatalogCard
      class_name="group h-full"
      interactive={!planned}
      muted={planned}
      onClick={() => {
        if (!planned) on_configure(item);
      }}
      size="catalog"
    >
      <WorkspaceCatalogHeader class_name="items-center gap-3.5">
        <ChannelIcon type={item.channel_type} />
        <div className="min-w-0 flex-1">
          <WorkspaceCatalogTitle class_name="min-w-0" size="sm" truncate>
            {item.title}
          </WorkspaceCatalogTitle>
          <div className="mt-1 flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-[11px] text-(--text-soft)">
            <span className="truncate">{item.bot_label}</span>
            {item.runtime_status === "external_adapter" ? <span>外部适配器</span> : null}
          </div>
        </div>
        <ChannelStatePill tone={state_tone}>
          {channel_status_text(item)}
        </ChannelStatePill>
      </WorkspaceCatalogHeader>

      <WorkspaceCatalogBody class_name="space-y-4" grow>
        <WorkspaceCatalogDescription lines={2} min_height>
          {description}
        </WorkspaceCatalogDescription>

        <div className="grid grid-cols-3 gap-3 border-y border-(--divider-subtle-color) py-3">
          <ChannelMetric label="用户" value={item.stats.paired_user_count} />
          <ChannelMetric label="群聊" value={item.supports_group ? item.stats.paired_group_count : "-"} />
          <ChannelMetric label="待处理" value={item.stats.pending_count} />
        </div>

        {item.runtime_note ? (
          <p className="line-clamp-2 text-[11.5px] leading-5 text-(--text-soft)">
            {item.runtime_note}
          </p>
        ) : null}
      </WorkspaceCatalogBody>

      <WorkspaceCatalogFooter class_name="items-center">
        <div className="flex min-w-0 flex-wrap items-center gap-1.5">
          <WorkspaceCatalogTag class_name="px-2.5 text-[10px] text-(--text-soft)">
            {item.configured ? "已绑定智能体" : "待配置"}
          </WorkspaceCatalogTag>
          {!item.supports_group ? (
            <WorkspaceCatalogTag class_name="px-2.5 text-[10px] text-(--text-soft)">
              仅私聊
            </WorkspaceCatalogTag>
          ) : null}
        </div>

        <div className="flex shrink-0 items-center gap-1.5" onClick={(event) => event.stopPropagation()}>
          {!planned && item.docs_url ? (
            <WorkspaceCatalogAction
              onClick={() => window.open(item.docs_url, "_blank", "noopener,noreferrer")}
              size="sm"
              title="查看接入文档"
            >
              <ExternalLink className="h-3 w-3" />
            </WorkspaceCatalogAction>
          ) : null}
          {!planned ? (
            <WorkspaceCatalogTextAction
              onClick={() => on_configure(item)}
              tone="primary"
            >
              <Sparkles className="h-3.5 w-3.5" />
              {channel_hint(item)}
              <ChevronRight className="h-3.5 w-3.5" />
            </WorkspaceCatalogTextAction>
          ) : (
            <WorkspaceCatalogTextAction disabled>
              <Plus className="h-3.5 w-3.5" />
              未上线
            </WorkspaceCatalogTextAction>
          )}
        </div>
      </WorkspaceCatalogFooter>
    </WorkspaceCatalogCard>
  );
}

function ChannelLoadingGrid() {
  return (
    <div className="grid grid-cols-1 gap-4 xl:grid-cols-2 2xl:grid-cols-3">
      {Array.from({ length: 5 }, (_, index) => (
        <UiPanel class_name="min-h-[190px]" key={index} radius="lg" padding="lg">
          <div className="flex items-center gap-3.5">
            <UiSkeleton class_name="h-11 w-11 rounded-[16px]" />
            <span className="min-w-0 flex-1 space-y-2">
              <UiSkeleton class_name="h-4 w-24" />
              <UiSkeleton class_name="h-3 w-32" />
            </span>
          </div>
          <div className="mt-5 space-y-2">
            <UiSkeleton class_name="h-3 w-full" />
            <UiSkeleton class_name="h-3 w-2/3" />
          </div>
          <div className="mt-5 grid grid-cols-3 gap-3 border-y border-(--divider-subtle-color) py-3">
            {[0, 1, 2].map((metric) => (
              <span className="space-y-2" key={metric}>
                <UiSkeleton class_name="h-2.5 w-12" />
                <UiSkeleton class_name="h-4 w-8" />
              </span>
            ))}
          </div>
        </UiPanel>
      ))}
    </div>
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
        body_scrollable
        header={(
          <WorkspaceSurfaceHeader
            badge={`${channels.length || 5} 个渠道`}
            density="compact"
            leading={<MessageCircle className="h-4 w-4" />}
            subtitle="统一管理 IM 渠道、机器人凭证与配对授权。"
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
        <div className="mx-auto w-full max-w-[1180px] px-6 py-5">
          {loading ? (
            <ChannelLoadingGrid />
          ) : (
            <div className="grid grid-cols-1 gap-4 xl:grid-cols-2 2xl:grid-cols-3">
              {sorted_channels.map((item) => (
                <ChannelCard item={item} key={item.channel_type} on_configure={set_selected} />
              ))}
            </div>
          )}
        </div>
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
