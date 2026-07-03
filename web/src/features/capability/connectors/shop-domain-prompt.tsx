"use client";

/* eslint-disable react-refresh/only-export-components */

import { useCallback, useState } from "react";
import { createRoot } from "react-dom/client";

import { PromptDialog } from "@/shared/ui/dialog/confirm-dialog";

const SHOP_PATTERN = /^[a-z0-9][a-z0-9-]*$/;

function normalizeShopDomain(value: string): string | null {
  const normalized = value.trim().toLowerCase().replace(/^https?:\/\//, "").replace(/\/.*$/, "").replace(/\.myshopify\.com$/, "");
  if (!SHOP_PATTERN.test(normalized)) {
    return null;
  }
  return normalized;
}

function ShopDomainPrompt({
  onFinish: onFinish,
}: {
  onFinish: (value: string | null) => void;
}) {
  const [error, setError] = useState<string | null>(null);

  const handleConfirm = useCallback(
    (value: string) => {
      const shop = normalizeShopDomain(value);
      if (!shop) {
        setError("请输入有效的 Shopify 店铺子域名");
        return;
      }
      onFinish(shop);
    },
    [onFinish],
  );

  return (
    <>
      <PromptDialog
        defaultValue=""
        isOpen
        message={error || "输入 myshopify.com 前面的店铺子域名。"}
        onCancel={() => onFinish(null)}
        onConfirm={handleConfirm}
        placeholder="nexus-dev"
        title="Shopify 店铺"
      />
    </>
  );
}

/** 打开 Shopify 店铺域名输入弹窗，返回规范化后的 shop 子域名。 */
export function openShopPrompt(): Promise<string | null> {
  if (typeof document === "undefined") {
    return Promise.resolve(null);
  }

  const host = document.createElement("div");
  document.body.appendChild(host);
  const root = createRoot(host);

  return new Promise((resolve) => {
    const finish = (value: string | null) => {
      root.unmount();
      host.remove();
      resolve(value);
    };

    root.render(<ShopDomainPrompt onFinish={finish} />);
  });
}
