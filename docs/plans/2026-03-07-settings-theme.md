# Settings & Theme Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a settings popover (⚙️ in header) that lets users pick accent color theme and chat font size, persisted to localStorage.

**Architecture:** Zustand settingsStore with manual localStorage persistence → CSS custom properties on `<html>` element synced via store subscribe → SettingsPanel popover component wired into App header. Only accent color + font-chat variable are introduced; all other styling stays untouched.

**Tech Stack:** React 18, Zustand, Tailwind CSS, Vitest + @testing-library/react

---

### Task 1: settingsStore — test then implement

**Files:**
- Create: `web/src/stores/settingsStore.test.ts`
- Create: `web/src/stores/settingsStore.ts`

**Step 1: Write the failing test**

Create `web/src/stores/settingsStore.test.ts`:

```ts
/** @vitest-environment jsdom */
import { describe, it, expect, beforeEach } from "vitest";

// Reset module between tests so store is fresh
const makeStore = async () => {
  const mod = await import("./settingsStore");
  return mod.useSettingsStore;
};

describe("settingsStore", () => {
  beforeEach(() => {
    localStorage.clear();
    // Reset module registry so initial state re-reads localStorage
    vi.resetModules();
  });

  it("defaults to slate theme and md font size", async () => {
    const { vi } = await import("vitest");
    const store = (await makeStore()).getState();
    expect(store.theme).toBe("slate");
    expect(store.fontSize).toBe("md");
  });

  it("setTheme updates theme and persists to localStorage", async () => {
    const useStore = await makeStore();
    useStore.getState().setTheme("ocean");
    expect(useStore.getState().theme).toBe("ocean");
    const saved = JSON.parse(localStorage.getItem("ai-workflow-settings") ?? "{}");
    expect(saved.theme).toBe("ocean");
  });

  it("setFontSize updates fontSize and persists to localStorage", async () => {
    const useStore = await makeStore();
    useStore.getState().setFontSize("lg");
    expect(useStore.getState().fontSize).toBe("lg");
    const saved = JSON.parse(localStorage.getItem("ai-workflow-settings") ?? "{}");
    expect(saved.fontSize).toBe("lg");
  });

  it("reads persisted values from localStorage on init", async () => {
    localStorage.setItem("ai-workflow-settings", JSON.stringify({ theme: "amber", fontSize: "sm" }));
    const useStore = await makeStore();
    expect(useStore.getState().theme).toBe("amber");
    expect(useStore.getState().fontSize).toBe("sm");
  });
});
```

**Step 2: Run test to verify it fails**

```bash
cd D:/project/ai-workflow/web && npx vitest run src/stores/settingsStore.test.ts
```

Expected: FAIL — module not found.

**Step 3: Implement settingsStore**

Create `web/src/stores/settingsStore.ts`:

```ts
import { create } from "zustand";

export type Theme = "slate" | "ocean" | "forest" | "amber";
export type FontSize = "sm" | "md" | "lg";

const STORAGE_KEY = "ai-workflow-settings";

const loadFromStorage = (): { theme: Theme; fontSize: FontSize } => {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return { theme: "slate", fontSize: "md" };
    const parsed = JSON.parse(raw) as Partial<{ theme: Theme; fontSize: FontSize }>;
    return {
      theme: (["slate", "ocean", "forest", "amber"] as Theme[]).includes(parsed.theme as Theme)
        ? (parsed.theme as Theme)
        : "slate",
      fontSize: (["sm", "md", "lg"] as FontSize[]).includes(parsed.fontSize as FontSize)
        ? (parsed.fontSize as FontSize)
        : "md",
    };
  } catch {
    return { theme: "slate", fontSize: "md" };
  }
};

const saveToStorage = (theme: Theme, fontSize: FontSize) => {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ theme, fontSize }));
  } catch {
    // ignore
  }
};

interface SettingsState {
  theme: Theme;
  fontSize: FontSize;
  setTheme: (theme: Theme) => void;
  setFontSize: (size: FontSize) => void;
}

export const useSettingsStore = create<SettingsState>((set, get) => ({
  ...loadFromStorage(),
  setTheme: (theme) => {
    set({ theme });
    saveToStorage(theme, get().fontSize);
  },
  setFontSize: (fontSize) => {
    set({ fontSize });
    saveToStorage(get().theme, fontSize);
  },
}));
```

