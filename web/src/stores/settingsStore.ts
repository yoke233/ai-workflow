import { create } from "zustand";
import type { ParsedVscodeTheme } from "@/lib/vscodeTheme";
import { parseVscodeTheme } from "@/lib/vscodeTheme";
import {
  listUserThemes,
  getUserTheme as fetchUserThemeJson,
  saveUserTheme as apiSaveTheme,
  deleteUserTheme as apiDeleteTheme,
} from "@/lib/themeApi";

export type BuiltinTheme = "slate" | "ocean" | "forest" | "amber";
export type FontSize = "sm" | "md" | "lg";

const BUILTIN_THEMES: BuiltinTheme[] = ["slate", "ocean", "forest", "amber"];
const STORAGE_KEY = "ai-workflow-settings";

// ── Theme data (shared by user & bundled themes once parsed) ──

export interface StoredVscodeTheme {
  id: string;
  name: string;
  type: "dark" | "light";
  cssVars: Record<string, string>;
  previewColors: {
    background: string;
    foreground: string;
    primary: string;
    accent: string;
    border: string;
  };
}

/** Entry from /themes/manifest.json (bundled in the binary) */
export interface BundledThemeEntry {
  id: string;
  name: string;
  type: "dark" | "light";
  folder: string;
  description: string;
}

/** Lightweight entry returned by GET /api/themes (user-saved on disk) */
export interface UserThemeEntry {
  id: string;
  name: string;
  type: "dark" | "light";
  folder: string;
}

// ── Settings persistence (localStorage — only theme id + font size) ──

const loadFromStorage = (): { theme: string; fontSize: FontSize } => {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return { theme: "slate", fontSize: "md" };
    const parsed = JSON.parse(raw) as Record<string, unknown>;
    const theme = typeof parsed.theme === "string" ? parsed.theme : "slate";
    const fontSize = parsed.fontSize as string;
    return {
      theme,
      fontSize: (["sm", "md", "lg"] as FontSize[]).includes(fontSize as FontSize)
        ? (fontSize as FontSize)
        : "md",
    };
  } catch {
    return { theme: "slate", fontSize: "md" };
  }
};

const saveToStorage = (theme: string, fontSize: FontSize) => {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ theme, fontSize }));
  } catch {
    // ignore
  }
};

// ── Store ──

interface SettingsState {
  theme: string;
  fontSize: FontSize;

  /** User-imported themes: lightweight entries from backend */
  userThemeEntries: UserThemeEntry[];
  /** Parsed & cached user themes (loaded on demand) */
  userThemeCache: Record<string, StoredVscodeTheme>;

  /** Bundled themes from /themes/manifest.json */
  bundledThemes: BundledThemeEntry[];
  /** Parsed cache for bundled themes (loaded on demand) */
  bundledThemeCache: Record<string, StoredVscodeTheme>;
  bundledLoading: boolean;

  setTheme: (theme: string) => void;
  setFontSize: (size: FontSize) => void;

  /** Import a parsed VSCode theme → save to backend + activate */
  addCustomTheme: (theme: ParsedVscodeTheme, rawJson: string) => Promise<void>;
  /** Remove a user theme from backend */
  removeCustomTheme: (id: string) => Promise<void>;

  isBuiltinTheme: (id: string) => boolean;
  getActiveCustomTheme: () => StoredVscodeTheme | null;

  /** Load user theme list from GET /api/themes */
  loadUserThemes: () => Promise<void>;
  /** Fetch + parse a user theme by id, cache it, activate it */
  activateUserTheme: (id: string) => Promise<void>;

  /** Load bundled theme manifest from /themes/manifest.json */
  loadBundledManifest: () => Promise<void>;
  /** Fetch + parse a bundled theme by id, cache it, activate it */
  activateBundledTheme: (id: string) => Promise<void>;
}

const initial = loadFromStorage();

