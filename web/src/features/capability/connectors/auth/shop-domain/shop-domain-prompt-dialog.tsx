// INPUT: 店铺域名输入事务状态与精确确认/取消命令。
// OUTPUT: 双语 Prompt，说明与字段错误分别表达且语言切换保留草稿。
// POS: Shopify 输入视图；复用公共命名、校验与键盘语义。
"use client";

import { PromptDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { useI18n } from "@/shared/i18n/i18n-context";

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
  const { t } = useI18n();
  return (
    <PromptDialog
      defaultValue=""
      error={state.kind === "open" && state.error ? t("capability.shop_domain_invalid") : undefined}
      inputLabel={t("capability.shop_domain_label")}
      isOpen={state.kind === "open"}
      message={t("capability.shop_domain_description")}
      onCancel={onCancel}
      onConfirm={onConfirm}
      placeholder="nexus-dev"
      title={t("capability.shop_domain_title")}
    />
  );
}