**Step 4: Run tests — expect pass**

```bash
cd D:/project/ai-workflow/web && npx vitest run src/stores/settingsStore.test.ts
```

Expected: all 4 tests pass (note: the `vi.resetModules()` approach may not fully isolate; if tests interfere, simplify to just testing state mutations without cross-test localStorage isolation).

**Step 5: Commit**

```bash
cd D:/project/ai-workflow && git add web/src/stores/settingsStore.ts web/src/stores/settingsStore.test.ts && git commit -m "feat(web): add settingsStore with localStorage persistence"
```

---

### Task 2: CSS variables in index.css

**Files:**
- Modify: `web/src/index.css`

No new test needed — purely declarative CSS verified visually.

**Step 1: Append CSS variables to index.css**

After the existing body rule, add:

```css
/* ── Settings: accent color themes ── */
:root {
  --accent: #475569;      /* slate-600 */
  --accent-dark: #0f172a; /* slate-900 */
  --font-chat: 0.9375rem; /* 15px — MD default */
}

[data-theme="ocean"] {
  --accent: #0284c7;
  --accent-dark: #0369a1;
}
[data-theme="forest"] {
  --accent: #059669;
  --accent-dark: #047857;
}
[data-theme="amber"] {
  --accent: #d97706;
  --accent-dark: #b45309;
}

[data-font-size="sm"] { --font-chat: 0.8125rem; } /* 13px */
[data-font-size="lg"] { --font-chat: 1.0625rem; } /* 17px */

/* Utility classes using CSS variables */
.accent-bg   { background-color: var(--accent-dark); color: #fff; }
.accent-text { color: var(--accent); }
.accent-border { border-color: var(--accent); }
```

**Step 2: Commit**

```bash
cd D:/project/ai-workflow && git add web/src/index.css && git commit -m "feat(web): add CSS variable theme system"
```

---

### Task 3: Sync store to HTML data attributes in main.tsx

**Files:**
- Modify: `web/src/main.tsx`

**Step 1: Update main.tsx**

```tsx
import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import "./index.css";
import { useSettingsStore } from "./stores/settingsStore";

// Sync settings store → <html> data attributes immediately and on every change
const applySettings = (state: { theme: string; fontSize: string }) => {
  document.documentElement.dataset.theme = state.theme;
  document.documentElement.dataset.fontSize = state.fontSize;
};

applySettings(useSettingsStore.getState());
useSettingsStore.subscribe(applySettings);

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
```

**Step 2: Commit**

```bash
cd D:/project/ai-workflow && git add web/src/main.tsx && git commit -m "feat(web): sync settingsStore to html data-theme/data-font-size"
```

---

### Task 4: SettingsPanel component — test then implement

**Files:**
- Create: `web/src/components/SettingsPanel.test.tsx`
- Create: `web/src/components/SettingsPanel.tsx`

**Step 1: Write the failing test**

Create `web/src/components/SettingsPanel.test.tsx`:

