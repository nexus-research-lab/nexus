"use client";

import {
  Github,
  Instagram,
  Linkedin,
  Slack,
  Twitter,
  Youtube,
  type LucideIcon,
} from "lucide-react";
import type { ReactNode } from "react";

import { cn } from "@/lib/utils";

import { get_connector_colors, get_connector_letter } from "./connector-icons";

type ConnectorIconSize = "md" | "lg";

interface ConnectorIconProps {
  icon: string;
  title: string;
  size?: ConnectorIconSize;
  class_name?: string;
}

interface BrandLogo {
  tone: string;
  node: ReactNode;
}

const ICON_SIZE_CLASS: Record<ConnectorIconSize, string> = {
  md: "h-11 w-11 rounded-[12px] text-[13px]",
  lg: "h-16 w-16 rounded-[18px] text-[19px]",
};

const LOGO_SCALE_CLASS: Record<ConnectorIconSize, string> = {
  md: "h-7 w-7",
  lg: "h-10 w-10",
};

const LUCIDE_LOGO_SCALE_CLASS: Record<ConnectorIconSize, string> = {
  md: "h-6 w-6",
  lg: "h-9 w-9",
};

function LucideLogo({ icon: Icon, class_name }: { icon: LucideIcon; class_name?: string }) {
  return <Icon aria-hidden="true" className={cn("stroke-[2]", class_name)} />;
}

function TextLogo({ children, class_name }: { children: ReactNode; class_name?: string }) {
  return (
    <span aria-hidden="true" className={cn("font-bold leading-none tracking-normal", class_name)}>
      {children}
    </span>
  );
}

function GmailLogo({ class_name }: { class_name?: string }) {
  return (
    <svg aria-hidden="true" className={class_name} viewBox="0 0 48 48">
      <path d="M8 14.5v21h7.5V24.8L8 19.2z" fill="#4285F4" />
      <path d="M32.5 35.5H40v-21l-7.5 4.7z" fill="#34A853" />
      <path d="M8 14.5l16 12 16-12v8.8l-16 12-16-12z" fill="#EA4335" />
      <path d="M8 14.5l16 12 16-12-4-5L24 18.7 12 9.5z" fill="#FBBC04" />
      <path d="M8 14.5l16 12 16-12-4-5L24 18.7 12 9.5z" fill="#EA4335" opacity="0.24" />
    </svg>
  );
}

function GoogleCalendarLogo({ class_name }: { class_name?: string }) {
  return (
    <svg aria-hidden="true" className={class_name} viewBox="0 0 48 48">
      <rect fill="#fff" height="34" rx="6" width="34" x="7" y="7" />
      <path d="M13 7h22a6 6 0 0 1 6 6v7H7v-7a6 6 0 0 1 6-6z" fill="#4285F4" />
      <path d="M7 20h34v15a6 6 0 0 1-6 6H13a6 6 0 0 1-6-6z" fill="#fff" />
      <path d="M13 7h6v34h-6a6 6 0 0 1-6-6V13a6 6 0 0 1 6-6z" fill="#34A853" opacity="0.92" />
      <path d="M29 7h6a6 6 0 0 1 6 6v22a6 6 0 0 1-6 6h-6z" fill="#FBBC04" opacity="0.92" />
      <path d="M13 35h22a6 6 0 0 0 6-6v6a6 6 0 0 1-6 6H13a6 6 0 0 1-6-6v-6a6 6 0 0 0 6 6z" fill="#EA4335" opacity="0.88" />
      <text fill="#1f2937" fontFamily="Arial, sans-serif" fontSize="13" fontWeight="700" x="18" y="31">
        31
      </text>
    </svg>
  );
}

function GoogleDriveLogo({ class_name }: { class_name?: string }) {
  return (
    <svg aria-hidden="true" className={class_name} viewBox="0 0 48 48">
      <path d="M18 7h12l14 24H32z" fill="#34A853" />
      <path d="M18 7 4 31l6 10L24 17z" fill="#FBBC04" />
      <path d="M10 41h28l6-10H16z" fill="#4285F4" />
      <path d="M24 17 16 31h16z" fill="#188038" opacity="0.26" />
    </svg>
  );
}

function FeishuLogo({ class_name }: { class_name?: string }) {
  return (
    <svg aria-hidden="true" className={class_name} viewBox="0 0 48 48">
      <rect fill="#3370FF" height="15" rx="4.5" width="15" x="7" y="7" />
      <rect fill="#00C853" height="15" rx="4.5" width="15" x="26" y="7" />
      <rect fill="#FFB300" height="15" rx="4.5" width="15" x="7" y="26" />
      <rect fill="#FF5A5F" height="15" rx="4.5" width="15" x="26" y="26" />
      <circle cx="24" cy="24" fill="#fff" r="4.5" />
    </svg>
  );
}

