/**
 * VSCode theme file parser.
 * Converts VSCode color theme JSON into CSS variable overrides
 * compatible with our shadcn/ui-based theme system.
 */

// ── Types ──

export interface VscodeThemeFile {
  name?: string;
  type?: "dark" | "light";
  colors?: Record<string, string>;
  tokenColors?: unknown[];
}

export interface ParsedVscodeTheme {
  id: string;
  name: string;
  type: "dark" | "light";
  /** CSS variable name → HSL value string (without `hsl()` wrapper) */
  cssVars: Record<string, string>;
  /** Raw hex colors for preview swatches */
  previewColors: {
    background: string;
    foreground: string;
    primary: string;
    accent: string;
    border: string;
  };
}

// ── Hex → HSL conversion ──

function hexToRgb(hex: string): [number, number, number] | null {
  const h = hex.replace("#", "");
  if (h.length === 3) {
    return [
      parseInt(h[0] + h[0], 16),
      parseInt(h[1] + h[1], 16),
      parseInt(h[2] + h[2], 16),
    ];
  }
  if (h.length === 6 || h.length === 8) {
    return [
      parseInt(h.slice(0, 2), 16),
      parseInt(h.slice(2, 4), 16),
      parseInt(h.slice(4, 6), 16),
    ];
  }
  return null;
}

function rgbToHsl(r: number, g: number, b: number): [number, number, number] {
  r /= 255;
  g /= 255;
  b /= 255;
  const max = Math.max(r, g, b);
  const min = Math.min(r, g, b);
  const l = (max + min) / 2;

  if (max === min) return [0, 0, l * 100];

  const d = max - min;
  const s = l > 0.5 ? d / (2 - max - min) : d / (max + min);

  let h = 0;
  if (max === r) h = ((g - b) / d + (g < b ? 6 : 0)) / 6;
  else if (max === g) h = ((b - r) / d + 2) / 6;
  else h = ((r - g) / d + 4) / 6;

  return [h * 360, s * 100, l * 100];
}

/** Convert hex color to HSL string for CSS variable (e.g. "220 14% 10%") */
function hexToHslVar(hex: string): string | null {
  const rgb = hexToRgb(hex);
  if (!rgb) return null;
  const [h, s, l] = rgbToHsl(...rgb);
  return `${Math.round(h * 10) / 10} ${Math.round(s * 10) / 10}% ${Math.round(l * 10) / 10}%`;
}

/** Lighten/darken an HSL var string by adjusting lightness */
function adjustLightness(hslVar: string, delta: number): string {
  const parts = hslVar.split(/\s+/);
  if (parts.length < 3) return hslVar;
  const h = parts[0];
  const s = parts[1];
  const l = parseFloat(parts[2]);
  const newL = Math.max(0, Math.min(100, l + delta));
  return `${h} ${s} ${Math.round(newL * 10) / 10}%`;
}

// ── VSCode color key → CSS variable mapping ──

const MAPPING: Array<{
  cssVar: string;
  keys: string[];
  /** fallback: derive from another CSS var */
  fallbackVar?: string;
  fallbackAdjust?: number;
}> = [
  { cssVar: "--background", keys: ["editor.background"] },
  { cssVar: "--foreground", keys: ["editor.foreground", "foreground"] },
  { cssVar: "--card", keys: ["editorWidget.background", "editor.background"] },
  { cssVar: "--card-foreground", keys: ["editorWidget.foreground", "editor.foreground"] },
  { cssVar: "--popover", keys: ["editorWidget.background", "dropdown.background"] },
  { cssVar: "--popover-foreground", keys: ["editorWidget.foreground", "dropdown.foreground"] },
  { cssVar: "--primary", keys: ["button.background", "focusBorder"] },
  { cssVar: "--primary-foreground", keys: ["button.foreground"] },
  { cssVar: "--secondary", keys: ["button.secondaryBackground", "sideBar.background"] },
  { cssVar: "--secondary-foreground", keys: ["button.secondaryForeground", "sideBar.foreground"] },
  { cssVar: "--muted", keys: ["input.background", "editorGroupHeader.tabsBackground"] },
  { cssVar: "--muted-foreground", keys: ["descriptionForeground", "editorLineNumber.foreground"] },
  { cssVar: "--accent", keys: ["list.activeSelectionBackground", "list.hoverBackground"] },
  { cssVar: "--accent-foreground", keys: ["list.activeSelectionForeground", "foreground"] },
  { cssVar: "--destructive", keys: ["errorForeground", "editorError.foreground"] },
  { cssVar: "--destructive-foreground", keys: ["button.foreground"] },
  { cssVar: "--border", keys: ["panel.border", "editorGroup.border", "sideBar.border"] },
  { cssVar: "--input", keys: ["input.border", "input.background"] },
  { cssVar: "--ring", keys: ["focusBorder"] },
];

// ── Parser ──