```tsx
/** @vitest-environment jsdom */
import { describe, it, expect, afterEach, vi } from "vitest";
import { render, screen, fireEvent, cleanup } from "@testing-library/react";
import { SettingsPanel } from "./SettingsPanel";
import { useSettingsStore } from "../stores/settingsStore";

describe("SettingsPanel", () => {
  afterEach(() => {
    cleanup();
    localStorage.clear();
    useSettingsStore.setState({ theme: "slate", fontSize: "md" });
  });

  it("renders theme swatches and font size buttons", () => {
    render(<SettingsPanel open onClose={() => {}} />);
    expect(screen.getByTitle("slate")).toBeTruthy();
    expect(screen.getByTitle("ocean")).toBeTruthy();
    expect(screen.getByTitle("forest")).toBeTruthy();
    expect(screen.getByTitle("amber")).toBeTruthy();
    expect(screen.getByText("S")).toBeTruthy();
    expect(screen.getByText("M")).toBeTruthy();
    expect(screen.getByText("L")).toBeTruthy();
  });

  it("clicking a theme swatch updates the store", () => {
    render(<SettingsPanel open onClose={() => {}} />);
    fireEvent.click(screen.getByTitle("ocean"));
    expect(useSettingsStore.getState().theme).toBe("ocean");
  });

  it("clicking a font size button updates the store", () => {
    render(<SettingsPanel open onClose={() => {}} />);
    fireEvent.click(screen.getByText("L"));
    expect(useSettingsStore.getState().fontSize).toBe("lg");
  });

  it("calls onClose when close button clicked", () => {
    const onClose = vi.fn();
    render(<SettingsPanel open onClose={onClose} />);
    fireEvent.click(screen.getByLabelText("关闭设置"));
    expect(onClose).toHaveBeenCalled();
  });

  it("renders nothing when open=false", () => {
    const { container } = render(<SettingsPanel open={false} onClose={() => {}} />);
    expect(container.firstChild).toBeNull();
  });
});
```

**Step 2: Run to verify it fails**

```bash
cd D:/project/ai-workflow/web && npx vitest run src/components/SettingsPanel.test.tsx
```

Expected: FAIL — module not found.

**Step 3: Implement SettingsPanel**

Create `web/src/components/SettingsPanel.tsx`:

```tsx
import { useRef, useEffect, useCallback } from "react";
import { useSettingsStore, type Theme, type FontSize } from "../stores/settingsStore";

const THEMES: { id: Theme; label: string; color: string }[] = [
  { id: "slate",  label: "Slate",  color: "#475569" },
  { id: "ocean",  label: "Ocean",  color: "#0284c7" },
  { id: "forest", label: "Forest", color: "#059669" },
  { id: "amber",  label: "Amber",  color: "#d97706" },
];

const FONT_SIZES: { id: FontSize; label: string }[] = [
  { id: "sm", label: "S" },
  { id: "md", label: "M" },
  { id: "lg", label: "L" },
];

interface SettingsPanelProps {
  open: boolean;
  onClose: () => void;
}

export function SettingsPanel({ open, onClose }: SettingsPanelProps) {
  const theme = useSettingsStore((s) => s.theme);
  const fontSize = useSettingsStore((s) => s.fontSize);
  const setTheme = useSettingsStore((s) => s.setTheme);
  const setFontSize = useSettingsStore((s) => s.setFontSize);
  const panelRef = useRef<HTMLDivElement>(null);

  const handleOutsideClick = useCallback(
    (e: MouseEvent) => {
      if (panelRef.current && !panelRef.current.contains(e.target as Node)) {
        onClose();
      }
    },
    [onClose],
  );

  useEffect(() => {
    if (!open) return;
    document.addEventListener("mousedown", handleOutsideClick);
    return () => document.removeEventListener("mousedown", handleOutsideClick);
  }, [open, handleOutsideClick]);

  if (!open) return null;

  return (
    <div
      ref={panelRef}
      className="absolute right-0 top-full z-50 mt-2 w-72 rounded-xl border border-slate-200 bg-white p-4 shadow-xl"
    >
      <div className="flex items-center justify-between mb-3">
        <span className="text-sm font-semibold text-slate-700">外观设置</span>
        <button
          type="button"
          aria-label="关闭设置"
          onClick={onClose}
          className="text-slate-400 hover:text-slate-600"
        >
          ✕
        </button>
      </div>

      {/* Theme swatches */}
      <p className="mb-2 text-xs font-medium text-slate-500">主题颜色</p>
      <div className="mb-4 flex gap-3">
        {THEMES.map((t) => (
          <button
            key={t.id}
            type="button"
            title={t.id}
            aria-label={t.label}
            onClick={() => setTheme(t.id)}
            className="flex flex-col items-center gap-1"
          >
            <span
              className="block h-7 w-7 rounded-full border-2 transition-transform hover:scale-110"
              style={{
                backgroundColor: t.color,
                borderColor: theme === t.id ? t.color : "transparent",
                outline: theme === t.id ? `2px solid ${t.color}` : "2px solid transparent",
                outlineOffset: "2px",
              }}
            />
            <span className="text-[10px] text-slate-500">{t.label}</span>
          </button>
        ))}
      </div>

      {/* Font size */}
      <p className="mb-2 text-xs font-medium text-slate-500">聊天字号</p>
      <div className="flex gap-2">
        {FONT_SIZES.map((f) => (
          <button
            key={f.id}
            type="button"
            onClick={() => setFontSize(f.id)}
            className={`flex-1 rounded-lg border py-1.5 text-sm font-semibold transition ${
              fontSize === f.id
                ? "accent-bg border-transparent"
                : "border-slate-200 text-slate-600 hover:bg-slate-50"
            }`}
          >
            {f.label}
          </button>
        ))}
      </div>
    </div>
  );
}
```

