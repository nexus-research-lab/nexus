"use client";

import {
  CircleCheck,
  Clock3,
  ExternalLink,
  Gamepad2,
  Loader2,
  MessageCircle,
  Power,
  QrCode,
  RefreshCw,
  Send,
  Settings2,
  SlidersHorizontal,
  Terminal,
  TriangleAlert,
} from "lucide-react";
import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";

import { get_agents } from "@/lib/api/agent-manage-api";
import {
  ChannelLoginView,
  ChannelConfigView,
  ChannelCredentialField,
  ImChannelType,
  get_channel_login_api,
  list_channels_api,
  start_channel_login_api,
  submit_channel_login_verify_code_api,
  upsert_channel_config_api,
} from "@/lib/api/channel-api";
import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
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
import { UiListActionButton } from "@/shared/ui/list-action";
import { UiListRow } from "@/shared/ui/list-row";
import { UiSelectMenu } from "@/shared/ui/select-menu";
import { UiStateBlock } from "@/shared/ui/state-block";
import {
  CapabilityFilterBar,
  CapabilityFilterSearchInput,
  CapabilityFilterSelect,
  CapabilityPageLayout,
  CapabilitySectionHeader,
} from "@/features/capability/shared/capability-page-layout";
import {
  WorkspaceSurfaceHeader,
  WorkspaceSurfaceToolbarAction,
} from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import type { Agent } from "@/types/agent/agent";

const CHANNEL_ORDER: ImChannelType[] = ["dingtalk", "wechat", "weixin-personal", "feishu", "telegram", "discord"];
const ENABLED_CHANNEL_TYPES = new Set<ImChannelType>(["weixin-personal", "feishu"]);
type ChannelFilter = "all" | "connected" | "configured" | "unconfigured" | "planned";

const CHANNEL_FILTER_OPTIONS: ReadonlyArray<{ value: ChannelFilter; label_key: TranslationKey }> = [
  { value: "all", label_key: "capability.channels_filter_all" },
  { value: "connected", label_key: "capability.channels_filter_connected" },
  { value: "configured", label_key: "capability.channels_filter_configured" },
  { value: "unconfigured", label_key: "capability.channels_filter_unconfigured" },
  { value: "planned", label_key: "capability.channels_filter_planned" },
];

const CHANNEL_STYLES: Record<ImChannelType, { color: string; icon: typeof Send; cnName: string }> = {
  dingtalk: { color: "#1677ff", icon: Send, cnName: "bg-[#1677ff] text-white" },
  wechat: { color: "#15c45d", icon: MessageCircle, cnName: "bg-[#15c45d] text-white" },
  "weixin-personal": { color: "#10a36a", icon: QrCode, cnName: "bg-[#10a36a] text-white" },
  feishu: { color: "#356bff", icon: Send, cnName: "bg-[#356bff] text-white" },
  telegram: { color: "#28a8ea", icon: Send, cnName: "bg-[#28a8ea] text-white" },
  discord: { color: "#5865f2", icon: Gamepad2, cnName: "bg-[#5865f2] text-white" },
};

function is_channel_enabled(channel_type: ImChannelType) {
  return ENABLED_CHANNEL_TYPES.has(channel_type);
}

function is_channel_closed(item: ChannelConfigView) {
  return !is_channel_enabled(item.channel_type);
}

function is_channel_planned(item: ChannelConfigView) {
  return item.runtime_status === "planned" || is_channel_closed(item);
}

function is_personal_weixin_channel(channel_type: ImChannelType) {
  return channel_type === "weixin-personal";
}

function channel_status_text(item: ChannelConfigView) {
  if (is_channel_closed(item)) return "未开放";
  if (is_channel_planned(item)) return "未上线";
  if (!item.configured) return "未关联";
  if (item.connection_state === "connected") return "已连接";
  if (item.connection_state === "error") return "异常";
  return "已配置";
}

