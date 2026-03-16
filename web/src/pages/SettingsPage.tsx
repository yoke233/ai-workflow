import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Check, Upload, Trash2, Sun, Moon, Palette, Download, Loader2 } from "lucide-react";
import { cn } from "@/lib/utils";
import {
  useSettingsStore,
  type FontSize,
  type StoredVscodeTheme,
  type BundledThemeEntry,
  type UserThemeEntry,
} from "@/stores/settingsStore";
import { parseVscodeTheme } from "@/lib/vscodeTheme";

// ── Built-in accent theme definitions (light, CSS-only) ──

const ACCENT_THEMES = [
  { id: "slate", labelKey: "settings.themeSlate", accent: "#475569", type: "light" as const },
  { id: "ocean", labelKey: "settings.themeOcean", accent: "#0284c7", type: "light" as const },
  { id: "forest", labelKey: "settings.themeForest", accent: "#059669", type: "light" as const },
  { id: "amber", labelKey: "settings.themeAmber", accent: "#d97706", type: "light" as const },
];

const FONT_SIZES: { value: FontSize; labelKey: string; px: string }[] = [
  { value: "sm", labelKey: "settings.fontSm", px: "13px" },
  { value: "md", labelKey: "settings.fontMd", px: "15px" },
  { value: "lg", labelKey: "settings.fontLg", px: "17px" },
];

export function SettingsPage() {
  const { t } = useTranslation();
  const {
    theme,
    fontSize,
    userThemeEntries,
    userThemeCache,
    bundledThemes,
    bundledThemeCache,
    bundledLoading,
    setTheme,
    setFontSize,
    addCustomTheme,
    removeCustomTheme,
    loadUserThemes,
    activateUserTheme,
    loadBundledManifest,
    activateBundledTheme,
  } = useSettingsStore();

  const fileRef = useRef<HTMLInputElement>(null);
  const [importError, setImportError] = useState<string | null>(null);
  const [dragOver, setDragOver] = useState(false);

  // Load manifests on mount
  useEffect(() => {
    void loadBundledManifest();
    void loadUserThemes();
  }, [loadBundledManifest, loadUserThemes]);

  const handleFileImport = useCallback(
    async (file: File) => {
      setImportError(null);
      try {
        const text = await file.text();
        const parsed = parseVscodeTheme(text, file.name);
        await addCustomTheme(parsed, text);
      } catch (err) {
        setImportError(err instanceof Error ? err.message : String(err));
      }
    },
    [addCustomTheme],
  );

  const onFileChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];
      if (file) void handleFileImport(file);
      e.target.value = "";
    },
    [handleFileImport],
  );

  const onDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      setDragOver(false);
      const file = e.dataTransfer.files[0];
      if (file && file.name.endsWith(".json")) {
        void handleFileImport(file);
      } else {
        setImportError(t("settings.jsonOnly"));
      }
    },
    [handleFileImport, t],
  );

  return (
    <div className="mx-auto max-w-3xl px-6 py-8 space-y-8">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t("settings.title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("settings.subtitle")}</p>
      </div>

      {/* ── Light Accent Themes ── */}
      <section className="space-y-3">
        <h2 className="text-base font-semibold">{t("settings.builtinThemes")}</h2>
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          {ACCENT_THEMES.map((bt) => (
            <ThemeCard
              key={bt.id}
              active={theme === bt.id}
              onClick={() => setTheme(bt.id)}
              label={t(bt.labelKey)}
              type={bt.type}
              previewBg="#ffffff"
              previewFg="#1e293b"
              previewAccent={bt.accent}
              previewBorder="#e2e8f0"
            />
          ))}
        </div>
      </section>

      {/* ── Bundled VSCode Themes (from /themes/) ── */}
      {(bundledThemes.length > 0 || bundledLoading) && (
        <section className="space-y-3">
          <div className="flex items-center gap-2">
            <h2 className="text-base font-semibold">{t("settings.bundledThemes")}</h2>
            {bundledLoading && <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />}
          </div>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
            {bundledThemes.map((entry) => (
              <BundledThemeCard
                key={entry.id}
                entry={entry}
                active={theme === entry.id}
                cached={!!bundledThemeCache[entry.id]}
                onSelect={() => void activateBundledTheme(entry.id)}
              />
            ))}
          </div>
        </section>
      )}

      {/* ── User-imported VSCode Themes ── */}
      <section className="space-y-3">
        <div className="flex items-center justify-between">
          <h2 className="text-base font-semibold">{t("settings.userThemes")}</h2>
          <button
            onClick={() => fileRef.current?.click()}
            className="inline-flex items-center gap-1.5 rounded-md bg-primary px-3 py-1.5 text-xs font-medium text-primary-foreground transition-colors hover:bg-primary/90"
          >
            <Upload className="h-3.5 w-3.5" />
            {t("settings.importTheme")}
          </button>
          <input ref={fileRef} type="file" accept=".json" className="hidden" onChange={onFileChange} />
        </div>

        {/* Drop zone */}
        <div
          onDragOver={(e) => { e.preventDefault(); setDragOver(true); }}
          onDragLeave={() => setDragOver(false)}
          onDrop={onDrop}
          className={cn(
            "flex flex-col items-center justify-center rounded-lg border-2 border-dashed px-6 py-8 text-center transition-colors",
            dragOver ? "border-primary bg-primary/5" : "border-border",
          )}
        >
          <Palette className="mb-2 h-8 w-8 text-muted-foreground" />
          <p className="text-sm text-muted-foreground">{t("settings.dropThemeHint")}</p>
          <p className="mt-1 text-xs text-muted-foreground/60">{t("settings.dropThemeFormats")}</p>
        </div>

        {importError && (
          <p className="text-sm text-destructive">{importError}</p>
        )}

        {/* User-imported theme list */}
        {userThemeEntries.length > 0 && (
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
            {userThemeEntries.map((entry) => {
              const cached = userThemeCache[entry.id];
              return cached ? (
                <CustomThemeCard
                  key={entry.id}
                  theme={cached}
                  active={theme === entry.id}
                  onSelect={() => setTheme(entry.id)}
                  onRemove={() => void removeCustomTheme(entry.id)}
                  removeLabel={t("common.delete")}
                />
              ) : (
                <UserThemeEntryCard
                  key={entry.id}
                  entry={entry}
                  active={theme === entry.id}
                  onSelect={() => void activateUserTheme(entry.id)}
                  onRemove={() => void removeCustomTheme(entry.id)}
                  removeLabel={t("common.delete")}
                />
              );
            })}
          </div>
        )}
      </section>

      {/* ── Font Size ── */}
      <section className="space-y-3">
        <h2 className="text-base font-semibold">{t("settings.fontSize")}</h2>
        <div className="flex gap-2">
          {FONT_SIZES.map((fs) => (
            <button
              key={fs.value}
              onClick={() => setFontSize(fs.value)}
              className={cn(
                "flex items-center gap-2 rounded-md border px-4 py-2 text-sm font-medium transition-colors",
                fontSize === fs.value
                  ? "border-primary bg-primary/10 text-primary"
                  : "border-border hover:bg-accent",
              )}
            >
              {fontSize === fs.value && <Check className="h-3.5 w-3.5" />}
              {t(fs.labelKey)}
              <span className="text-xs text-muted-foreground">({fs.px})</span>
            </button>
          ))}
        </div>
      </section>
    </div>
  );
}

