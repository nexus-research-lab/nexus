// INPUT: 当前正文字体与本机字体目录能力。
// OUTPUT: 共享字体选择器、跨宿主空/失败目录的文本兜底、读取状态与显式重开重试。
// POS: 外观页字体读取与交互边界；不读取或上传字体文件。
import { useCallback, useEffect, useRef, useState } from "react";

import { getDesktopSystemFonts, isDesktopBridgeAvailable } from "@/lib/desktop-bridge";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiInput } from "@/shared/ui/form/form-control";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

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

  const generation = useRef(0);
  const [loading, setLoading] = useState(false);
  const [hasRead, setHasRead] = useState(false);

  useEffect(() => () => {
    generation.current += 1;
    reading.current = false;
  }, []);

  const readFonts = useCallback(async () => {
    if ((!desktop && !canReadFonts) || reading.current) return;
    const request = ++generation.current;
    reading.current = true;
    setLoading(true);
    setFailed(false);
    try {
      const names = desktop
        ? (await getDesktopSystemFonts()).families
        : (await (window as FontWindow).queryLocalFonts!()).map((font) => font.family);
      if (request === generation.current) setFamilies(names);
    } catch {
      if (request === generation.current) setFailed(true);
    } finally {
      if (request === generation.current) {
        reading.current = false;
        setLoading(false);
        setHasRead(true);
      }
    }
  }, [desktop, canReadFonts]);

  useEffect(() => {
    if (desktop) void readFonts();
  }, [desktop, readFonts]);

  useEffect(() => {
    if (desktop || !canReadFonts || !navigator.permissions) return;
    let active = true;
    // 已授权时自动读取；首次请求留在下拉框打开事件中，保留浏览器所需的用户手势。
    void navigator.permissions.query({ name: "local-fonts" as PermissionName }).then((permission) => {
      if (active && permission.state === "granted") void readFonts();
    }).catch(() => { /* 不支持权限查询时，仍可在打开下拉框时读取。 */ });
    return () => { active = false; };
  }, [desktop, canReadFonts, readFonts]);

  const presets = ["default", "system", "serif"] as const;
  const names = [...new Set([...families, value])]
    .filter((name) => name && !presets.some((preset) => preset === name))
    .sort((a, b) => a.localeCompare(b));

  return (
    <div className="space-y-2">
      <UiSelectMenu
        ariaLabel={t("settings.reading.font")}
        className="w-full"
        menuMinWidth={280}
        size="lg"
        value={value}
        onChange={onChange}
        onOpen={() => { if (families.length === 0) void readFonts(); }}
        options={[
          ...presets.map((preset) => ({ value: preset, label: t(`settings.reading.${preset}`) })),
          ...names.map((name) => ({ value: name, label: name })),
        ]}
      />
      {(hasRead || (!desktop && !canReadFonts)) && families.length === 0 && (
        <UiInput
          aria-label={t("settings.reading.custom_font")}
          className="w-full"
          controlSize="lg"
          placeholder={t("settings.reading.font_hint")}
          maxLength={100}
          value={presets.some((preset) => preset === value) ? "" : value}
          onChange={(event) => onChange(event.target.value)}
        />
      )}
      {(loading || failed) && (
        <p className={getUiTypographyClassName({ role: "caption", tone: "soft" })} role="status">
          {t(loading ? "common.loading" : "settings.reading.font_error")}
        </p>
      )}
    </div>
  );
}