export const useSettingsStore = create<SettingsState>((set, get) => ({
  theme: initial.theme,
  fontSize: initial.fontSize,
  userThemeEntries: [],
  userThemeCache: {},
  bundledThemes: [],
  bundledThemeCache: {},
  bundledLoading: false,

  setTheme: (theme) => {
    set({ theme });
    saveToStorage(theme, get().fontSize);
  },

  setFontSize: (fontSize) => {
    set({ fontSize });
    saveToStorage(get().theme, fontSize);
  },

  addCustomTheme: async (parsed: ParsedVscodeTheme, rawJson: string) => {
    const stored: StoredVscodeTheme = {
      id: parsed.id,
      name: parsed.name,
      type: parsed.type,
      cssVars: parsed.cssVars,
      previewColors: parsed.previewColors,
    };

    // Save to backend disk
    await apiSaveTheme({
      id: parsed.id,
      name: parsed.name,
      type: parsed.type,
      data: JSON.parse(rawJson),
    });

    // Update local state
    const newEntries = [
      ...get().userThemeEntries.filter((e) => e.id !== stored.id),
      { id: stored.id, name: stored.name, type: stored.type, folder: stored.id },
    ];
    set({
      userThemeEntries: newEntries,
      userThemeCache: { ...get().userThemeCache, [stored.id]: stored },
      theme: stored.id,
    });
    saveToStorage(stored.id, get().fontSize);
  },

  removeCustomTheme: async (id: string) => {
    await apiDeleteTheme(id);
    const newEntries = get().userThemeEntries.filter((e) => e.id !== id);
    const newCache = { ...get().userThemeCache };
    delete newCache[id];
    set({ userThemeEntries: newEntries, userThemeCache: newCache });
    if (get().theme === id) {
      set({ theme: "slate" });
      saveToStorage("slate", get().fontSize);
    }
  },

  isBuiltinTheme: (id: string) => BUILTIN_THEMES.includes(id as BuiltinTheme),

  getActiveCustomTheme: () => {
    const { theme, userThemeCache, bundledThemeCache } = get();
    return userThemeCache[theme] ?? bundledThemeCache[theme] ?? null;
  },

  loadUserThemes: async () => {
    try {
      const items = await listUserThemes();
      set({
        userThemeEntries: items.map((it) => ({
          id: it.id,
          name: it.name,
          type: it.type,
          folder: it.folder,
        })),
      });
    } catch {
      // silent
    }
  },

  activateUserTheme: async (id: string) => {
    if (get().userThemeCache[id]) {
      set({ theme: id });
      saveToStorage(id, get().fontSize);
      return;
    }
    try {
      const raw = await fetchUserThemeJson(id);
      if (!raw) return;
      const parsed = parseVscodeTheme(raw, `${id}.json`);
      const stored: StoredVscodeTheme = {
        id,
        name: parsed.name,
        type: parsed.type,
        cssVars: parsed.cssVars,
        previewColors: parsed.previewColors,
      };
      set({
        userThemeCache: { ...get().userThemeCache, [id]: stored },
        theme: id,
      });
      saveToStorage(id, get().fontSize);
    } catch {
      // silent
    }
  },

  loadBundledManifest: async () => {
    if (get().bundledThemes.length > 0) return;
    set({ bundledLoading: true });
    try {
      const resp = await fetch("/themes/manifest.json");
      if (!resp.ok) return;
      const data = (await resp.json()) as { themes: BundledThemeEntry[] };
      set({ bundledThemes: data.themes ?? [] });
    } catch {
      // silent
    } finally {
      set({ bundledLoading: false });
    }
  },

  activateBundledTheme: async (id: string) => {
    if (get().bundledThemeCache[id]) {
      set({ theme: id });
      saveToStorage(id, get().fontSize);
      return;
    }
    const entry = get().bundledThemes.find((t) => t.id === id);
    if (!entry) return;
    try {
      const resp = await fetch(`/themes/${entry.folder}/theme.json`);
      if (!resp.ok) return;
      const text = await resp.text();
      const parsed = parseVscodeTheme(text, `${entry.folder}.json`);
      const stored: StoredVscodeTheme = {
        id: entry.id,
        name: parsed.name,
        type: parsed.type,
        cssVars: parsed.cssVars,
        previewColors: parsed.previewColors,
      };
      set({
        bundledThemeCache: { ...get().bundledThemeCache, [id]: stored },
        theme: id,
      });
      saveToStorage(id, get().fontSize);
    } catch {
      // silent
    }
  },
}));

// Re-export for backward compat
export type Theme = BuiltinTheme;
