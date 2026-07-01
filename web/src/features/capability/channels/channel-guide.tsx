import type { ReactNode } from "react";

import { ChannelConfigView, ImChannelType } from "@/lib/api/channel-api";
import {
  get_dialog_note_class_name,
  get_dialog_note_style,
} from "@/shared/ui/dialog/dialog-styles";
import { is_personal_weixin_channel } from "./channel-model";

interface ChannelGuideStep {
  content: ReactNode;
  id: string;
}

function guide_steps(channel_type: ImChannelType): ChannelGuideStep[] {
  switch (channel_type) {
  case "dingtalk":
    return [
      { id: "dingtalk-create-app", content: <>前往 <a href="https://open.dingtalk.com/" target="_blank" rel="noreferrer">钉钉开放平台</a> 创建企业内部应用，并添加 <b>机器人能力</b></> },
      { id: "dingtalk-stream-mode", content: <>进入 <b>应用配置</b>，左侧菜单 <b>机器人 → 机器人配置</b>，消息接收模式必须选择 <b>Stream</b> 模式，不要选 Webhook</> },
      { id: "dingtalk-copy-credentials", content: <>在 <b>凭证与基础信息</b> 页面复制 <b>Client ID</b> 和 <b>Client Secret</b></> },
      { id: "dingtalk-session-webhook", content: <>常规收消息后原路回复会使用钉钉 Stream 的 <b>sessionWebhook</b>；只有需要主动群发到指定 openConversationId 时才填写 <b>Robot Code</b></> },
      { id: "dingtalk-publish-version", content: <>先在钉钉侧 <b>发布应用版本</b>，确认应用可见范围包含你的账号</> },
      { id: "dingtalk-add-bot", content: <>在钉钉群中添加该机器人并 <b>@机器人</b>，或私聊机器人完成配对</> },
    ];
  case "wechat":
    return [
      { id: "wechat-create-bot", content: <>登录 <a href="https://developer.work.weixin.qq.com/" target="_blank" rel="noreferrer">企业微信开发者后台</a>，创建或选择 <b>智能机器人</b></> },
      { id: "wechat-copy-credentials", content: <>在机器人配置页复制 <b>Bot ID</b> 和 <b>Secret</b>，填入下方表单</> },
      { id: "wechat-long-connection", content: <>Nexus 会通过企业微信官方长连接接收入站消息，并用同一长连接 <b>stream</b> 回复</> },
      { id: "wechat-stream-reply", content: <>智能体回复会使用企业微信智能机器人的 <b>stream</b> 回复回到原会话</> },
      { id: "wechat-approve-pairing", content: <>确认机器人可见范围包含目标成员或群；首次收到外部消息后在配对控制台批准</> },
    ];
  case "weixin-personal":
    return [
      { id: "weixin-personal-login", content: <>选择处理智能体后点击 <b>保存并扫码登录</b>，Nexus 会直接请求腾讯 iLink Bot API 生成微信登录二维码</> },
      { id: "weixin-personal-scan", content: <>用手机微信扫码并确认；如手机端显示数字验证码，在下方扫码面板输入后继续等待登录完成</> },
      { id: "weixin-personal-token", content: <>登录成功后 Nexus 会保存 <b>ilink_bot_token</b>，并内置长轮询 <b>getupdates</b> 接收微信私聊消息</> },
      { id: "weixin-personal-send", content: <>智能体回复时，Nexus 会使用同一 iLink 账号调用 <b>sendmessage</b> 回投文本消息</> },
      { id: "weixin-personal-approve", content: <>Nexus 首次收到发送者消息后，在配对控制台批准，再由选定智能体处理</> },
    ];
  case "feishu":
    return [
      { id: "feishu-create-app", content: <>登录 <a href="https://open.feishu.cn/" target="_blank" rel="noreferrer">飞书开放平台</a> 创建企业自建应用，在 <b>应用能力</b> 中添加机器人能力</> },
      { id: "feishu-copy-credentials", content: <>在 <b>凭证与基础信息</b> 页面获取 <b>App ID</b> 和 <b>App Secret</b></> },
      { id: "feishu-permissions", content: <>进入 <b>权限管理</b>，为机器人添加收发消息、读取消息、消息表情回复所需的 IM 权限，并提交发布</> },
      { id: "feishu-events", content: <>在 <b>事件订阅</b> 中选择 <b>使用长连接接收事件</b>，订阅 <b>im.message.receive_v1</b> 和 <b>im.message.reaction.created_v1</b>；Nexus 启动后会用 App ID/App Secret 主动连接飞书</> },
      { id: "feishu-verification", content: <>如开启事件加密或需要校验 Token，把飞书侧的 <b>Encrypt Key</b> / <b>Verification Token</b> 填到下方；首条用户消息进入后才会生成配对请求</> },
      { id: "feishu-add-bot", content: <>确认应用可用范围包含目标用户或群，并在飞书群中添加该机器人</> },
    ];
  case "telegram":
    return [
      { id: "telegram-create-bot", content: <>在 Telegram 中搜索 <a href="https://t.me/BotFather" target="_blank" rel="noreferrer">@BotFather</a>，发送 <b>/newbot</b> 创建机器人</> },
      { id: "telegram-copy-token", content: <>按提示设置机器人名称和用户名，成功后 BotFather 会返回 <b>Bot Token</b></> },
      { id: "telegram-save-token", content: <>将 <b>Bot Token</b> 填入下方表单，完成连接</> },
      { id: "telegram-add-bot", content: <>在 Telegram 群中添加该机器人并 <b>@机器人</b>，或私聊机器人完成配对</> },
    ];
  case "discord":
    return [
      { id: "discord-create-app", content: <>打开 <a href="https://discord.com/developers/applications" target="_blank" rel="noreferrer">Discord 开发者平台</a>，点击 <b>New Application</b> 创建应用</> },
      { id: "discord-copy-token", content: <>复制 <b>Application ID</b>；进入左侧 <b>Bot</b> 页面，点击 <b>Reset Token</b> 获取 <b>Bot Token</b>，不是 OAuth Client Secret</> },
      { id: "discord-message-content", content: <>开启 <b>Message Content Intent</b>，否则 Gateway 消息事件可能没有正文内容</> },
      { id: "discord-authorize", content: <>在下方填写 Application ID 和 Bot Token，生成 <b>授权链接</b>，打开链接并添加到 <b>服务器</b></> },
    ];
  }
}

export function ChannelGuide({
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
        {steps.map((step) => (
          <li key={step.id} className="[&_a]:font-semibold [&_a]:text-(--primary) [&_b]:font-semibold">
            {step.content}
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
          微信与企业微信分开配置；本通道由 Nexus 内置 iLink 连接能力提供，不复用企业微信回调。
        </div>
      ) : null}
    </div>
  );
}
