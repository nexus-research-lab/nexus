/**
 * INPUT: 按基础/进阶分组的引导、完成状态与打开/重置命令。
 * OUTPUT: 可滚动、无介绍套话的 plain 引导目录。
 * POS: Onboarding 引导选择边界；条目直接说明目的并进入对应导览或功能。
 */
"use client";

import { useId } from "react";
import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { BookOpen, Check, RotateCcw } from "lucide-react";

import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { UiButton, UiLinkButton } from "@/shared/ui/button/button";

import type {
  GuideCenterItem,
  GuideCenterSection,
} from "./guide-center-model";

interface GuideCenterDialogProps {
  closeLabel: string;
  docsLabel: string;
  isOpen: boolean;
  items: readonly GuideCenterItem[];
  onClose: () => void;
  onReset: () => void;
  resetLabel: string;
  reviewedLabel: string;
  sectionLabels: Record<GuideCenterSection, string>;
  title: string;
}

const GUIDE_CENTER_SECTIONS: readonly GuideCenterSection[] = [
  "basics",
  "advanced",
];

export function GuideCenterDialog({
  closeLabel,
  docsLabel,
  isOpen,
  items,
  onClose,
  onReset,
  resetLabel,
  reviewedLabel,
  sectionLabels,
  title,
}: GuideCenterDialogProps) {
  const titleId = useId();
  if (!isOpen) {
    return null;
  }

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        layer="tourDialog"
        labelledBy={titleId}
        onClose={onClose}
      >
        <div className="w-full max-w-lg">
          <UiDialogShell viewport="compactMax">
            <UiDialogHeader
              appearance="plain"
              closeLabel={closeLabel}
              onClose={onClose}
              title={title}
              titleId={titleId}
            />

            <UiDialogBody className="!px-5 !py-1" scrollable>
              {GUIDE_CENTER_SECTIONS.map((section) => {
                const sectionItems = items.filter((item) => item.section === section);
                if (sectionItems.length === 0) {
                  return null;
                }
                return (
                  <section className="py-2 first:pt-1 last:pb-1" key={section}>
                    <h3 className={cn("pb-1", getUiTypographyClassName({ role: "caption", tone: "muted", weight: "medium" }))}>
                      {sectionLabels[section]}
                    </h3>
                    <div className="divide-y divide-(--divider-subtle-color)">
                      {sectionItems.map((item) => {
                        const Icon = item.icon;
                        return (
                          <div
                            className="flex items-center gap-3 py-3 first:pt-2 last:pb-2"
                            key={item.id}
                          >
                            <Icon className="h-4 w-4 shrink-0 text-(--icon-muted)" />

                            <div className="min-w-0 flex-1">
                              <div className="flex min-w-0 flex-wrap items-center gap-2">
                                <h4 className={cn("wrap-anywhere", getUiTypographyClassName({ role: "control", tone: "strong", weight: "medium" }))}>
                                  {item.title}
                                </h4>
                                {item.completed ? (
                                  <span className={cn("inline-flex items-center gap-1", getUiTypographyClassName({ role: "metadata", tone: "brand", weight: "medium" }))}>
                                    <Check className="h-3 w-3" />
                                    {reviewedLabel}
                                  </span>
                                ) : null}
                              </div>
                              <p className={cn("mt-0.5 wrap-anywhere", getUiTypographyClassName({ role: "caption", tone: "soft" }))}>
                                {item.description}
                              </p>
                            </div>

                            <UiButton
                              className="shrink-0"
                              onClick={item.onAction}
                              size="xs"
                              tone="primary"
                              variant="text"
                            >
                              {item.actionLabel}
                            </UiButton>
                          </div>
                        );
                      })}
                    </div>
                  </section>
                );
              })}
            </UiDialogBody>

            <UiDialogFooter appearance="plain" className="!px-5 !py-3">
              <UiButton
                className="mr-auto"
                onClick={onReset}
                size="xs"
                tone="default"
                variant="text"
              >
                <RotateCcw className="h-3 w-3" />
                {resetLabel}
              </UiButton>
              <UiLinkButton
                href="https://www.nexusos.cn/docs"
                target="_blank"
                rel="noopener noreferrer"
                size="xs"
                variant="text"
              >
                <BookOpen className="h-3.5 w-3.5" />
                {docsLabel}
              </UiLinkButton>
              <UiButton
                onClick={onClose}
                size="xs"
                tone="default"
                variant="surface"
              >
                {closeLabel}
              </UiButton>
            </UiDialogFooter>
          </UiDialogShell>
        </div>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