**Step 4: Run tests — expect pass**

```bash
cd D:/project/ai-workflow/web && npx vitest run src/components/SettingsPanel.test.tsx
```

Expected: 5/5 pass.

**Step 5: Commit**

```bash
cd D:/project/ai-workflow && git add web/src/components/SettingsPanel.tsx web/src/components/SettingsPanel.test.tsx && git commit -m "feat(web): add SettingsPanel component with theme and font size selection"
```

---

### Task 5: Wire SettingsPanel into App.tsx header + update nav active tab

**Files:**
- Modify: `web/src/App.tsx`

**Step 1: Update App.tsx**

In `App.tsx`:

1. Import `SettingsPanel` and `useState`:
   ```tsx
   import { SettingsPanel } from "./components/SettingsPanel";
   // useState already imported
   ```

2. Add state inside `App` component (after existing useState calls):
   ```tsx
   const [settingsOpen, setSettingsOpen] = useState(false);
   ```

3. In the header `<div className="flex items-center gap-2">` (the right side with project select), **append** a settings trigger button before the closing `</div>`:
   ```tsx
   <div className="relative">
     <button
       type="button"
       title="外观设置"
       aria-label="外观设置"
       onClick={() => setSettingsOpen((v) => !v)}
       className="rounded-md border border-slate-300 bg-white px-3 py-2 text-sm hover:bg-slate-50"
     >
       ⚙️
     </button>
     <SettingsPanel open={settingsOpen} onClose={() => setSettingsOpen(false)} />
   </div>
   ```

4. Update the nav active tab class from `bg-slate-900 text-white` to `accent-bg`:
   ```tsx
   // Before:
   activeView === view
     ? "bg-slate-900 text-white"
     : "bg-slate-100 text-slate-700 hover:bg-slate-200"
   // After:
   activeView === view
     ? "accent-bg"
     : "bg-slate-100 text-slate-700 hover:bg-slate-200"
   ```

**Step 2: Run the App.test.tsx to make sure nothing broke**

```bash
cd D:/project/ai-workflow/web && npx vitest run src/App.test.tsx
```

If the test checks for `bg-slate-900` on active nav button, update it to check for `accent-bg` instead.

**Step 3: Commit**

```bash
cd D:/project/ai-workflow && git add web/src/App.tsx && git commit -m "feat(web): wire SettingsPanel into App header and update nav active tab"
```

---

### Task 6: Apply --font-chat to chat messages

**Files:**
- Modify: `web/src/components/TuiMessage.tsx`

