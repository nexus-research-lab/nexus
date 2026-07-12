"use client";

import { RotateCcw } from "lucide-react";

import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { getDialogActionClassName } from "@/shared/ui/dialog/dialog-styles";

import type { GuideCenterItem } from "./guide-center-model";

interface GuideCenterDialogProps {
  closeLabel: string;
  description: string;
  isOpen: boolean;
  items: readonly GuideCenterItem[];
  onClose: () => void;
  onReset: () => void;
  resetLabel: string;
  reviewedLabel: string;
  title: string;
}

const GUIDE_CENTER_DESCRIPTION_ID = "onboarding-guide-center-description";
const GUIDE_CENTER_TITLE_ID = "onboarding-guide-center-title";

export function GuideCenterDialog({
  closeLabel,
  description,
  isOpen,
  items,
  onClose,
  onReset,
  resetLabel,
  reviewedLabel,
  title,
}: GuideCenterDialogProps) {
  if (!isOpen) {
    return null;
  }

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        className="z-[11050]"
        describedBy={GUIDE_CENTER_DESCRIPTION_ID}
        labelledBy={GUIDE_CENTER_TITLE_ID}
        onClose={onClose}
      >
        <div className="relative w-full max-w-lg">
          <img
            alt=""
            aria-hidden="true"
            className="pointer-events-none absolute -top-13 right-2 z-20 h-[92px] w-auto select-none drop-shadow-[0_14px_20px_rgba(68,74,120,0.12)] max-[520px]:hidden"
            src="/nexus/stickers/card-top.png"
          />
          <UiDialogShell>
            <UiDialogHeader
              className="relative z-30 !px-4 !py-3"
              closeLabel={closeLabel}
              onClose={onClose}
            >
              <div className="flex min-w-0 flex-1 items-center gap-3">
                <img
                  alt="Nexus"
                  className="h-9 w-9 shrink-0 object-contain"
                  src="/nexus/welcome.png"
                />
                <div className="min-w-0 flex-1">
                  <h3
                    className="text-[15px] font-bold tracking-tight text-(--text-strong)"
                    id={GUIDE_CENTER_TITLE_ID}
                  >
                    {title}
                  </h3>
                  <p
                    className="mt-0.5 text-[11px] leading-5 text-(--text-soft)"
                    id={GUIDE_CENTER_DESCRIPTION_ID}
                  >
                    {description}
                  </p>
                </div>
              </div>
            </UiDialogHeader>

            <UiDialogBody className="!px-4 !py-3 flex flex-col gap-2">
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
            </UiDialogBody>

            <UiDialogFooter className="!px-4 !py-2.5 border-t border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--modal-card-background)_84%,transparent)]">
              <button
                className={getDialogActionClassName("default", "compact")}
                onClick={onClose}
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
            </UiDialogFooter>
          </UiDialogShell>
        </div>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