// ── Sub-components ──

function ThemeCard({
  active,
  onClick,
  label,
  type,
  previewBg,
  previewFg,
  previewAccent,
  previewBorder,
}: {
  active: boolean;
  onClick: () => void;
  label: string;
  type: "dark" | "light";
  previewBg: string;
  previewFg: string;
  previewAccent: string;
  previewBorder: string;
}) {
  return (
    <button
      onClick={onClick}
      className={cn(
        "group relative flex flex-col rounded-lg border-2 p-3 text-left transition-all hover:shadow-sm",
        active ? "border-primary shadow-sm" : "border-border hover:border-primary/40",
      )}
    >
      <MiniPreview bg={previewBg} fg={previewFg} accent={previewAccent} border={previewBorder} />
      <div className="flex items-center gap-1.5">
        {active && <Check className="h-3.5 w-3.5 text-primary" />}
        <span className="text-sm font-medium">{label}</span>
        <TypeIcon type={type} />
      </div>
    </button>
  );
}

function BundledThemeCard({
  entry,
  active,
  cached,
  onSelect,
}: {
  entry: BundledThemeEntry;
  active: boolean;
  cached: boolean;
  onSelect: () => void;
}) {
  const previewColors = cached
    ? useSettingsStore.getState().bundledThemeCache[entry.id]?.previewColors
    : null;

  return (
    <button
      onClick={onSelect}
      className={cn(
        "group relative flex flex-col rounded-lg border-2 p-3 text-left transition-all hover:shadow-sm",
        active ? "border-primary shadow-sm" : "border-border hover:border-primary/40",
      )}
    >
      {previewColors ? (
        <MiniPreview
          bg={previewColors.background}
          fg={previewColors.foreground}
          accent={previewColors.primary}
          border={previewColors.border}
        />
      ) : (
        <BundledPreviewPlaceholder type={entry.type} />
      )}
      <div className="flex items-center gap-1.5">
        {active && <Check className="h-3.5 w-3.5 text-primary" />}
        <span className="truncate text-sm font-medium">{entry.name}</span>
        {!cached && <Download className="ml-auto h-3 w-3 text-muted-foreground opacity-50" />}
        {cached && <TypeIcon type={entry.type} />}
      </div>
    </button>
  );
}