function guide_steps(channel_type: ImChannelType) {
  switch (channel_type) {
  case "dingtalk":
    return [
      <>前往 <a href="https://open.dingtalk.com/" target="_blank" rel="noreferrer">钉钉开放平台</a> 创建企业内部应用，并添加 <b>机器人能力</b></>,
      <>进入 <b>应用配置</b>，左侧菜单 <b>机器人 → 机器人配置</b>，消息接收模式必须选择 <b>Stream</b> 模式，不要选 Webhook</>,
      <>在 <b>凭证与基础信息</b> 页面复制 <b>Client ID</b> 和 <b>Client Secret</b></>,
      <>先在钉钉侧 <b>发布应用版本</b>，确认应用可见范围包含你的账号</>,
      <>在钉钉群中添加该机器人并 <b>@机器人</b>，或私聊机器人完成配对</>,
    ];
  case "wechat":
    return [
      <>登录 <a href="https://developer.work.weixin.qq.com/" target="_blank" rel="noreferrer">企业微信开发者后台</a>，创建或选择 <b>自建应用</b></>,
      <>在应用详情中复制 <b>企业 ID</b>、<b>Agent ID</b> 和 <b>Secret</b></>,
      <>如需接收成员消息，打开 <b>接收消息</b>，配置回调地址为当前服务的 <b>/nexus/v1/channels/wechat/messages</b></>,
      <>复制回调配置中的 <b>Token</b> 与 <b>EncodingAESKey</b>，填入下方表单</>,
      <>确认应用可见范围包含目标成员；个人微信不支持官方机器人 IM 接入</>,
    ];
  case "weixin-personal":
    return [
      <>选择处理智能体后点击 <b>保存并扫码登录</b>，Nexus 会直接请求腾讯 iLink Bot API 生成个人微信登录二维码</>,
      <>用手机微信扫码并确认；如手机端显示数字验证码，在下方扫码面板输入后继续等待登录完成</>,
      <>登录成功后 Nexus 会保存 <b>ilink_bot_token</b>，并内置长轮询 <b>getupdates</b> 接收个人微信私聊消息</>,
      <>智能体回复时，Nexus 会使用同一 iLink 账号调用 <b>sendmessage</b> 回投文本消息</>,
      <>Nexus 首次收到发送者消息后，在配对授权页批准，再由选定智能体处理</>,
    ];
  case "feishu":
    return [
      <>登录 <a href="https://open.feishu.cn/" target="_blank" rel="noreferrer">飞书开放平台</a> 创建企业自建应用，在 <b>应用能力</b> 中添加机器人能力</>,
      <>在 <b>凭证与基础信息</b> 页面获取 <b>App ID</b> 和 <b>App Secret</b></>,
      <>进入 <b>权限管理</b>，为机器人添加收发消息、读取消息、消息表情回复所需的 IM 权限，并提交发布</>,
      <>在 <b>事件订阅</b> 中选择 <b>使用长连接接收事件</b>，订阅 <b>im.message.receive_v1</b> 和 <b>im.message.reaction.created_v1</b>；Nexus 启动后会用 App ID/App Secret 主动连接飞书</>,
      <>如开启事件加密或需要校验 Token，把飞书侧的 <b>Encrypt Key</b> / <b>Verification Token</b> 填到下方；首条用户消息进入后才会生成配对请求</>,
      <>确认应用可用范围包含目标用户或群，并在飞书群中添加该机器人</>,
    ];
  case "telegram":
    return [
      <>在 Telegram 中搜索 <a href="https://t.me/BotFather" target="_blank" rel="noreferrer">@BotFather</a>，发送 <b>/newbot</b> 创建机器人</>,
      <>按提示设置机器人名称和用户名，成功后 BotFather 会返回 <b>Bot Token</b></>,
      <>将 <b>Bot Token</b> 填入下方表单，完成连接</>,
      <>在 Telegram 群中添加该机器人并 <b>@机器人</b>，或私聊机器人完成配对</>,
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

function is_channel_login_running(view: ChannelLoginView | null) {
  return view?.status === "running";
}

function channel_login_status_label(status: string) {
  switch (status) {
  case "running":
    return "等待扫码";
  case "verify_code_required":
    return "需要验证码";
  case "succeeded":
    return "登录完成";
  case "expired":
    return "已超时";
  case "cancelled":
    return "已取消";
  case "error":
    return "登录失败";
  default:
    return status || "未启动";
  }
}

function channel_login_status_tone(status: string): "default" | "success" | "warning" | "danger" | "info" {
  switch (status) {
  case "succeeded":
    return "success";
  case "running":
    return "info";
  case "verify_code_required":
    return "warning";
  case "expired":
  case "cancelled":
    return "warning";
  case "error":
    return "danger";
  default:
    return "default";
  }
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
          本通道默认使用飞书长连接事件订阅；请确认应用已选择长连接并订阅“接收消息”事件。
        </div>
      ) : null}
      {is_personal_weixin_channel(item.channel_type) ? (
        <div className="mt-4 border-t border-(--divider-subtle-color) pt-3 text-[12px] font-medium leading-5 text-(--text-muted)">
          个人微信与企业微信分开配置；本通道由 Nexus 内置 iLink 连接能力提供，不复用企业微信回调。
        </div>
      ) : null}
    </div>
  );
}

