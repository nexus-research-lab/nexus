import type { ButtonHTMLAttributes, ReactNode } from "react";

import { UiButton } from "@/shared/ui/button/button";
import { UiListActionButton } from "@/shared/ui/list/list-action";

type CatalogActionTone = "default" | "danger";
type CatalogActionSize = "sm" | "md";
type CatalogTextActionTone = "default" | "primary" | "danger";

export function WorkspaceCatalogAction({
  children,
  className,
  tone = "default",
  size = "md",
  type = "button",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  children: ReactNode;
  tone?: CatalogActionTone;
  size?: CatalogActionSize;
}) {
  return (
    <UiListActionButton
      className={className}
      size={size === "sm" ? "xs" : "md"}
      tone={tone}
      type={type}
      visibility="visible"
      {...props}
    >
      {children}
    </UiListActionButton>
  );
}

export function WorkspaceCatalogTextAction({
  children,
  className,
  tone = "default",
  type = "button",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  children: ReactNode;
  tone?: CatalogTextActionTone;
}) {
  return (
    <UiButton
      className={className}
      size="sm"
      tone={tone}
      type={type}
      variant="text"
      {...props}
    >
      {children}
    </UiButton>
  );
}