export function parseVscodeTheme(raw: string, fileName: string): ParsedVscodeTheme {
  let json: VscodeThemeFile;
  try {
    json = JSON.parse(raw) as VscodeThemeFile;
  } catch {
    throw new Error("Invalid JSON file");
  }

  const colors = json.colors ?? {};
  if (Object.keys(colors).length === 0) {
    throw new Error("No 'colors' field found in theme file");
  }

  // Detect theme type
  const type: "dark" | "light" = json.type === "light" ? "light" : detectThemeType(colors);

  // Build CSS variables
  const cssVars: Record<string, string> = {};

  for (const mapping of MAPPING) {
    let resolved: string | null = null;
    for (const key of mapping.keys) {
      if (colors[key]) {
        resolved = hexToHslVar(colors[key]);
        if (resolved) break;
      }
    }
    if (resolved) {
      cssVars[mapping.cssVar] = resolved;
    }
  }

  // Fallback: derive card from background with slight adjustment
  if (!cssVars["--card"] && cssVars["--background"]) {
    cssVars["--card"] = adjustLightness(cssVars["--background"], type === "dark" ? 3 : -2);
  }
  if (!cssVars["--card-foreground"] && cssVars["--foreground"]) {
    cssVars["--card-foreground"] = cssVars["--foreground"];
  }
  if (!cssVars["--popover"] && cssVars["--card"]) {
    cssVars["--popover"] = cssVars["--card"];
  }
  if (!cssVars["--popover-foreground"] && cssVars["--foreground"]) {
    cssVars["--popover-foreground"] = cssVars["--foreground"];
  }
  if (!cssVars["--muted"] && cssVars["--background"]) {
    cssVars["--muted"] = adjustLightness(cssVars["--background"], type === "dark" ? 5 : -4);
  }
  if (!cssVars["--muted-foreground"] && cssVars["--foreground"]) {
    cssVars["--muted-foreground"] = adjustLightness(cssVars["--foreground"], type === "dark" ? -20 : 20);
  }
  if (!cssVars["--accent"] && cssVars["--background"]) {
    cssVars["--accent"] = adjustLightness(cssVars["--background"], type === "dark" ? 8 : -6);
  }
  if (!cssVars["--accent-foreground"] && cssVars["--foreground"]) {
    cssVars["--accent-foreground"] = cssVars["--foreground"];
  }
  if (!cssVars["--destructive"]) {
    cssVars["--destructive"] = "0 84.2% 60.2%";
  }
  if (!cssVars["--destructive-foreground"]) {
    cssVars["--destructive-foreground"] = "210 40% 98%";
  }
  if (!cssVars["--primary-foreground"]) {
    cssVars["--primary-foreground"] = type === "dark" ? "0 0% 100%" : "210 40% 98%";
  }
  if (!cssVars["--secondary-foreground"] && cssVars["--foreground"]) {
    cssVars["--secondary-foreground"] = cssVars["--foreground"];
  }
  if (!cssVars["--border"] && cssVars["--background"]) {
    cssVars["--border"] = adjustLightness(cssVars["--background"], type === "dark" ? 10 : -8);
  }
  if (!cssVars["--input"] && cssVars["--border"]) {
    cssVars["--input"] = cssVars["--border"];
  }
  if (!cssVars["--ring"] && cssVars["--primary"]) {
    cssVars["--ring"] = cssVars["--primary"];
  }

  // Also generate accent color vars used by sidebar
  const accentHex = colors["focusBorder"] ?? colors["button.background"] ?? colors["activityBarBadge.background"];
  if (accentHex) {
    cssVars["--theme-accent-raw"] = accentHex;
  }

  const name = json.name ?? fileName.replace(/\.json$/i, "");
  const id = "vsc-" + name.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/-+$/, "");

  return {
    id,
    name,
    type,
    cssVars,
    previewColors: {
      background: colors["editor.background"] ?? (type === "dark" ? "#1e1e1e" : "#ffffff"),
      foreground: colors["editor.foreground"] ?? (type === "dark" ? "#d4d4d4" : "#333333"),
      primary: colors["button.background"] ?? colors["focusBorder"] ?? "#007acc",
      accent: colors["activityBarBadge.background"] ?? colors["badge.background"] ?? "#007acc",
      border: colors["panel.border"] ?? colors["editorGroup.border"] ?? "#444444",
    },
  };
}

/** Detect dark/light by checking editor.background luminance */
function detectThemeType(colors: Record<string, string>): "dark" | "light" {
  const bg = colors["editor.background"];
  if (!bg) return "dark";
  const rgb = hexToRgb(bg);
  if (!rgb) return "dark";
  const luminance = (0.299 * rgb[0] + 0.587 * rgb[1] + 0.114 * rgb[2]) / 255;
  return luminance > 0.5 ? "light" : "dark";
}

/** Apply a parsed VSCode theme to the document */
export function applyVscodeTheme(theme: ParsedVscodeTheme): void {
  const root = document.documentElement;

  // Set CSS variables
  for (const [varName, value] of Object.entries(theme.cssVars)) {
    if (varName === "--theme-accent-raw") {
      root.style.setProperty("--theme-accent", value);
      // Darken for --theme-accent-dark
      const rgb = hexToRgb(value);
      if (rgb) {
        const [h, s, l] = rgbToHsl(...rgb);
        const darkL = Math.max(0, l - 10);
        const darkHex = `hsl(${Math.round(h)}, ${Math.round(s)}%, ${Math.round(darkL)}%)`;
        root.style.setProperty("--theme-accent-dark", darkHex);
      }
    } else {
      root.style.setProperty(varName, value);
    }
  }

  // Set color-scheme for scrollbars / system UI
  root.style.colorScheme = theme.type;

  // Set body background for theme type
  if (theme.cssVars["--background"]) {
    document.body.style.background = `hsl(${theme.cssVars["--background"]})`;
    root.style.color = theme.cssVars["--foreground"]
      ? `hsl(${theme.cssVars["--foreground"]})`
      : "";
  }
}

/** Remove all custom theme CSS variable overrides */
export function clearVscodeTheme(): void {
  const root = document.documentElement;
  for (const mapping of MAPPING) {
    root.style.removeProperty(mapping.cssVar);
  }
  root.style.removeProperty("--theme-accent");
  root.style.removeProperty("--theme-accent-dark");
  root.style.colorScheme = "light";
  document.body.style.background = "#ffffff";
  root.style.color = "";
}
