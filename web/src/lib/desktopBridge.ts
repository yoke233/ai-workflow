export interface DesktopBootstrap {
  token: string;
}

export const isDesktop = (): boolean => {
  if (typeof window === "undefined") {
    return false;
  }
  const w = window as unknown as {
    go?: unknown;
    runtime?: unknown;
  };
  return Boolean(w.go || w.runtime);
};

/** @deprecated Use isDesktop() instead. */
export const isTauri = isDesktop;

const sleep = (ms: number): Promise<void> =>
  new Promise((resolve) => setTimeout(resolve, ms));

export const fetchDesktopBootstrap = async (options?: {
  timeoutMs?: number;
  retryIntervalMs?: number;
}): Promise<DesktopBootstrap> => {
  if (!isDesktop()) {
    throw new Error("not running in desktop mode");
  }

  const timeoutMs = options?.timeoutMs ?? 8000;
  const retryIntervalMs = options?.retryIntervalMs ?? 300;
  const startedAt = Date.now();
  let lastError: unknown = null;

  while (Date.now() - startedAt < timeoutMs) {
    try {
      const w = window as unknown as {
        go?: {
          main?: {
            DesktopApp?: {
              GetBootstrap?: () => Promise<DesktopBootstrap>;
            };
          };
        };
      };
      const fn = w.go?.main?.DesktopApp?.GetBootstrap;
      if (!fn) {
        throw new Error("Wails bindings not ready");
      }
      const result = await fn();
      if (!result || typeof result.token !== "string" || result.token.trim().length === 0) {
        throw new Error("GetBootstrap returned empty token");
      }
      return result;
    } catch (err) {
      lastError = err;
      await sleep(retryIntervalMs);
    }
  }

  if (lastError instanceof Error) {
    throw lastError;
  }
  throw new Error("desktop bootstrap timed out");
};
