// INPUT: Masked credential, write-only draft, permissions and explicit credential commands.
// OUTPUT: Separate replace/save/cancel/clear controls; clearing confirms removal and disabling.
// POS: Provider credential editor. Never treats an empty replacement draft as key removal.
import { ExternalLink } from "lucide-react";
import { useId, useRef } from "react";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { isImeKeyboardEvent } from "@/shared/lib/browser/ime-keyboard-event";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { UiField, UiInput } from "@/shared/ui/form/form-control";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { ProviderConfigRecord } from "@/types/capability/provider";

interface ProviderSettingsKeyFieldProps {
  record: ProviderConfigRecord | null;
  value: string;
  disabled: boolean;
  pending: boolean;
  isEditing: boolean;
  keyURL?: string;
  providerTitle: string;
  onChange: (value: string) => void;
  onSave: () => void;
  onClear: () => void;
}

export function ProviderSettingsKeyField({ record, value, disabled, pending, isEditing,
  keyURL, providerTitle, onChange, onSave, onClear }: ProviderSettingsKeyFieldProps) {
  const { t } = useI18n();
  const id = useId();
  const inputRef = useRef<HTMLInputElement>(null);
  const identity = `${record?.id}:${record?.configuration_version}`;
  const [replacing, setReplacing] = useResettableState(false, identity);
  const [clearing, setClearing] = useResettableState(false, identity);
  const hasKey = !!record?.auth_token_masked;
  const editable = !hasKey || replacing;
  const unavailable = disabled || pending;
  return (
    <>
      <UiField htmlFor={id} label={t("settings.providers.api_key")} required={!isEditing}>
        <div className="flex flex-wrap items-center gap-2">
          <UiInput
            ref={inputRef} id={id} className="min-w-0 flex-1" controlSize="md"
            autoCapitalize="off" autoComplete="off" autoCorrect="off" spellCheck={false}
            data-form-type="other" data-lpignore="true" name="provider-auth-token"
            disabled={unavailable} readOnly={!editable} required={!isEditing}
            type="password" value={editable ? value : ""}
            placeholder={editable ? t("settings.providers.api_key_placeholder") : record?.auth_token_masked}
            onChange={(event) => onChange(event.target.value)}
            onBlur={!isEditing ? onSave : undefined}
            onKeyDown={(event) => {
              if (event.key === "Enter" && !isImeKeyboardEvent(event.nativeEvent) && editable && value.trim() && !unavailable) {
                event.preventDefault();
                onSave();
              }
            }}
          />
          {hasKey && !replacing ? (
            <>
              <UiButton size="sm" variant="surface" disabled={unavailable} onClick={() => {
                onChange("");
                setReplacing(true);
                inputRef.current?.focus();
              }}>{t("settings.providers.replace_key")}</UiButton>
              <UiButton size="sm" variant="ghost" disabled={unavailable} onClick={() => setClearing(true)}>
                {t("settings.providers.clear_key")}
              </UiButton>
            </>
          ) : isEditing ? (
            <>
              <UiButton size="sm" variant="surface" disabled={unavailable || !value.trim()} onClick={onSave}>
                {t("common.save")}
              </UiButton>
              {replacing ? <UiButton size="sm" variant="ghost" disabled={pending} onClick={() => {
                onChange(""); setReplacing(false);
              }}>{t("common.cancel")}</UiButton> : null}
            </>
          ) : null}
        </div>
        {keyURL ? <a className={`inline-flex items-center gap-1 hover:underline ${getUiTypographyClassName({ role: "supporting", tone: "brand", weight: "medium" })}`}
          href={keyURL} rel="noreferrer" target="_blank">
          {t("settings.providers.get_api_key_from", { name: providerTitle })}
          <ExternalLink aria-hidden="true" className="h-3.5 w-3.5" />
        </a> : null}
      </UiField>
      <ConfirmDialog isOpen={clearing} title={t("settings.providers.clear_key")}
        message={t("settings.providers.clear_key_message", { name: providerTitle })}
        confirmText={t("settings.providers.clear_key")} variant="danger"
        onCancel={() => setClearing(false)} onConfirm={() => {
          if (!unavailable) { setClearing(false); onClear(); }
        }} />
    </>
  );
}
