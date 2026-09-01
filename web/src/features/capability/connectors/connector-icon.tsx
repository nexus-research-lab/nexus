"use client";

import { cn } from "@/shared/ui/class-name";
import { UiSeededAvatar } from "@/shared/ui/display/seeded-avatar";

import { getConnectorLetter } from "./connector-icons";

type ConnectorIconSize = "sm" | "md" | "lg";

interface ConnectorIconProps {
  icon: string;
  title: string;
  size?: ConnectorIconSize;
  className?: string;
}

const ICON_SIZE_CLASS: Record<ConnectorIconSize, string> = {
  sm: "h-5 w-5 rounded-[5px] text-2xs",
  md: "h-9 w-9 rounded-[8px] text-compact",
  lg: "h-14 w-14 surface-radius-md text-md",
};

const ICON_MASK_SIZE_CLASS: Record<ConnectorIconSize, string> = {
  sm: "h-3.5 w-3.5",
  md: "h-6 w-6",
  lg: "h-9 w-9",
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
    <span
      aria-label={title}
      className={cn(
        "flex shrink-0 items-center justify-center overflow-hidden border border-(--divider-subtle-color) bg-(--surface-panel-background) font-semibold text-(--text-strong)",
        ICON_SIZE_CLASS[size],
        className,
      )}
    >
      {staticIconSrc ? (
        <span
          aria-hidden="true"
          className={ICON_MASK_SIZE_CLASS[size]}
          style={{
            backgroundColor: "var(--text-strong)",
            maskImage: `url(${staticIconSrc})`,
            maskPosition: "center",
            maskRepeat: "no-repeat",
            maskSize: "contain",
            WebkitMaskImage: `url(${staticIconSrc})`,
            WebkitMaskPosition: "center",
            WebkitMaskRepeat: "no-repeat",
            WebkitMaskSize: "contain",
          }}
        />
      ) : (
        <span aria-hidden="true" className="leading-none tracking-normal">
          {letter}
        </span>
      )}
    </span>
  );
}
