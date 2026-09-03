// INPUT: Connector 图标键、标题与标准能力图标尺寸。
// OUTPUT: 静态品牌轮廓或稳定种子回退，并统一投影为 Capability 公共身份图标。
// POS: Connector 图标资源映射；品牌容器视觉由 CapabilityBrandIcon 持有。
"use client";

import {
  CapabilityBrandIcon,
  type CapabilityBrandIconSize,
} from "@/features/capability/shared/capability-brand-icon";
import { cn } from "@/shared/ui/class-name";
import { UiSeededAvatar } from "@/shared/ui/display/seeded-avatar";

import { getConnectorLetter } from "./connector-icons";

type ConnectorIconSize = CapabilityBrandIconSize;

interface ConnectorIconProps {
  icon: string;
  title: string;
  size?: ConnectorIconSize;
  className?: string;
}

const ICON_SIZE_CLASS: Record<ConnectorIconSize, string> = {
  sm: "h-5 w-5 radius-control-xs",
  md: "h-9 w-9 radius-control-sm",
  lg: "h-14 w-14 surface-radius-md",
};

const CONNECTOR_ICON_SRC: Record<string, string> = {
  airtable: "/icon/connector/airtable.svg",
  ahrefs: "/icon/connector/ahrefs.svg",
  alibaba: "/icon/connector/alibabadotcom.svg",
  amap: "/icon/connector/amap.svg",
  atlassian: "/icon/connector/atlassian.svg",
  didi: "/icon/connector/didi.svg",
  dingtalk: "/icon/connector/dingtalk.svg",
  dropbox: "/icon/connector/dropbox.svg",
  "feishu-docx": "/icon/connector/feishu.svg",
  github: "/icon/connector/github.svg",
  gmail: "/icon/connector/gmail.svg",
  "google-calendar": "/icon/connector/googlecalendar.svg",
  "google-drive": "/icon/connector/googledrive.svg",
  instagram: "/icon/connector/instagram.svg",
  linear: "/icon/connector/linear.svg",
  linkedin: "/icon/connector/linkedin.svg",
  make: "/icon/connector/make.svg",
  meta: "/icon/connector/meta.svg",
  monday: "/icon/connector/monday.svg",
  notion: "/icon/connector/notion.svg",
  odoo: "/icon/connector/odoo.svg",
  outlook: "/icon/connector/outlook.svg",
  reddit: "/icon/connector/reddit.svg",
  richmail: "/icon/connector/richmail.svg",
  shopify: "/icon/connector/shopify.svg",
  similarweb: "/icon/connector/similarweb.svg",
  slack: "/icon/connector/slack.svg",
  square: "/icon/connector/square.svg",
  "tencent-docs": "/icon/connector/tencent.svg",
  tiktok: "/icon/connector/tiktok.svg",
  "x-twitter": "/icon/connector/x.svg",
  youtube: "/icon/connector/youtube.svg",
  yuque: "/icon/connector/yuque.svg",
  zapier: "/icon/connector/zapier.svg",
};

function getStaticConnectorIconSrc(icon: string): string {
  return CONNECTOR_ICON_SRC[icon] ?? "";
}

export function ConnectorIcon({
  icon,
  title,
  size = "md",
  className,
}: ConnectorIconProps) {
  if (icon === "custom-mcp") {
    return (
      <UiSeededAvatar
        className={cn(ICON_SIZE_CLASS[size], className)}
        seed={title}
        size={size === "sm" ? "2xs" : size === "md" ? "sm" : "lg"}
      />
    );
  }
  const staticIconSrc = getStaticConnectorIconSrc(icon);
  const letter = getConnectorLetter(icon, title);

  return (
    <CapabilityBrandIcon
      className={className}
      fallback={letter}
      size={size}
      src={staticIconSrc}
      title={title}
    />
  );
}