function LoginQRCode({ payload }: { payload?: string }) {
  const [image_url, set_image_url] = useState("");

  useEffect(() => {
    const value = payload?.trim() || "";
    if (!value) {
      set_image_url("");
      return;
    }
    if (value.startsWith("data:image/")) {
      set_image_url(value);
      return;
    }
    let cancelled = false;
    void import("qrcode")
      .then((module) => module.toDataURL(value, {
        errorCorrectionLevel: "M",
        margin: 1,
        scale: 7,
        width: 220,
      }))
      .then((url) => {
        if (!cancelled) {
          set_image_url(url);
        }
      })
      .catch(() => {
        if (!cancelled) {
          set_image_url("");
        }
      });
    return () => {
      cancelled = true;
    };
  }, [payload]);

  if (!payload) {
    return null;
  }

  return (
    <div className="flex flex-col items-center gap-2 rounded-[12px] border border-(--divider-subtle-color) px-4 py-4">
      {image_url ? (
        <img
          alt="个人微信扫码登录二维码"
          className="h-[220px] w-[220px] rounded-[8px] bg-white p-2"
          src={image_url}
        />
      ) : (
        <div className="flex h-[220px] w-[220px] items-center justify-center rounded-[8px] bg-white p-4 text-center text-[12px] leading-5 text-neutral-700">
          二维码生成失败，请使用下方链接
        </div>
      )}
      <code className="max-w-full truncate rounded-[8px] border border-(--divider-subtle-color) px-2 py-1 text-[11px] text-(--text-muted)">
        {payload}
      </code>
    </div>
  );
}

