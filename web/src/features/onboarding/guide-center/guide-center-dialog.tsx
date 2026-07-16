"use client";

import { Check, RotateCcw } from "lucide-react";

import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { getUiButtonClassName } from "@/shared/ui/button/button-styles";

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
        <div className="w-full max-w-lg">
          <UiDialogShell>
            <UiDialogHeader
              className="!px-5 !py-4"
              closeLabel={closeLabel}
              onClose={onClose}
            >
              <div className="min-w-0 flex-1">
                <h3
                  className="text-[16px] font-semibold tracking-tight text-(--text-strong)"
                  id={GUIDE_CENTER_TITLE_ID}
                >
                  {title}
                </h3>
                <p
                  className="mt-1 text-[12px] leading-5 text-(--text-soft)"
                  id={GUIDE_CENTER_DESCRIPTION_ID}
                >
                  {description}
                </p>
              </div>
            </UiDialogHeader>

            <UiDialogBody className="!px-5 !py-1">
              <div className="divide-y divide-(--divider-subtle-color)">
                {items.map((item) => {
                  const Icon = item.icon;
                  return (
                    <div
                      className="flex items-center gap-3 py-3 first:pt-2 last:pb-2"
                      key={item.id}
                    >
                      <Icon className="h-4 w-4 shrink-0 text-(--icon-muted)" />

                      <div className="min-w-0 flex-1">
                        <div className="flex min-w-0 flex-wrap items-center gap-2">
                          <h4 className="text-[13px] font-medium text-(--text-strong)">
                            {item.title}
                          </h4>
                          {item.completed ? (
                            <span className="inline-flex items-center gap-1 text-[10px] font-medium text-(--primary)">
                              <Check className="h-3 w-3" />
                              {reviewedLabel}
                            </span>
                          ) : null}
                        </div>
                        <p className="mt-0.5 text-[11px] leading-5 text-(--text-soft)">
                          {item.description}
                        </p>
                      </div>

                      <button
                        className={getUiButtonClassName(
                          { size: "xs", tone: "primary", variant: "text" },
                          "shrink-0 px-1.5 font-medium",
                        )}
                        onClick={item.onAction}
                        type="button"
                      >
                        {item.actionLabel}
                      </button>
                    </div>
                  );
                })}
              </div>
            </UiDialogBody>

            <UiDialogFooter className="!px-5 !py-3">
              <button
                className={getUiButtonClassName(
                  { size: "xs", tone: "default", variant: "text" },
                  "mr-auto px-1 font-medium",
                )}
                onClick={onReset}
                type="button"
              >
                <RotateCcw className="h-3 w-3" />
                {resetLabel}
              </button>
              <button
                className={getUiButtonClassName(
                  { size: "xs", tone: "default", variant: "surface" },
                  "font-medium",
                )}
                onClick={onClose}
                type="button"
              >
                {closeLabel}
              </button>
            </UiDialogFooter>
          </UiDialogShell>
        </div>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