**Step 1: Update TuiMessage.tsx**

For both the user and assistant message `<p>` / content `<div>`, add `style={{ fontSize: "var(--font-chat)" }}`:

In the **user** branch (around line 56), change:
```tsx
// Before:
<p className="mt-1 text-sm font-medium whitespace-pre-wrap">{content}</p>
// After:
<p className="mt-1 font-medium whitespace-pre-wrap" style={{ fontSize: "var(--font-chat)" }}>{content}</p>
```

In the **assistant** branch (around line 67), change:
```tsx
// Before:
<div className="min-w-0 flex-1 text-sm">
// After:
<div className="min-w-0 flex-1" style={{ fontSize: "var(--font-chat)" }}>
```

**Step 2: Run TuiMessage tests**

```bash
cd D:/project/ai-workflow/web && npx vitest run src/components/TuiMessage.test.tsx
```

Expected: all pass (tests check text content, not font size class).

**Step 3: Commit**

```bash
cd D:/project/ai-workflow && git add web/src/components/TuiMessage.tsx && git commit -m "feat(web): apply --font-chat CSS variable to chat messages"
```

---

### Task 7: Apply accent color to TuiActivityBlock border + ChatView primary buttons

**Files:**
- Modify: `web/src/components/TuiActivityBlock.tsx`
- Modify: `web/src/views/ChatView.tsx`

**Step 1: Update TuiActivityBlock.tsx**

Change the wrapper div border class from `border-slate-200` to dynamically use accent when expanded:

```tsx
// In the return JSX, change:
<div className="ml-6 my-1 border-l-2 border-slate-200 px-3 py-1 text-sm">
// To:
<div className={`ml-6 my-1 border-l-2 px-3 py-1 text-sm ${expanded ? "accent-border" : "border-slate-200"}`}>
```

**Step 2: Update ChatView.tsx primary action buttons**

Find the send button (search for `aria-label="发送"` or the submit button in the input area) and the "创建 issue" button. Replace their `bg-slate-900` / `bg-sky-700` background classes with `accent-bg`:

For the **send button** (look for `type="submit"` near the bottom of ChatView):
```tsx
// Before (example): className="... bg-slate-900 text-white ..."
// After: className="... accent-bg ..."
```

For the **"从选中文件创建 issue" button** in the left tree panel:
```tsx
// Before: className="... border-sky-700 ... text-sky-700 ..."
// After: use accent-text and accent-border classes, or inline style={{ color: "var(--accent)", borderColor: "var(--accent)" }}
```

Note: Read the exact button classNames in ChatView.tsx before making changes to get the correct `old_string`.

**Step 3: Run all affected tests**

```bash
cd D:/project/ai-workflow/web && npx vitest run src/components/TuiActivityBlock.test.tsx src/views/ChatView.test.tsx
```

Fix any test failures (likely only if tests check specific Tailwind class names on buttons).

**Step 4: Commit**

```bash
cd D:/project/ai-workflow && git add web/src/components/TuiActivityBlock.tsx web/src/views/ChatView.tsx && git commit -m "feat(web): apply accent color to activity block border and primary buttons"
```

---

### Task 8: Full test run + final verification

**Step 1: Run all frontend tests**

```bash
cd D:/project/ai-workflow/web && npx vitest run
```

Expected: all tests pass.

**Step 2: Type check**

```bash
npm --prefix D:/project/ai-workflow/web run typecheck
```

Expected: no errors.

**Step 3: Quick manual smoke**

Start frontend dev server:
```bash
npm --prefix D:/project/ai-workflow/web run dev -- --strictPort
```

Open browser → click ⚙️ → switch theme → verify nav tab color changes → switch font size → verify chat text grows/shrinks → refresh page → verify settings persisted.

**Step 4: Final commit if any stragglers**

```bash
cd D:/project/ai-workflow && git status
# commit any remaining changes
```
