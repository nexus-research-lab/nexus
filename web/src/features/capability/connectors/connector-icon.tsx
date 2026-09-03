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

const CONNECTOR_BRAND_ASSETS: Record<string, string> = {
  amap: "/icon/connector/amap.svg",
  didi: "/icon/connector/didi.svg",
  dingtalk: "/icon/connector/dingtalk.svg",
  "feishu-docx": "/icon/connector/feishu.svg",
  github: "/icon/connector/github.svg",
  richmail: "/icon/connector/richmail.svg",
  "tencent-docs": "/icon/connector/tencent.svg",
  yuque: "/icon/connector/yuque.svg",
};

function getConnectorBrandAsset(icon: string): string {
  return CONNECTOR_BRAND_ASSETS[icon];
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
  const brandAsset = getConnectorBrandAsset(icon);
  const letter = getConnectorLetter(icon, title);

  return (
    <CapabilityBrandIcon
      className={className}
      fallback={letter}
      size={size}
      src={brandAsset}
      title={title}
    />
  );
}
