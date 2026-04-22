"use client";

/* eslint-disable react-refresh/only-export-components */

import { useCallback, useState } from "react";
import { createRoot } from "react-dom/client";

import { PromptDialog } from "@/shared/ui/dialog/confirm-dialog";

const SHOP_PATTERN = /^[a-z0-9][a-z0-9-]*$/;

function normalize_shop_domain(value: string): string | null {
  const normalized = value.trim().toLowerCase().replace(/^https?:\/\//, "").replace(/\/.*$/, "").replace(/\.myshopify\.com$/, "");
  if (!SHOP_PATTERN.test(normalized)) {
    return null;
  }
  return normalized;
}

function ShopDomainPrompt({
  on_finish,
}: {
  on_finish: (value: string | null) => void;
}) {
  const [error, set_error] = useState<string | null>(null);

  const handle_confirm = useCallback(
    (value: string) => {
      const shop = normalize_shop_domain(value);
      if (!shop) {
        set_error("请输入有效的 Shopify 店铺子域名");
        return;
      }
      on_finish(shop);
    },
    [on_finish],
  );

  return (
    <>
      <PromptDialog
        default_value=""
        is_open
        message={error || "输入 myshopify.com 前面的店铺子域名。"}
        on_cancel={() => on_finish(null)}
        on_confirm={handle_confirm}
        placeholder="nexus-dev"
        title="Shopify 店铺"
      />
    </>
  );
}

/** 打开 Shopify 店铺域名输入弹窗，返回规范化后的 shop 子域名。 */
export function open_shop_prompt(): Promise<string | null> {
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

    root.render(<ShopDomainPrompt on_finish={finish} />);
  });
}
