/**
 * Lightweight API client for theme CRUD operations.
 * Uses the same auth token stored by WorkbenchContext.
 */

const TOKEN_KEY = "ai-workflow-api-token";

function getAuthHeaders(): Record<string, string> {
  const token = localStorage.getItem(TOKEN_KEY);
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (token) headers["Authorization"] = `Bearer ${token}`;
  return headers;
}

export interface UserThemeListItem {
  id: string;
  name: string;
  type: "dark" | "light";
  folder: string;
  created_at: string;
}

export interface SaveThemeRequest {
  id: string;
  name: string;
  type: "dark" | "light";
  data: unknown; // the raw VSCode theme JSON
}

/** GET /api/themes — list all user-imported themes */
export async function listUserThemes(): Promise<UserThemeListItem[]> {
  const resp = await fetch("/api/themes", { headers: getAuthHeaders() });
  if (!resp.ok) return [];
  return resp.json();
}

/** GET /api/themes/:id — get theme.json content */
export async function getUserTheme(id: string): Promise<string | null> {
  const resp = await fetch(`/api/themes/${id}`, { headers: getAuthHeaders() });
  if (!resp.ok) return null;
  return resp.text();
}

/** POST /api/themes — save a new user theme to disk */
export async function saveUserTheme(req: SaveThemeRequest): Promise<boolean> {
  const resp = await fetch("/api/themes", {
    method: "POST",
    headers: getAuthHeaders(),
    body: JSON.stringify(req),
  });
  return resp.ok;
}

/** DELETE /api/themes/:id — remove a user theme from disk */
export async function deleteUserTheme(id: string): Promise<boolean> {
  const resp = await fetch(`/api/themes/${id}`, {
    method: "DELETE",
    headers: getAuthHeaders(),
  });
  return resp.ok || resp.status === 404;
}
