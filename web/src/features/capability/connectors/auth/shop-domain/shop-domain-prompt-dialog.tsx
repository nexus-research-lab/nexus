"use client";

import { PromptDialog } from "@/shared/ui/dialog/decision/decision-dialog";

import { SHOP_DOMAIN_PROMPT_MESSAGE } from "./shop-domain-model";
import type { ShopDomainPromptState } from "./use-shop-domain-prompt";

interface ShopDomainPromptDialogProps {
  onCancel: () => void;
  onConfirm: (value: string) => void;
  state: ShopDomainPromptState;
}

export function ShopDomainPromptDialog({
  onCancel,
  onConfirm,
  state,
}: ShopDomainPromptDialogProps) {
  return (
    <PromptDialog
      defaultValue=""
      isOpen={state.kind === "open"}
      message={state.kind === "open" && state.error
        ? state.error
        : SHOP_DOMAIN_PROMPT_MESSAGE}
      onCancel={onCancel}
      onConfirm={onConfirm}
      placeholder="nexus-dev"
      title="Shopify 店铺"
    />
  );
}