function DropboxLogo({ class_name }: { class_name?: string }) {
  return (
    <svg aria-hidden="true" className={class_name} viewBox="0 0 48 48">
      <path d="m15 8 10 6-10 6-10-6zM33 8l10 6-10 6-10-6zM15 22l10 6-10 6-10-6zM33 22l10 6-10 6-10-6zM24 31l10 6-10 6-10-6z" fill="#0061FF" />
    </svg>
  );
}

function AirtableLogo({ class_name }: { class_name?: string }) {
  return (
    <svg aria-hidden="true" className={class_name} viewBox="0 0 48 48">
      <path d="M23.2 6.8 7.8 13.2c-1.7.7-1.7 3.1 0 3.8l15.4 6.4c.5.2 1.1.2 1.6 0L40.2 17c1.7-.7 1.7-3.1 0-3.8L24.8 6.8a2 2 0 0 0-1.6 0z" fill="#FCB400" />
      <path d="M26.5 26.9v13.4c0 1.4 1.4 2.3 2.6 1.7l11.2-5.4c.7-.3 1.1-1 1.1-1.8V21.5z" fill="#18BFFF" />
      <path d="M21.5 26.9 6.6 21.5v13.3c0 .8.4 1.5 1.1 1.8L18.9 42c1.2.6 2.6-.3 2.6-1.7z" fill="#F82B60" />
    </svg>
  );
}

function ShopifyLogo({ class_name }: { class_name?: string }) {
  return (
    <svg aria-hidden="true" className={class_name} viewBox="0 0 48 48">
      <path d="M34.8 14.2 31 13.1c-.6-3.7-2.5-6.1-5.4-6.1-3.7 0-7.2 3.8-8.4 9l-4.8 1.5c-.8.2-1.1.6-1.2 1.4L7.7 40.2 31.2 44l10-2.5-4.3-25.8c-.1-.8-.5-1.2-1.3-1.5z" fill="#95BF47" />
      <path d="m34.8 14.2-3.6 29.7 10-2.4-4.3-25.8c-.1-.8-.5-1.2-1.3-1.5z" fill="#5E8E3E" opacity="0.9" />
      <path d="M25.3 9.2c1.2 0 2.1 1.1 2.6 3.1l-6.6 2.1c1-3.1 2.7-5.2 4-5.2zm-1 17.4c-2-.8-2.5-1.3-2.4-2.1.1-1 1.1-1.6 2.4-1.5 1.5.1 2.9.8 2.9.8l1.2-3.7s-1.1-.8-3.6-1c-4.2-.3-7.1 2.1-7.4 5.7-.3 2.7 1.5 4.8 4.4 6 2.3 1 3 1.6 2.9 2.7-.1 1.1-1.1 1.8-2.6 1.7-2-.1-3.9-1.2-3.9-1.2l-1.3 3.7s1.8 1.4 5 1.6c4.4.3 7.4-1.9 7.8-5.9.2-2.9-1.5-4.8-5.4-6.8z" fill="#fff" />
    </svg>
  );
}

function OutlookLogo({ class_name }: { class_name?: string }) {
  return (
    <svg aria-hidden="true" className={class_name} viewBox="0 0 48 48">
      <rect fill="#0078D4" height="30" rx="4" width="30" x="6" y="9" />
      <path d="M20 15h20a3 3 0 0 1 3 3v20a3 3 0 0 1-3 3H20z" fill="#106EBE" />
      <path d="m20 18 11.5 8.5L43 18v20a3 3 0 0 1-3 3H20z" fill="#50A7F0" />
      <path d="M31.5 26.5 20 35V18z" fill="#0B5CAB" opacity="0.65" />
      <text fill="#fff" fontFamily="Arial, sans-serif" fontSize="18" fontWeight="700" x="11" y="30">
        O
      </text>
    </svg>
  );
}

function SquareLogo({ class_name }: { class_name?: string }) {
  return (
    <svg aria-hidden="true" className={class_name} viewBox="0 0 48 48">
      <rect fill="#111827" height="34" rx="8" width="34" x="7" y="7" />
      <rect fill="none" height="16" rx="3" stroke="#fff" strokeWidth="5" width="16" x="16" y="16" />
    </svg>
  );
}

function MondayLogo({ class_name }: { class_name?: string }) {
  return (
    <svg aria-hidden="true" className={class_name} viewBox="0 0 48 48">
      <rect fill="#FF3D57" height="10" rx="5" transform="rotate(-35 12 30)" width="25" x="6" y="25" />
      <rect fill="#FFCB00" height="10" rx="5" transform="rotate(-35 25 30)" width="25" x="19" y="25" />
      <circle cx="38" cy="32" fill="#00CA72" r="5" />
    </svg>
  );
}

function LinearLogo({ class_name }: { class_name?: string }) {
  return (
    <svg aria-hidden="true" className={class_name} viewBox="0 0 48 48">
      <rect fill="#5E6AD2" height="34" rx="10" width="34" x="7" y="7" />
      <path d="M16 34 34 16M12 27l15-15M21 36l15-15M12 20l8-8" stroke="#fff" strokeLinecap="round" strokeWidth="4" />
    </svg>
  );
}

