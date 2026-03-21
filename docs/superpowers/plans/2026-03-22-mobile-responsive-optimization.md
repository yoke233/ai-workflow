# Mobile Responsive Optimization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Optimize the frontend for mobile devices — responsive sidebar, mobile-friendly chat, touch-optimized controls, and iOS input zoom fix.

**Architecture:** A `useIsMobile()` hook drives layout changes across the app. On mobile (<768px), the AppSidebar becomes an overlay drawer toggled by a hamburger button in a new mobile header. The ChatPage's session sidebar also converts to a drawer. All interactive targets get minimum 44px touch areas. iOS input zoom is prevented via viewport meta and `text-[16px]` on inputs.

**Tech Stack:** React 18, Tailwind CSS, React Router DOM v7, Zustand (existing), Lucide icons (existing)

---

## File Structure

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `web/src/hooks/useIsMobile.ts` | Media query hook: returns `true` when viewport < 768px |
| Modify | `web/index.html` | Add `maximum-scale=1` to viewport meta for iOS zoom prevention |
| Modify | `web/src/index.css` | Add mobile utility classes, safe-area insets |
| Modify | `web/src/layouts/AppLayout.tsx` | Mobile: hamburger header + drawer sidebar; Desktop: unchanged |
| Modify | `web/src/components/app-sidebar.tsx` | Accept `open`/`onClose` props for drawer mode; add overlay backdrop |
| Modify | `web/src/pages/ChatPage.tsx` | Mobile: hide ChatSessionSidebar, add toggle button; Desktop: unchanged |
| Modify | `web/src/components/chat/ChatSessionSidebar.tsx` | Accept `open`/`onClose` props for mobile drawer mode |
| Modify | `web/src/components/chat/ChatInputBar.tsx` | Touch-friendly sizing, `text-[16px]` to prevent iOS zoom |
| Modify | `web/src/pages/MobileHomePage.tsx` | Touch target sizes, input zoom prevention |
| Modify | `web/src/layouts/MonitoringLayout.tsx` | Mobile-friendly tab scrolling |
| Modify | `web/src/layouts/RuntimeLayout.tsx` | Mobile-friendly tab scrolling |

---

### Task 1: Create `useIsMobile` Hook

**Files:**
- Create: `web/src/hooks/useIsMobile.ts`

- [ ] **Step 1: Create the hook file**

```typescript
import { useEffect, useState } from "react";

const MOBILE_BREAKPOINT = 768;

export function useIsMobile(): boolean {
  const [isMobile, setIsMobile] = useState(
    () => typeof window !== "undefined" && window.innerWidth < MOBILE_BREAKPOINT,
  );

  useEffect(() => {
    const mql = window.matchMedia(`(max-width: ${MOBILE_BREAKPOINT - 1}px)`);
    const handler = (e: MediaQueryListEvent) => setIsMobile(e.matches);
    mql.addEventListener("change", handler);
    return () => mql.removeEventListener("change", handler);
  }, []);

  return isMobile;
}
```

- [ ] **Step 2: Verify the file compiles**

Run: `npm --prefix web run typecheck`
Expected: No errors related to `useIsMobile`

- [ ] **Step 3: Commit**

```bash
git add web/src/hooks/useIsMobile.ts
git commit -m "feat(web): add useIsMobile media query hook"
```

---

### Task 2: Viewport Meta & CSS Foundation

**Files:**
- Modify: `web/index.html:5`
- Modify: `web/src/index.css`

- [ ] **Step 1: Update viewport meta tag in `index.html`**

Change line 5 from:
```html
<meta name="viewport" content="width=device-width, initial-scale=1.0" />
```
to:
```html
<meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, viewport-fit=cover" />
```

This prevents iOS auto-zoom on input focus and enables safe-area support for notched devices.

- [ ] **Step 2: Add mobile CSS utilities to `index.css`**

Append at the end of `web/src/index.css`:

```css
/* ── Mobile: safe-area insets for notched devices ── */
@supports (padding-top: env(safe-area-inset-top)) {
  .safe-top    { padding-top: env(safe-area-inset-top); }
  .safe-bottom { padding-bottom: env(safe-area-inset-bottom); }
}

/* ── Mobile drawer overlay ── */
.drawer-overlay {
  position: fixed;
  inset: 0;
  z-index: 40;
  background: rgba(0, 0, 0, 0.4);
}

```

Note: Drawer animation (slide-in/out) is a future enhancement — the current implementation uses conditional rendering (mount/unmount) without transition animation for simplicity.

