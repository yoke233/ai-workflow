# Settings & Theme Design

Date: 2026-03-07

## Goal

Allow users to configure chat font size and UI accent color via a lightweight settings panel. No backend changes required — preferences persist in localStorage.

## Scope

- 4 accent color themes (Slate/Ocean/Forest/Amber)
- 3 font size levels (SM/MD/LG)
- Settings persist across sessions via localStorage
- Settings UI: small popover from gear icon in App header

## Architecture

### settingsStore (`web/src/stores/settingsStore.ts`)

Zustand store with localStorage persistence:

```ts
interface SettingsState {
  theme: "slate" | "ocean" | "forest" | "amber";
  fontSize: "sm" | "md" | "lg";
  setTheme: (theme: ...) => void;
  setFontSize: (size: ...) => void;
}
// persisted via localStorage key "ai-workflow-settings"
```

### CSS Variables (`web/src/index.css`)

Three root variables:

```css
:root {
  --accent: #475569;       /* slate-600 */
  --accent-dark: #0f172a;  /* slate-900 */
  --font-chat: 0.9375rem;  /* 15px, MD default */
}
[data-theme="ocean"]  { --accent: #0284c7; --accent-dark: #0369a1; }
[data-theme="forest"] { --accent: #059669; --accent-dark: #047857; }
[data-theme="amber"]  { --accent: #d97706; --accent-dark: #b45309; }

[data-font-size="sm"] { --font-chat: 0.8125rem; } /* 13px */
[data-font-size="lg"] { --font-chat: 1.0625rem; } /* 17px */
```

Three utility classes consumed by components:

```css
.accent-bg   { background-color: var(--accent-dark); }
.accent-text { color: var(--accent); }
.accent-border { border-color: var(--accent); }
```

### DOM Sync (`web/src/main.tsx`)

A tiny effect syncs store state to `<html>` data attributes:

```ts
useSettingsStore.subscribe((state) => {
  document.documentElement.dataset.theme = state.theme;
  document.documentElement.dataset.fontSize = state.fontSize;
});
```

### SettingsPanel (`web/src/components/SettingsPanel.tsx`)

Popover triggered by ⚙️ button in App.tsx header. Contains:
- 4 color swatch circles (click to select theme)
- 3 font size toggle buttons: S / M / L
- Closes on outside click (mousedown listener)
- ~280px wide, positioned below/right of trigger button

## Components to Update

| File | Change |
|------|--------|
| `App.tsx` | Add ⚙️ button + `<SettingsPanel>` to header; nav active tab uses `accent-bg` class |
| `ChatView.tsx` | Primary buttons use `accent-bg`; send button uses `accent-bg` |
| `TuiMessage.tsx` | Message text uses `var(--font-chat)` via `style` prop |
| `TuiActivityBlock.tsx` | `border-l` color uses `accent-border` when expanded |

## Out of Scope

- Dark mode
- Full background/surface theme changes
- Server-side persistence
- Per-session settings
