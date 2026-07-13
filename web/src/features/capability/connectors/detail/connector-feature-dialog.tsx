import { Check } from "lucide-react";

import { UiBadge } from "@/shared/ui/display/badge";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { UiPanel } from "@/shared/ui/panel";
import type { ConnectorFeatureDetail } from "@/types/capability/connector";

interface ConnectorFeatureDialogProps {
  connectorTitle: string;
  feature: ConnectorFeatureDetail | null;
  onClose: () => void;
}

export function ConnectorFeatureDialog({
  connectorTitle,
  feature,
  onClose,
}: ConnectorFeatureDialogProps) {
  if (!feature) {
    return null;
  }
  return (
    <UiDialogPortal>
      <UiDialogBackdrop className="z-[9999]" onClose={onClose}>
        <UiDialogShell className="max-h-[min(84vh,640px)]" size="lg">
          <UiDialogHeader
            icon={<Check className="h-4 w-4" />}
            onClose={onClose}
            subtitle={`${connectorTitle} 能力`}
            title={feature.name}
          />
          <UiDialogBody className="space-y-4" scrollable>
            <p className="text-[14px] leading-7 text-(--text-default)">
              {feature.description}
            </p>
            {feature.items?.length ? (
              <UiPanel padding="sm" radius="sm" variant="inset">
                <div className="mb-2 text-[12px] font-semibold text-(--text-strong)">
                  能力范围
                </div>
                <div className="space-y-2">
                  {feature.items.map((item) => (
                    <div
                      className="flex gap-2 text-[13px] leading-6 text-(--text-default)"
                      key={item}
                    >
                      <Check className="mt-1 h-3.5 w-3.5 shrink-0 text-(--primary)" />
                      <span>{item}</span>
                    </div>
                  ))}
                </div>
              </UiPanel>
            ) : null}
            {feature.scopes?.length ? (
              <div>
                <div className="mb-2 text-[12px] font-medium text-(--text-muted)">
                  相关 OAuth scopes
                </div>
                <div className="flex flex-wrap gap-1.5">
                  {feature.scopes.map((scope) => (
                    <UiBadge key={scope} size="xs">
                      {scope}
                    </UiBadge>
                  ))}
                </div>
              </div>
            ) : null}
          </UiDialogBody>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
