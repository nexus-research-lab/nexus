// INPUT: Native button attributes, catalog text action content and semantic tone.
// OUTPUT: Catalog text actions delegated to the shared Button primitive.
// POS: Catalog text action adapter; Button owns interaction states and visual recipes.

import type { ButtonHTMLAttributes, ReactNode } from "react";

import { UiButton } from "@/shared/ui/button/button";
type CatalogTextActionTone = "default" | "primary" | "danger";

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
