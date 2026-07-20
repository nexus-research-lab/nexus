import type {
  ButtonHTMLAttributes,
  HTMLAttributes,
  ReactNode,
} from "react";

import { cn } from "@/shared/ui/class-name";

type CatalogCardSize = "compact" | "catalog" | "comfort" | "panel";
type CatalogCardAlign = "start" | "center";

const CATALOG_CARD_SIZE_CLASSES: Record<CatalogCardSize, string> = {
  compact: "min-h-[138px] rounded-[12px] px-4 py-4",
  catalog: "min-h-[170px] rounded-[12px] px-5 py-4",
  comfort: "rounded-[14px] px-6 py-6",
  panel: "rounded-[14px] px-5 py-5 sm:px-6 sm:py-6",
};

export function WorkspaceCatalogCard({
  children,
  className,
  muted = false,
  size = "catalog",
  align = "start",
  ...props
}: Omit<
  HTMLAttributes<HTMLElement>,
  "onClick" | "onKeyDown" | "role" | "tabIndex"
> & {
  children: ReactNode;
  muted?: boolean;
  size?: CatalogCardSize;
  align?: CatalogCardAlign;
}) {
  return (
    <article
      className={cn(
        "flex flex-col border border-(--divider-subtle-color) bg-transparent transition duration-(--motion-duration-fast) ease-out",
        CATALOG_CARD_SIZE_CLASSES[size],
        align === "center" && "items-center text-center",
        muted && "opacity-70",
        className,
      )}
      {...props}
    >
      {children}
    </article>
  );
}

export function WorkspaceCatalogGhostAction({
  children,
  className,
  size = "comfort",
  type = "button",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  children: ReactNode;
  size?: Extract<CatalogCardSize, "compact" | "catalog" | "comfort" | "panel">;
}) {
  return (
    <button
      className={cn(
        "flex w-full cursor-pointer flex-col items-center justify-center border border-dashed border-(--divider-subtle-color) bg-transparent text-center transition duration-(--motion-duration-fast) ease-out hover:border-(--surface-interactive-active-border) hover:bg-(--surface-interactive-hover-background)",
        CATALOG_CARD_SIZE_CLASSES[size],
        className,
      )}
      type={type}
      {...props}
    >
      {children}
    </button>
  );
}
