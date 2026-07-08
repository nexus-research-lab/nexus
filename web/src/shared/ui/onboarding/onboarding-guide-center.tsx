"use client";

import { type MouseEvent } from "react";
import { createPortal } from "react-dom";
import { type LucideIcon, RotateCcw, X } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import {
  DIALOG_BACKDROP_CLASS_NAME,
  DIALOG_ICON_BUTTON_CLASS_NAME,
  getDialogActionClassName,
} from "@/shared/ui/dialog/dialog-styles";

export interface OnboardingGuideCenterItem {
  id: string;
  icon: LucideIcon;
  title: string;
  description: string;
  actionLabel: string;
  completed?: boolean;
  onAction: () => void;
}

interface OnboardingGuideCenterProps {
  isOpen: boolean;
  title: string;
  description: string;
  items: OnboardingGuideCenterItem[];
  resetLabel: string;
  closeLabel: string;
  reviewedLabel: string;
  onClose: () => void;
  onReset: () => void;
}

export function OnboardingGuideCenter({
  isOpen: isOpen,
  title,
  description,
  items,
  resetLabel: resetLabel,
  closeLabel: closeLabel,
  reviewedLabel: reviewedLabel,
  onClose: onClose,
  onReset: onReset,
}: OnboardingGuideCenterProps) {
  if (!isOpen || typeof document === "undefined") {
    return null;
  }

  const handleCloseClick = (event: MouseEvent<HTMLButtonElement>) => {
    event.preventDefault();
    event.stopPropagation();
    onClose();
  };

  const dialog = (
    // eslint-disable-next-line jsx-a11y/no-noninteractive-element-interactions -- backdrop click-to-close + Escape is a standard modal dialog pattern
    <div
      aria-describedby="onboarding-guide-center-description"
      aria-labelledby="onboarding-guide-center-title"
      aria-modal="true"
      className={`${DIALOG_BACKDROP_CLASS_NAME} z-[11050]`}
      data-modal-root="true"
      onClick={onClose}
      onKeyDown={(event) => {
        if (event.key === "Escape") {
          onClose();
        }
      }}
      role="dialog"
    >
      <div
        className="relative w-full max-w-lg"
        onClick={(event) => event.stopPropagation()}
        onKeyDown={(event) => event.stopPropagation()}
        onPointerDown={(event) => event.stopPropagation()}
        role="presentation"
      >
        <img
          alt=""
          aria-hidden="true"
          className="pointer-events-none absolute -top-13 right-2 z-20 h-[92px] w-auto select-none drop-shadow-[0_14px_20px_rgba(68,74,120,0.12)] max-[520px]:hidden"
          src="/nexus/stickers/card-top.png"
        />
        <section className="dialog-shell surface-radius-md flex w-full flex-col overflow-hidden">
          <div className="dialog-header relative z-30 !px-4 !py-3">
            <div className="flex min-w-0 flex-1 items-center gap-3">
              <img
                alt="Nexus"
                className="h-9 w-9 shrink-0 object-contain"
                src="/nexus/welcome.png"
              />
              <div className="min-w-0 flex-1">
                <h3
                  className="text-[15px] font-bold tracking-tight text-(--text-strong)"
                  id="onboarding-guide-center-title"
                >
                  {title}
                </h3>
                <p
                  className="mt-0.5 text-[11px] leading-5 text-(--text-soft)"
                  id="onboarding-guide-center-description"
                >
                  {description}
                </p>
              </div>
            </div>

            <button
              aria-label={closeLabel}
              className={cn(DIALOG_ICON_BUTTON_CLASS_NAME, "relative z-30 shrink-0")}
              onClick={handleCloseClick}
              onPointerDown={(event) => event.stopPropagation()}
              type="button"
            >
              <X className="h-4 w-4" />
            </button>
          </div>

          <div className="dialog-body !px-4 !py-3 flex flex-col gap-2">
            {items.map((item) => {
              const Icon = item.icon;
              return (
                <section
                  className="rounded-[16px] border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-panel-background)_92%,white)] px-3 py-2.5"
                  key={item.id}
                >
                  <div className="flex items-start gap-2.5">
                    <div className="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-[14px] bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-(--primary)">
                      <Icon className="h-3.5 w-3.5" />
                    </div>

                    <div className="min-w-0 flex-1">
                      <div className="flex min-w-0 flex-wrap items-center gap-1.5">
                        <h4 className="text-[13px] font-semibold text-(--text-strong)">
                          {item.title}
                        </h4>
                        {item.completed ? (
                          <span className="rounded-[6px] border border-[color:color-mix(in_srgb,var(--primary)_14%,transparent)] bg-transparent px-1.5 py-0 text-[10px] font-medium text-(--primary)">
                            {reviewedLabel}
                          </span>
                        ) : null}
                      </div>
                      <p className="mt-0.5 text-[11px] leading-5 text-(--text-soft)">
                        {item.description}
                      </p>
                    </div>

                    <button
                      className={getDialogActionClassName("primary", "compact", "shrink-0")}
                      onClick={item.onAction}
                      type="button"
                    >
                      {item.actionLabel}
                    </button>
                  </div>
                </section>
              );
            })}
          </div>

          <div className="dialog-footer !px-4 !py-2.5 border-t border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--modal-card-background)_84%,transparent)]">
            <button
              className={getDialogActionClassName("default", "compact")}
              onClick={handleCloseClick}
              type="button"
            >
              {closeLabel}
            </button>
            <button
              className={getDialogActionClassName(
                "default",
                "inline-flex items-center gap-1.5 text-[11px]",
              )}
              onClick={onReset}
              type="button"
            >
              <RotateCcw className="h-3 w-3" />
              {resetLabel}
            </button>
          </div>
        </section>
      </div>
    </div>
  );

  return createPortal(dialog, document.body);
}