- [ ] **Step 3: Verify build**

Run: `npm --prefix web run typecheck`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add web/index.html web/src/index.css
git commit -m "feat(web): add mobile viewport meta and CSS utilities"
```

---

### Task 3: Make AppSidebar a Mobile Drawer

**Files:**
- Modify: `web/src/components/app-sidebar.tsx`
- Modify: `web/src/layouts/AppLayout.tsx`

- [ ] **Step 1: Add drawer props to `AppSidebar`**

In `web/src/components/app-sidebar.tsx`, update the component signature and add drawer support. The sidebar needs to accept optional `drawerOpen` and `onClose` props. When `drawerOpen` is provided, it renders as a fixed overlay drawer instead of a static sidebar.

Change the component definition from:

```typescript
export function AppSidebar() {
```

to:

```typescript
interface AppSidebarProps {
  drawerOpen?: boolean;
  onClose?: () => void;
}

export function AppSidebar({ drawerOpen, onClose }: AppSidebarProps = {}) {
```

- [ ] **Step 2: Wrap the sidebar JSX with drawer mode**

In `AppSidebar`, replace the outermost `<aside>` element. When `drawerOpen` is defined, render as a fixed overlay; otherwise render as the existing static sidebar.

Replace the current return:
```tsx
  return (
    <aside
      className={cn(
        "flex h-screen flex-col border-r bg-sidebar transition-[width] duration-200",
        collapsed ? "w-14" : "w-56",
      )}
    >
```

with:
```tsx
  const isDrawer = drawerOpen !== undefined;

  if (isDrawer && !drawerOpen) return null;

  const sidebarContent = (
    <aside
      className={cn(
        "flex h-screen flex-col border-r bg-sidebar",
        isDrawer ? "w-72" : "transition-[width] duration-200",
        !isDrawer && (collapsed ? "w-14" : "w-56"),
      )}
    >
```

And at the end of the component, before the final `);`, wrap the return:

Replace the closing:
```tsx
    </aside>
  );
}
```

with:
```tsx
    </aside>
  );

  if (isDrawer) {
    return (
      <div className="fixed inset-0 z-50 flex" onClick={onClose}>
        <div onClick={(e) => e.stopPropagation()}>
          {sidebarContent}
        </div>
        <div className="flex-1 drawer-overlay" />
      </div>
    );
  }

  return sidebarContent;
}
```

- [ ] **Step 3: Add auto-close on nav link click in drawer mode**

In `AppSidebar`, for each `<NavLink>` in the nav items map, add an `onClick` handler that calls `onClose` when in drawer mode:

Change:
```tsx
          <NavLink
            key={item.to}
            to={item.to}
            end={item.to === "/"}
            title={collapsed ? t(item.labelKey) : undefined}
            className={({ isActive }) =>
```
to:
```tsx
          <NavLink
            key={item.to}
            to={item.to}
            end={item.to === "/"}
            title={collapsed ? t(item.labelKey) : undefined}
            onClick={isDrawer ? onClose : undefined}
            className={({ isActive }) =>
```

Do the same for the Settings `<NavLink>`:

Add `onClick={isDrawer ? onClose : undefined}` to the Settings NavLink.

- [ ] **Step 4: Update AppLayout for mobile**

Replace `web/src/layouts/AppLayout.tsx` entirely:

```tsx
import { useState } from "react";
import { Outlet } from "react-router-dom";
import { Menu } from "lucide-react";
import { AppSidebar } from "@/components/app-sidebar";
import { useIsMobile } from "@/hooks/useIsMobile";

export function AppLayout() {
  const isMobile = useIsMobile();
  const [drawerOpen, setDrawerOpen] = useState(false);

  if (isMobile) {
    return (
      <div className="flex h-screen flex-col overflow-hidden">
        {/* Mobile top bar */}
        <header className="flex h-12 shrink-0 items-center gap-3 border-b bg-sidebar px-3">
          <button
            onClick={() => setDrawerOpen(true)}
            className="flex h-9 w-9 items-center justify-center rounded-md hover:bg-accent"
          >
            <Menu className="h-5 w-5" />
          </button>
          <span className="text-sm font-semibold">AI Workflow</span>
        </header>

        {/* Drawer sidebar */}
        <AppSidebar drawerOpen={drawerOpen} onClose={() => setDrawerOpen(false)} />

        {/* Page content */}
        <main className="flex-1 overflow-auto bg-background">
          <Outlet />
        </main>
      </div>
    );
  }

  return (
    <div className="flex h-screen overflow-hidden">
      <AppSidebar />
      <main className="flex-1 overflow-auto bg-background">
        <Outlet />
      </main>
    </div>
  );
}
```

- [ ] **Step 5: Verify build**

Run: `npm --prefix web run typecheck`
Expected: PASS

- [ ] **Step 6: Manual test**

Open browser, resize to < 768px width:
- Desktop: Sidebar visible as before (static, collapsible)
- Mobile: Hamburger header visible; tapping it opens sidebar as overlay drawer
- Tapping a nav link closes the drawer
- Tapping the dark overlay closes the drawer

- [ ] **Step 7: Commit**

```bash
git add web/src/components/app-sidebar.tsx web/src/layouts/AppLayout.tsx
git commit -m "feat(web): responsive AppSidebar — drawer on mobile, static on desktop"
```

---

### Task 4: Mobile Chat Page — Session Sidebar as Drawer

**Files:**
- Modify: `web/src/pages/ChatPage.tsx:1322-1340`
- Modify: `web/src/components/chat/ChatSessionSidebar.tsx:147`

- [ ] **Step 1: Add drawer props to ChatSessionSidebar**

In `web/src/components/chat/ChatSessionSidebar.tsx`, find the props interface and add:

```typescript
  drawerOpen?: boolean;
  onClose?: () => void;
```

- [ ] **Step 2: Wrap ChatSessionSidebar with drawer mode**

In `ChatSessionSidebar`, find the root element (line 147):
```tsx
    <div className="flex w-72 flex-col border-r bg-sidebar">
```

Add drawer logic similar to AppSidebar. When `drawerOpen` is defined and `false`, return `null`. When `true`, wrap in fixed overlay. When `undefined` (desktop), render as-is.

The component is wrapped in `memo()` with signature `function ChatSessionSidebar(props: ChatSessionSidebarProps)`. The drawer logic must reference `props.drawerOpen` and `props.onClose` since those are the new fields added to the interface.

**Before** the existing root `<div>` (line 147), insert the early return:

```tsx
    const isDrawer = props.drawerOpen !== undefined;
    if (isDrawer && !props.drawerOpen) return null;
```

**Wrap** the entire existing JSX return in a variable. Change the root from:
```tsx
    <div className="flex w-72 flex-col border-r bg-sidebar">
      ... existing content ...
    </div>
```
to:
```tsx
    const content = (
      <div className={cn("flex w-72 flex-col border-r bg-sidebar", isDrawer && "h-screen")}>
        ... existing content (unchanged) ...
      </div>
    );

    if (isDrawer) {
      return (
        <div className="fixed inset-0 z-50 flex" onClick={props.onClose}>
          <div onClick={(e) => e.stopPropagation()}>
            {content}
          </div>
          <div className="flex-1 drawer-overlay" />
        </div>
      );
    }

    return content;
```

Note: The `cn` import should already exist in the file. If not, add `import { cn } from "@/lib/utils";` to the imports.

- [ ] **Step 3: Add auto-close on session select in drawer mode**

In `ChatSessionSidebar`, the `onSessionSelect` callback is passed to each `SessionItem` as the `onSelect` prop (around line 208: `onSelect={onSessionSelect}`). To auto-close the drawer when a session is selected, wrap the callback before passing it down:

At the top of the component body (after the `isDrawer` check), add:

```typescript
    const handleSessionSelect = (sessionId: string) => {
      props.onSessionSelect(sessionId);
      if (isDrawer && props.onClose) props.onClose();
    };
```

Then replace `onSelect={onSessionSelect}` (or `onSelect={props.onSessionSelect}`) with `onSelect={handleSessionSelect}` wherever `SessionItem` is rendered. There should be one call site inside the sessions list map.

- [ ] **Step 4: Update ChatPage to use mobile mode**

In `web/src/pages/ChatPage.tsx`, add imports and state at the top of the `ChatPage` component:

After the existing imports, add:
```typescript
import { useIsMobile } from "@/hooks/useIsMobile";
```

Inside `ChatPage()`, after the existing state declarations (around line 112):
```typescript
  const isMobile = useIsMobile();
  const [chatSidebarOpen, setChatSidebarOpen] = useState(false);
```

- [ ] **Step 5: Update ChatPage JSX return**

In the JSX return (line 1322), conditionally pass drawer props to `ChatSessionSidebar`. Instead of duplicating the entire component, use spread to add mobile-only props:

Replace:
```tsx
      <ChatSessionSidebar
        groupedSessions={groupedSessions}
        activeSession={activeSession}
        sessionSearch={sessionSearch}
        loadingSessions={loadingSessions}
        creatingSession={submitting && !activeSession}
        messagesBySession={messagesBySession}
        collapsedGroups={collapsedGroups}
        pendingPermissionSessionIds={pendingPermissionSessionIds}
        onSearchChange={setSessionSearch}
        onSessionSelect={setActiveSession}
        onGroupToggle={handleGroupToggle}
        onCreateSession={createSession}
        onArchiveSession={archiveSession}
      />
```

with:
```tsx
      <ChatSessionSidebar
        groupedSessions={groupedSessions}
        activeSession={activeSession}
        sessionSearch={sessionSearch}
        loadingSessions={loadingSessions}
        creatingSession={submitting && !activeSession}
        messagesBySession={messagesBySession}
        collapsedGroups={collapsedGroups}
        pendingPermissionSessionIds={pendingPermissionSessionIds}
        onSearchChange={setSessionSearch}
        onSessionSelect={setActiveSession}
        onGroupToggle={handleGroupToggle}
        onCreateSession={createSession}
        onArchiveSession={archiveSession}
        {...(isMobile ? { drawerOpen: chatSidebarOpen, onClose: () => setChatSidebarOpen(false) } : {})}
      />
```

- [ ] **Step 6: Add mobile session list toggle button in ChatHeader area**

In the ChatPage JSX, right after the opening `<div className="flex flex-1 flex-col">` (line 1340), add a mobile-only header:

```tsx
      <div className="flex flex-1 flex-col">
        {isMobile && (
          <div className="flex h-10 shrink-0 items-center border-b px-3">
            <button
              onClick={() => setChatSidebarOpen(true)}
              className="flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground hover:bg-accent"
              title="Sessions"
            >
              <MessagesSquare className="h-4 w-4" />
            </button>
          </div>
        )}
```

Add `MessagesSquare` to the existing lucide-react imports at the top of ChatPage if not already imported. Check ChatPage imports — it may already use `MessageSquare` (singular). Add `MessagesSquare` (plural) alongside it.

- [ ] **Step 7: Verify build**

Run: `npm --prefix web run typecheck`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add web/src/pages/ChatPage.tsx web/src/components/chat/ChatSessionSidebar.tsx
git commit -m "feat(web): ChatSessionSidebar as drawer on mobile"
```

---

### Task 5: Touch-Friendly Sizing & iOS Input Zoom Fix

**Files:**
- Modify: `web/src/components/chat/ChatInputBar.tsx`
- Modify: `web/src/pages/MobileHomePage.tsx`

- [ ] **Step 1: Fix ChatInputBar for mobile**

In `web/src/components/chat/ChatInputBar.tsx`, find the `<Input>` component (line 113) with className:
```
"h-auto flex-1 border-0 p-0 text-sm shadow-none focus-visible:ring-0"
```

Change `text-sm` to `text-[16px] md:text-sm` to prevent iOS auto-zoom (iOS zooms when font-size < 16px):
```
"h-auto flex-1 border-0 p-0 text-[16px] md:text-sm shadow-none focus-visible:ring-0"
```

Also find the send/attach/cancel buttons and ensure they have at least `h-10 w-10` sizing for touch targets (40px).

- [ ] **Step 2: Fix MobileHomePage input for iOS zoom**

In `web/src/pages/MobileHomePage.tsx`, find the textarea (line 339):
```tsx
className="w-full resize-none rounded-xl border bg-white/90 px-4 py-3 text-sm placeholder:text-muted-foreground..."
```

Change `text-sm` to `text-[16px] md:text-sm`:
```tsx
className="w-full resize-none rounded-xl border bg-white/90 px-4 py-3 text-[16px] md:text-sm placeholder:text-muted-foreground..."
```

- [ ] **Step 3: Ensure touch target sizes on MobileHomePage buttons**

In `web/src/pages/MobileHomePage.tsx`, the header buttons (Search, Filter) are `h-8 w-8`. Change to `h-10 w-10` for 40px touch targets:

Line 315: `className="h-8 w-8"` → `className="h-10 w-10"`
Line 322: `className="h-8 w-8"` → `className="h-10 w-10"`

The send button (line 424) `className="h-8 w-8 rounded-full"` → `className="h-10 w-10 rounded-full"`

The attach button (line 413) `className="h-8 w-8"` → `className="h-10 w-10"`

- [ ] **Step 4: Verify build**

Run: `npm --prefix web run typecheck`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/components/chat/ChatInputBar.tsx web/src/pages/MobileHomePage.tsx
git commit -m "feat(web): touch-friendly sizing and iOS input zoom prevention"
```

---

### Task 6: Mobile-Friendly Tab Layouts (Monitoring & Runtime)

**Files:**
- Modify: `web/src/layouts/MonitoringLayout.tsx`
- Modify: `web/src/layouts/RuntimeLayout.tsx`

- [ ] **Step 1: Update MonitoringLayout tab bar**

In `web/src/layouts/MonitoringLayout.tsx`, find the tab container and reduce padding on mobile:

Change the tab container className from:
```
"flex items-center gap-1 overflow-x-auto px-8 pt-4"
```
to:
```
"flex items-center gap-1 overflow-x-auto px-4 pt-3 md:px-8 md:pt-4"
```

Also find individual tab NavLinks and ensure they have touch-friendly padding. Note: `whitespace-nowrap` may already exist on tab links — check before adding it.

Change tab link padding from `px-4 pb-3` to `px-3 pb-2.5 md:px-4 md:pb-3`.

- [ ] **Step 2: Apply same changes to RuntimeLayout**

Apply identical responsive padding changes to `web/src/layouts/RuntimeLayout.tsx`.

- [ ] **Step 3: Verify build**

Run: `npm --prefix web run typecheck`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add web/src/layouts/MonitoringLayout.tsx web/src/layouts/RuntimeLayout.tsx
git commit -m "feat(web): mobile-friendly tab padding in monitoring/runtime layouts"
```

---

### Task 7: Hide Duplicate Logo on Mobile Home Page

**Files:**
- Modify: `web/src/pages/MobileHomePage.tsx`

- [ ] **Step 1: Remove duplicate header on mobile**

The MobileHomePage already has its own header with "AI Workflow" branding. On mobile, the new AppLayout also shows a mobile header with hamburger. This creates a double-header situation.

In `MobileHomePage`, wrap the existing `<header>` block with a check: only show it when inside mobile layout (where the AppLayout mobile header already provides branding).

Since the MobileHomePage is displayed inside the AppLayout, on mobile the AppLayout already shows a header with the hamburger + "AI Workflow" text. The MobileHomePage's own header (with Search/Filter buttons) should be simplified on mobile to just show the action buttons without the logo duplication.

Hide the logo portion on mobile using `hidden md:flex` (since AppLayout already shows it on mobile).

Change line 303:
```tsx
          <div className="flex items-center gap-2.5">
```
to:
```tsx
          <div className="hidden items-center gap-2.5 md:flex">
```

This hides the logo on mobile (since AppLayout header already shows it), and only shows Search/Filter buttons.

- [ ] **Step 2: Verify build**

Run: `npm --prefix web run typecheck`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/MobileHomePage.tsx
git commit -m "fix(web): hide duplicate logo in MobileHomePage on mobile"
```

---

### Task 8: Final Build Verification

**Files:**
- None (verification only)

- [ ] **Step 1: Run typecheck**

Run: `npm --prefix web run typecheck`
Expected: PASS with 0 errors

- [ ] **Step 2: Run frontend unit tests**

Run: `npm --prefix web run test`
Expected: All existing tests pass

- [ ] **Step 3: Run production build**

Run: `npm --prefix web run build`
Expected: Build succeeds with no errors

- [ ] **Step 4: Manual mobile testing checklist**

Open `http://localhost:5173` in browser, toggle device toolbar to iPhone 14 (390px):

1. **Home page (`/`)**: Logo hidden (uses AppLayout header), input area visible, session list scrollable, touch targets ≥ 40px
2. **Chat page (`/chat`)**: Session sidebar hidden, toggle button visible, tapping opens drawer, selecting session closes drawer
3. **Navigation**: Hamburger in top-left opens drawer, all nav links work, drawer closes on link click
4. **Monitoring/Runtime tabs**: Horizontal scroll on narrow screens, no text wrapping
5. **Input fields**: No zoom on focus (iOS), font size ≥ 16px on mobile

- [ ] **Step 5: Commit any final fixes**

```bash
git add -A
git commit -m "fix(web): mobile responsive polish"
```