function CustomThemeCard({
  theme: ct,
  active,
  onSelect,
  onRemove,
  removeLabel,
}: {
  theme: StoredVscodeTheme;
  active: boolean;
  onSelect: () => void;
  onRemove: () => void;
  removeLabel: string;
}) {
  return (
    <div
      className={cn(
        "group relative flex flex-col rounded-lg border-2 p-3 text-left transition-all",
        active ? "border-primary shadow-sm" : "border-border hover:border-primary/40",
      )}
    >
      <button
        onClick={(e) => { e.stopPropagation(); onRemove(); }}
        title={removeLabel}
        className="absolute right-1.5 top-1.5 z-10 rounded p-1 text-muted-foreground opacity-0 transition-opacity hover:bg-destructive/10 hover:text-destructive group-hover:opacity-100"
      >
        <Trash2 className="h-3.5 w-3.5" />
      </button>
      <button onClick={onSelect} className="flex flex-1 flex-col text-left">
        <MiniPreview
          bg={ct.previewColors.background}
          fg={ct.previewColors.foreground}
          accent={ct.previewColors.primary}
          border={ct.previewColors.border}
        />
        <div className="flex items-center gap-1.5">
          {active && <Check className="h-3.5 w-3.5 text-primary" />}
          <span className="truncate text-sm font-medium">{ct.name}</span>
          <TypeIcon type={ct.type} />
        </div>
      </button>
    </div>
  );
}

function UserThemeEntryCard({
  entry,
  active,
  onSelect,
  onRemove,
  removeLabel,
}: {
  entry: UserThemeEntry;
  active: boolean;
  onSelect: () => void;
  onRemove: () => void;
  removeLabel: string;
}) {
  return (
    <div
      className={cn(
        "group relative flex flex-col rounded-lg border-2 p-3 text-left transition-all",
        active ? "border-primary shadow-sm" : "border-border hover:border-primary/40",
      )}
    >
      <button
        onClick={(e) => { e.stopPropagation(); onRemove(); }}
        title={removeLabel}
        className="absolute right-1.5 top-1.5 z-10 rounded p-1 text-muted-foreground opacity-0 transition-opacity hover:bg-destructive/10 hover:text-destructive group-hover:opacity-100"
      >
        <Trash2 className="h-3.5 w-3.5" />
      </button>
      <button onClick={onSelect} className="flex flex-1 flex-col text-left">
        <BundledPreviewPlaceholder type={entry.type} />
        <div className="flex items-center gap-1.5">
          {active && <Check className="h-3.5 w-3.5 text-primary" />}
          <span className="truncate text-sm font-medium">{entry.name}</span>
          <Download className="ml-auto h-3 w-3 text-muted-foreground opacity-50" />
        </div>
      </button>
    </div>
  );
}

// ── Shared helpers ──

function MiniPreview({ bg, fg, accent, border }: { bg: string; fg: string; accent: string; border: string }) {
  return (
    <div
      className="mb-2.5 flex h-16 w-full items-end gap-1 overflow-hidden rounded-md border p-1.5"
      style={{ backgroundColor: bg, borderColor: border }}
    >
      <div className="h-full w-3 rounded-sm" style={{ backgroundColor: accent, opacity: 0.7 }} />
      <div className="flex flex-1 flex-col gap-1 py-1">
        <div className="h-1.5 w-3/4 rounded-full" style={{ backgroundColor: fg, opacity: 0.5 }} />
        <div className="h-1.5 w-1/2 rounded-full" style={{ backgroundColor: fg, opacity: 0.3 }} />
        <div className="h-1.5 w-2/3 rounded-full" style={{ backgroundColor: accent, opacity: 0.6 }} />
      </div>
    </div>
  );
}

function BundledPreviewPlaceholder({ type }: { type: "dark" | "light" }) {
  const bg = type === "dark" ? "#1e1e1e" : "#fafafa";
  const fg = type === "dark" ? "#cccccc" : "#333333";
  const accent = type === "dark" ? "#569cd6" : "#0066cc";
  const border = type === "dark" ? "#333333" : "#e0e0e0";
  return <MiniPreview bg={bg} fg={fg} accent={accent} border={border} />;
}

function TypeIcon({ type }: { type: "dark" | "light" }) {
  return type === "dark" ? (
    <Moon className="ml-auto h-3 w-3 text-muted-foreground" />
  ) : (
    <Sun className="ml-auto h-3 w-3 text-muted-foreground" />
  );
}

export default SettingsPage;