function brand_logo_for(icon: string, size: ConnectorIconSize): BrandLogo | null {
  const logo_size = LOGO_SCALE_CLASS[size];
  const lucide_size = LUCIDE_LOGO_SCALE_CLASS[size];
  const word_size = size === "lg" ? "text-[26px]" : "text-[17px]";
  const logos: Record<string, BrandLogo> = {
    gmail: { tone: "bg-white text-slate-900", node: <GmailLogo class_name={logo_size} /> },
    github: { tone: "bg-white text-slate-950", node: <LucideLogo class_name={lucide_size} icon={Github} /> },
    "feishu-docx": { tone: "bg-white text-slate-900", node: <FeishuLogo class_name={logo_size} /> },
    "x-twitter": { tone: "bg-white text-slate-950", node: <TextLogo class_name={size === "lg" ? "text-[32px]" : "text-[22px]"}>𝕏</TextLogo> },
    linkedin: { tone: "bg-[#0A66C2] text-white", node: <LucideLogo class_name={lucide_size} icon={Linkedin} /> },
    instagram: { tone: "bg-white text-[#E4405F]", node: <LucideLogo class_name={lucide_size} icon={Instagram} /> },
    youtube: { tone: "bg-white text-[#FF0000]", node: <LucideLogo class_name={lucide_size} icon={Youtube} /> },
    twitter: { tone: "bg-white text-[#1DA1F2]", node: <LucideLogo class_name={lucide_size} icon={Twitter} /> },
    slack: { tone: "bg-white text-[#4A154B]", node: <LucideLogo class_name={lucide_size} icon={Slack} /> },
    "google-calendar": { tone: "bg-white text-slate-900", node: <GoogleCalendarLogo class_name={logo_size} /> },
    "google-drive": { tone: "bg-white text-slate-900", node: <GoogleDriveLogo class_name={logo_size} /> },
    dropbox: { tone: "bg-white text-[#0061FF]", node: <DropboxLogo class_name={logo_size} /> },
    airtable: { tone: "bg-white text-slate-900", node: <AirtableLogo class_name={logo_size} /> },
    shopify: { tone: "bg-white text-[#5E8E3E]", node: <ShopifyLogo class_name={logo_size} /> },
    reddit: { tone: "bg-[#FF4500] text-white", node: <TextLogo class_name={word_size}>r</TextLogo> },
    tiktok: { tone: "bg-white text-slate-950", node: <TextLogo class_name={size === "lg" ? "text-[33px]" : "text-[23px]"}>♪</TextLogo> },
    odoo: { tone: "bg-[#714B67] text-white", node: <TextLogo class_name={size === "lg" ? "text-[20px]" : "text-[13px]"}>odoo</TextLogo> },
    square: { tone: "bg-white text-slate-950", node: <SquareLogo class_name={logo_size} /> },
    alibaba: { tone: "bg-[#FF6A00] text-white", node: <TextLogo class_name={word_size}>A</TextLogo> },
    outlook: { tone: "bg-white text-[#0078D4]", node: <OutlookLogo class_name={logo_size} /> },
    meta: { tone: "bg-white text-[#0866FF]", node: <TextLogo class_name={size === "lg" ? "text-[35px]" : "text-[24px]"}>∞</TextLogo> },
    ahrefs: { tone: "bg-[#FF8800] text-white", node: <TextLogo class_name={word_size}>a</TextLogo> },
    similarweb: { tone: "bg-white text-[#195AFE]", node: <TextLogo class_name={size === "lg" ? "text-[20px]" : "text-[13px]"}>sw</TextLogo> },
    notion: { tone: "bg-white text-slate-950", node: <TextLogo class_name={word_size}>N</TextLogo> },
    zapier: { tone: "bg-[#FF4A00] text-white", node: <TextLogo class_name={size === "lg" ? "text-[34px]" : "text-[24px]"}>*</TextLogo> },
    monday: { tone: "bg-white text-slate-900", node: <MondayLogo class_name={logo_size} /> },
    make: { tone: "bg-[#6D00CC] text-white", node: <TextLogo class_name={word_size}>M</TextLogo> },
    linear: { tone: "bg-white text-[#5E6AD2]", node: <LinearLogo class_name={logo_size} /> },
    atlassian: { tone: "bg-white text-[#0052CC]", node: <TextLogo class_name={word_size}>A</TextLogo> },
  };
  return logos[icon] ?? null;
}

export function ConnectorIcon({
  icon,
  title,
  size = "md",
  class_name,
}: ConnectorIconProps) {
  const logo = brand_logo_for(icon, size);
  const colors = get_connector_colors(icon);
  const letter = get_connector_letter(icon, title);

  return (
    <span
      className={cn(
        "flex shrink-0 items-center justify-center overflow-hidden border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_70%,transparent)] shadow-[0_1px_2px_rgba(15,23,42,0.04)]",
        ICON_SIZE_CLASS[size],
        logo ? logo.tone : [colors.bg, colors.text, "font-semibold"],
        class_name,
      )}
    >
      {logo ? logo.node : letter}
    </span>
  );
}
