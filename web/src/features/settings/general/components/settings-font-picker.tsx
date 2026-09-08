/** INPUT: 当前正文字体。OUTPUT: 本机字体目录与选择。POS: 外观页字体读取与交互边界。 */
import { useCallback, useEffect, useRef, useState } from "react";

import { getDesktopSystemFonts, isDesktopBridgeAvailable } from "@/lib/desktop-bridge";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";

type FontWindow = Window & {
  queryLocalFonts?: () => Promise<Array<{ family: string }>>;
};

export function SettingsFontPicker({ value, onChange }: { value: string; onChange: (font: string) => void }) {
  const { t } = useI18n();
  const [families, setFamilies] = useState<string[]>([]);
  const reading = useRef(false);
  const [failed, setFailed] = useState(false);
  const desktop = isDesktopBridgeAvailable();
  const canReadFonts = typeof window !== "undefined" && typeof (window as FontWindow).queryLocalFonts === "function";

  useEffect(() => {
    if (!desktop) return;
    let active = true;
    void getDesktopSystemFonts().then((result) => {
      if (active) setFamilies(result.families);
    }).catch(() => { if (active) setFailed(true); });
    return () => { active = false; };
  }, [desktop]);

  const readBrowserFonts = useCallback(async () => {
    if (desktop || !canReadFonts || reading.current) return;
    reading.current = true;
    setFailed(false);
    try {
      const fonts = await (window as FontWindow).queryLocalFonts!();
      setFamilies(fonts.map((font) => font.family));
    } catch {
      setFailed(true);
    } finally {
      reading.current = false;
    }
  }, [desktop, canReadFonts]);

  useEffect(() => {
    if (desktop || !canReadFonts || !navigator.permissions) return;
    let active = true;
    // 已授权时自动读取；首次请求留在下拉框打开事件中，保留浏览器所需的用户手势。
    void navigator.permissions.query({ name: "local-fonts" as PermissionName }).then((permission) => {
      if (active && permission.state === "granted") void readBrowserFonts();
    }).catch(() => { /* 不支持权限查询时，仍可在打开下拉框时读取。 */ });
    return () => { active = false; };
  }, [desktop, canReadFonts, readBrowserFonts]);

  const presets = ["default", "system", "serif"] as const;
  const names = [...new Set([...families, value])]
    .filter((name) => name && !presets.some((preset) => preset === name))
    .sort((a, b) => a.localeCompare(b));

  return (
    <div className="space-y-2">
      <UiSelectMenu
        ariaLabel={t("settings.reading.font")}
        buttonClassName="w-full border border-(--divider-subtle-color) bg-transparent shadow-none"
        menuMinWidth={280}
        value={value}
        onChange={onChange}
        onOpen={() => { if (families.length === 0) void readBrowserFonts(); }}
        options={[
          ...presets.map((preset) => ({ value: preset, label: t(`settings.reading.${preset}`) })),
          ...names.map((name) => ({ value: name, label: name })),
        ]}
      />
      {!desktop && (!canReadFonts || failed) && families.length === 0 && (
        <input
          aria-label={t("settings.reading.custom_font")}
          className="h-9 w-full rounded-lg border border-(--divider-subtle-color) bg-transparent px-3 text-sm"
          placeholder={t("settings.reading.font_hint")}
          maxLength={100}
          value={presets.some((preset) => preset === value) ? "" : value}
          onChange={(event) => onChange(event.target.value)}
        />
      )}
      {failed && <p role="status" className="text-xs text-(--text-soft)">{t("settings.reading.font_error")}</p>}
    </div>
  );
}
