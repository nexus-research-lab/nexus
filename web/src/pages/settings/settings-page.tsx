/**
 * # !/usr/bin/env tsx
 * # -*- coding: utf-8 -*-
 * # =====================================================
 * # @File   ：settings-page.tsx
 * # @Date   ：2026/04/14 15:08
 * # @Author ：leemysw
 * # 2026/04/14 15:08   Create
 * # =====================================================
 */

"use client";

import { SettingsPanel } from "@/features/settings/settings-panel";
import { WorkspacePageFrame } from "@/shared/ui/workspace/frame/workspace-page-frame";

export function SettingsPage() {
  return (
    <WorkspacePageFrame content_padding_class_name="p-0">
      <SettingsPanel />
    </WorkspacePageFrame>
  );
}