function ChannelLoginPanel({
  disabled,
  loading,
  login_view,
  on_start,
  on_submit_verify_code,
}: {
  disabled: boolean;
  loading: boolean;
  login_view: ChannelLoginView | null;
  on_start: () => void;
  on_submit_verify_code: (value: string) => void;
}) {
  const [verify_code, set_verify_code] = useState("");
  const running = is_channel_login_running(login_view);
  const output = login_view?.output?.trimEnd() || (running ? "等待 iLink 扫码状态..." : "");
  const status = login_view?.status || "";
  const status_tone = channel_login_status_tone(status);
  const StatusIcon = status === "succeeded" ? CircleCheck : status === "error" || status === "expired" ? TriangleAlert : Terminal;
  const verify_required = status === "verify_code_required";

  return (
    <div className="rounded-[14px] border border-(--divider-subtle-color) bg-transparent px-4 py-3">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2 text-[13px] font-semibold text-(--text-strong)">
            <QrCode className="h-4 w-4 text-(--primary)" />
            扫码登录
          </div>
          <p className="mt-1 text-[12px] leading-5 text-(--text-muted)">
            Nexus 会直接请求腾讯 iLink Bot API 生成二维码并保存登录凭据。
          </p>
        </div>
        <UiButton
          disabled={disabled || loading || running}
          onClick={on_start}
          size="sm"
          tone="primary"
          type="button"
          variant="solid"
        >
          {loading || running ? <Loader2 className="h-4 w-4 animate-spin" /> : <QrCode className="h-4 w-4" />}
          {login_view ? "重新拉起" : "拉起二维码"}
        </UiButton>
      </div>

      {login_view ? (
        <div className="mt-3 space-y-3">
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <UiBadge size="xs" tone={status_tone}>
              <StatusIcon className="mr-1 h-3 w-3" />
              {channel_login_status_label(status)}
            </UiBadge>
            <code className="min-w-0 truncate rounded-[8px] border border-(--divider-subtle-color) px-2 py-1 text-[11px] text-(--text-muted)">
              {login_view.account_id || login_view.command || "Nexus iLink QR login"}
            </code>
          </div>
          <LoginQRCode payload={login_view.qr_payload} />
          {verify_required ? (
            <div className="rounded-[10px] border border-[color:color-mix(in_srgb,var(--warning)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_8%,transparent)] px-3 py-3">
              <div className="mb-2 text-[12px] font-semibold text-(--text-strong)">
                {login_view.verify_code_hint || "输入手机微信显示的数字"}
              </div>
              <div className="flex gap-2">
                <UiInput
                  onChange={(event) => set_verify_code(event.target.value)}
                  placeholder="验证码"
                  value={verify_code}
                  variant="dialog"
                />
                <UiButton
                  disabled={!verify_code.trim() || loading}
                  onClick={() => {
                    on_submit_verify_code(verify_code);
                    set_verify_code("");
                  }}
                  size="sm"
                  tone="primary"
                  type="button"
                  variant="solid"
                >
                  提交
                </UiButton>
              </div>
            </div>
          ) : null}
          <pre className="max-h-[280px] min-h-[132px] overflow-auto whitespace-pre-wrap break-words rounded-[10px] bg-[#101418] px-3 py-3 font-mono text-[12px] leading-5 text-[#d7f8de]">{output}</pre>
          {login_view.error ? (
            <div className="rounded-[10px] border border-[color:color-mix(in_srgb,var(--destructive)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] px-3 py-2 text-[12px] leading-5 text-(--destructive)">
              {login_view.error}
            </div>
          ) : null}
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
  const [login_loading, set_login_loading] = useState(false);
  const [login_view, set_login_view] = useState<ChannelLoginView | null>(null);
  const is_planned = is_channel_planned(current_item);
  const discord_oauth_url = current_item.channel_type === "discord" ? build_discord_oauth_url(config) : "";
  const supports_personal_weixin_login = is_personal_weixin_channel(current_item.channel_type);
  const login_running = is_channel_login_running(login_view);
  const login_id = login_view?.login_id || "";
  const login_status = login_view?.status || "";

  useEffect(() => {
    set_current_item(item);
    set_agent_id(item.agent_id || agents[0]?.agent_id || "");
    set_config(item.public_config || {});
    set_credentials({});
    set_login_view(null);
    set_login_loading(false);
  }, [agents, item]);

  const handle_field_change = (field: ChannelCredentialField, value: string) => {
    if (field.secret) {
      set_credentials((current) => ({ ...current, [field.key]: value }));
      return;
    }
    set_config((current) => ({ ...current, [field.key]: value }));
  };

  const start_login = useCallback(async () => {
    if (!supports_personal_weixin_login || login_running) return;
    set_login_loading(true);
    try {
      const next_login = await start_channel_login_api(current_item.channel_type);
      set_login_view(next_login);
    } catch (error) {
      on_error(error instanceof Error ? error.message : "扫码登录启动失败");
    } finally {
      set_login_loading(false);
    }
  }, [current_item.channel_type, login_running, on_error, supports_personal_weixin_login]);

  const submit_verify_code = useCallback(async (value: string) => {
    if (!supports_personal_weixin_login || !login_id) return;
    set_login_loading(true);
    try {
      const next_login = await submit_channel_login_verify_code_api(current_item.channel_type, login_id, value);
      set_login_view(next_login);
    } catch (error) {
      on_error(error instanceof Error ? error.message : "验证码提交失败");
    } finally {
      set_login_loading(false);
    }
  }, [current_item.channel_type, login_id, on_error, supports_personal_weixin_login]);

  useEffect(() => {
    if (!supports_personal_weixin_login || !login_id || login_status !== "running") {
      return;
    }
    const timer = window.setInterval(async () => {
      try {
        const next_login = await get_channel_login_api(current_item.channel_type, login_id);
        set_login_view(next_login);
      } catch (error) {
        window.clearInterval(timer);
        on_error(error instanceof Error ? error.message : "扫码登录状态刷新失败");
      }
    }, 1500);
    return () => window.clearInterval(timer);
  }, [current_item.channel_type, login_id, login_status, on_error, supports_personal_weixin_login]);

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
      const should_start_login = is_personal_weixin_channel(saved.channel_type);
      on_saved(saved, !should_start_login);
      if (close_on_success && should_start_login) {
        set_login_loading(true);
        const next_login = await start_channel_login_api(saved.channel_type);
        set_login_view(next_login);
        return;
      }
      if (close_on_success) on_close();
    } catch (error) {
      on_error(error instanceof Error ? error.message : "连接失败");
    } finally {
      set_saving(false);
      set_login_loading(false);
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
                description="频道接入将在后续版本补充，当前版本暂不支持配置机器人或配对。"
                size="sm"
                title="该频道未上线"
                variant="inset"
              />
            ) : (
              <>
                <ChannelGuide item={current_item} />

                {current_item.runtime_note ? (
                  <div className="rounded-[14px] border border-(--divider-subtle-color) bg-transparent px-4 py-3 text-[13px] font-medium leading-5 text-(--text-default)">
                    {current_item.runtime_note}
                  </div>
                ) : null}

                {supports_personal_weixin_login ? (
                  <ChannelLoginPanel
                    disabled={!current_item.configured || saving}
                    loading={login_loading}
                    login_view={login_view}
                    on_start={start_login}
                    on_submit_verify_code={submit_verify_code}
                  />
                ) : null}

                <UiField label={<>处理智能体 <span className="text-(--destructive)">*</span></>}>
                  <UiSelectMenu
                    aria_label="选择频道处理智能体"
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
              disabled={saving || login_loading || !agent_id || is_planned}
              size="lg"
              tone="primary"
              type="submit"
              variant="solid"
            >
              {supports_personal_weixin_login ? <QrCode className="h-5 w-5" /> : <Power className="h-5 w-5" />}
              {is_planned
                ? "未上线"
                : saving
                  ? "保存中..."
                  : login_loading
                    ? "拉起二维码..."
                    : supports_personal_weixin_login
                      ? "保存并扫码登录"
                      : "连接"}
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
  const closed = is_channel_closed(item);
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
  const description = closed
    ? "该频道当前未开放配置，目前仅保留入口和信息结构。"
    : planned
      ? "该频道将在后续版本补充，目前仅保留入口和信息结构。"
      : item.runtime_status === "external_adapter" && !item.configured
        ? "选择处理智能体后，按通道说明完成外部连接。"
        : item.configured
          ? `由 ${item.agent_name || "已配置智能体"} 处理该渠道消息。`
          : "选择一个智能体并填写机器人凭证后，即可开始处理来自该渠道的消息。";
  const meta_items = [
    item.bot_label,
    `用户 ${item.stats.paired_user_count}`,
    `群聊 ${item.supports_group ? item.stats.paired_group_count : "-"}`,
    `待处理 ${item.stats.pending_count}`,
    item.configured ? "已绑定智能体" : "待配置",
    item.supports_group ? null : "仅私聊",
  ].filter(Boolean);

  return (
    <UiListRow
      class_name={cn(
        "min-h-[72px] rounded-[14px] px-2 py-1.5",
        planned && "cursor-default opacity-70",
      )}
      leading={<ChannelIcon type={item.channel_type} />}
      on_click={planned ? undefined : () => on_configure(item)}
      right={(
        <div className="flex shrink-0 items-center gap-1.5">
          <ChannelStatePill tone={state_tone}>
            {channel_status_text(item)}
          </ChannelStatePill>
          {!planned && item.docs_url ? (
            <UiListActionButton
              onClick={() => window.open(item.docs_url, "_blank", "noopener,noreferrer")}
              size="sm"
              stop_propagation
              title="查看接入文档"
            >
              <ExternalLink className="h-3 w-3" />
            </UiListActionButton>
          ) : null}
          {!planned ? (
            <UiListActionButton
              class_name="text-(--primary)"
              onClick={() => on_configure(item)}
              size="sm"
              stop_propagation
              title="设置机器人"
              visibility="visible"
            >
              <Settings2 className="h-3 w-3" />
            </UiListActionButton>
          ) : (
            <span className="flex h-8 w-8 items-center justify-center text-(--icon-muted)">
              <Clock3 className="h-3.5 w-3.5" />
            </span>
          )}
        </div>
      )}
    >
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2">
          <span className="truncate text-[15px] font-semibold tracking-[-0.02em] text-(--text-strong)">
            {item.title}
          </span>
          {item.runtime_status === "external_adapter" ? (
            <UiBadge size="xs" tone="warning">外部适配器</UiBadge>
          ) : null}
        </div>
        <div className="mt-0.5 truncate text-[13px] leading-5 text-(--text-muted)">
          {description}
        </div>
        <div className="mt-0.5 flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-0.5 text-[11px] leading-4 text-(--text-soft)">
          {meta_items.map((meta_item, index) => (
            <span className="min-w-0 truncate" key={`${item.channel_type}-${index}`}>
              {index > 0 ? "· " : ""}
              {meta_item}
            </span>
          ))}
        </div>
        {item.runtime_note ? (
          <div className="mt-0.5 truncate text-[11px] leading-4 text-(--text-soft)">
            {item.runtime_note}
          </div>
        ) : null}
      </div>
    </UiListRow>
  );
}

function ChannelLoadingGrid() {
  return (
    <div className="flex min-h-40 items-center justify-center text-sm text-(--text-muted)">
      <Loader2 className="h-5 w-5 animate-spin" />
    </div>
  );
}

export function ChannelsDirectory() {
  const { t } = useI18n();
  const [channels, set_channels] = useState<ChannelConfigView[]>([]);
  const [agents, set_agents] = useState<Agent[]>([]);
  const [selected, set_selected] = useState<ChannelConfigView | null>(null);
  const [search_query, set_search_query] = useState("");
  const [channel_filter, set_channel_filter] = useState<ChannelFilter>("all");
  const [loading, set_loading] = useState(true);
  const [feedback, set_feedback] = useState<{ tone: "success" | "error"; title: string; message: string } | null>(null);

  const sorted_channels = useMemo(() => {
    return [...channels].sort((left, right) => {
      const left_index = CHANNEL_ORDER.indexOf(left.channel_type);
      const right_index = CHANNEL_ORDER.indexOf(right.channel_type);
      return (left_index < 0 ? CHANNEL_ORDER.length : left_index) - (right_index < 0 ? CHANNEL_ORDER.length : right_index);
    });
  }, [channels]);
  const visible_channels = useMemo(() => {
    const query = search_query.trim().toLowerCase();
    return sorted_channels.filter((item) => {
      const matches_query = !query
        || item.title.toLowerCase().includes(query)
        || item.bot_label.toLowerCase().includes(query)
        || item.channel_type.toLowerCase().includes(query)
        || (item.agent_name ?? "").toLowerCase().includes(query);
      if (!matches_query) {
        return false;
      }
      if (channel_filter === "connected") {
        return item.connection_state === "connected";
      }
      if (channel_filter === "configured") {
        return item.configured && !is_channel_planned(item);
      }
      if (channel_filter === "unconfigured") {
        return !item.configured && !is_channel_planned(item);
      }
      if (channel_filter === "planned") {
        return is_channel_planned(item);
      }
      return true;
    });
  }, [channel_filter, search_query, sorted_channels]);

  const refresh = async () => {
    set_loading(true);
    try {
      const [next_channels, next_agents] = await Promise.all([list_channels_api(), get_agents()]);
      set_channels(next_channels);
      set_agents(next_agents);
    } catch (error) {
      set_feedback({ tone: "error", title: "加载失败", message: error instanceof Error ? error.message : "频道加载失败" });
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
            badge={t("capability.channels_badge", { count: channels.length || 6 })}
            density="compact"
            leading={<MessageCircle className="h-4 w-4" />}
            subtitle={t("capability.channels_subtitle")}
            title={t("capability.channels")}
            trailing={(
              <WorkspaceSurfaceToolbarAction onClick={() => void refresh()}>
                <RefreshCw className="h-3.5 w-3.5" />
                {t("capability.refresh")}
              </WorkspaceSurfaceToolbarAction>
            )}
          />
        )}
        stable_gutter
      >
        <CapabilityPageLayout
          description={t("capability.channels_intro_description")}
          title={t("capability.channels_intro_title")}
        >
          <CapabilityFilterBar>
            <CapabilityFilterSearchInput
              on_change={set_search_query}
              placeholder={t("capability.channels_search_placeholder")}
              value={search_query}
            />
            <CapabilityFilterSelect
              aria_label={t("capability.channels_filter_aria")}
              label={t("capability.category_label")}
              leading={<SlidersHorizontal className="h-3.5 w-3.5" />}
              on_change={(value) => set_channel_filter(value as ChannelFilter)}
              options={CHANNEL_FILTER_OPTIONS.map((option) => ({
                value: option.value,
                label: t(option.label_key),
              }))}
              value={channel_filter}
            />
          </CapabilityFilterBar>

          {loading ? (
            <ChannelLoadingGrid />
          ) : visible_channels.length === 0 ? (
            <UiStateBlock
              description={t("capability.channels_empty_description")}
              icon={<MessageCircle className="h-6 w-6 text-(--icon-default)" />}
              size="md"
              title={t("capability.channels_empty_title")}
            />
          ) : (
            <section>
              <CapabilitySectionHeader
                count={t("capability.result_count", { count: visible_channels.length })}
                title={t("capability.channels_section_title")}
              />
              <div className="grid grid-cols-1 gap-x-12 gap-y-4 md:grid-cols-2">
                {visible_channels.map((item) => (
                  <ChannelCard item={item} key={item.channel_type} on_configure={set_selected} />
                ))}
              </div>
            </section>
          )}
        </CapabilityPageLayout>
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
